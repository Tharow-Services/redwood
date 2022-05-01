package main

import (
	"github.com/andybalholm/redwood/efs"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"path/filepath"
)

// The built-in web server, which serves URLs under http://redwood.services or http://203.0.113.1

var localServer string = "redwood.services"

func (conf *config) startWebServer() {
	if conf.StaticFilesDir != "" {
		var hfs http.FileSystem
		if efs.IsEmbed(conf.StaticFilesDir) {
			hfs = http.FileSystem(http.FS(*efs.GetInternalFS()))
			// lock down internal fs after getting pointer
			efs.LockInternalFS()
		} else {
			hfs = http.Dir(conf.StaticFilesDir)
		}
		conf.ServeMux.Handle("/", http.FileServer(hfs))
	}

	if conf.CGIBin != "" {
		dir, err := os.Open(conf.CGIBin)
		if err != nil {
			log.Println("Could not open CGI directory:", err)
			return
		}
		defer func(dir *os.File) {
			err := dir.Close()
			if err != nil {
				Lce(err)
			}
		}(dir)

		info, err := dir.Readdir(0)
		if err != nil {
			log.Println("Could not read CGI directory:", err)
			return
		}

		for _, fi := range info {
			if mode := fi.Mode(); (mode&os.ModeType == 0) && (mode.Perm()&0100 != 0) {
				// It's an executable file.
				name := "/" + fi.Name()
				scriptPath := filepath.Join(conf.CGIBin, fi.Name())
				conf.ServeMux.Handle(name, &cgi.Handler{
					Path: scriptPath,
				})
			}
		}
	}
}
