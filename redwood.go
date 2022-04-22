// Redwood is an internet content-filtering program.
// It is designed to replace and improve on DansGuardian
// as the core of the Security Appliance internet filter.
package main

import (
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
)

// Version is the current version number. Fill it in by building with
//
// go build -ldflags="-X 'main.Version=$(git describe --tags)'"
var Version string = "Version 1.1.3-SNAPSHOT"

// Built-in Categories
//go:embed built-in
var BuiltInFS embed.FS

func main() {
	if Version != "" {
		log.Println("Redwood", Version)
	}

	conf, err := loadConfiguration()
	if err != nil {
		log.Fatal(err)
	}
	configuration = conf

	if conf.TestURL != "" {
		runURLTest(conf.TestURL)
		return
	}

	accessLog.Open(conf.AccessLog)
	tlsLog.Open(conf.TLSLog)
	contentLog.Open(filepath.Join(conf.ContentLogDir, "index.csv"))
	starlarkLog.Open(conf.StarlarkLog)

	if conf.PIDFile != "" {
		pid := os.Getpid()
		f, err := os.Create(conf.PIDFile)
		if err == nil {
			fmt.Fprintln(f, pid)
			f.Close()
		} else {
			log.Println("could not create pidfile:", err)
		}
	}

	if conf.CloseIdleConnections > 0 {
		httpTransport.IdleConnTimeout = conf.CloseIdleConnections
	}

	portsListening := 0

	for _, addr := range conf.ProxyAddresses {
		proxyListener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("error listening for connections on %s: %s", addr, err)
		}
		go func() {
			<-shutdownChan
			proxyListener.Close()
		}()
		server := http.Server{
			Handler:     proxyHandler{},
			IdleTimeout: conf.CloseIdleConnections,
		}
		go func() {
			err := server.Serve(tcpKeepAliveListener{proxyListener.(*net.TCPListener)})
			if err != nil && !strings.Contains(err.Error(), "use of closed") {
				log.Fatalln("Error running HTTP proxy:", err)
			}
		}()
		portsListening++
	}

	for _, addr := range conf.TransparentAddresses {
		go func() {
			err := runTransparentServer(addr)
			if err != nil && !strings.Contains(err.Error(), "use of closed") {
				log.Fatalln("Error running transparent HTTPS proxy:", err)
			}
		}()
		portsListening++
	}

	conf.openPerUserPorts()
	portsListening += len(conf.CustomPorts)

	if portsListening > 0 {
		// Wait forever (or until somebody calls log.Fatal).
		select {}
	}
}
