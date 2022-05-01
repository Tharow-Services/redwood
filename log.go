package main

import (
	"bytes"
	"crypto/md5"
	"encoding/csv"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// recording pages filtered to access log

var (
	accessLog   CSVLog
	tlsLog      CSVLog
	contentLog  CSVLog
	starlarkLog CSVLog
)

type CSVLog struct {
	lock sync.Mutex
	file *os.File
	csv  *csv.Writer
}

func (l *CSVLog) Open(filename string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.file != nil && l.file != os.Stdout {
		l.file.Close()
		l.file = nil
	}

	if filename != "" {
		logfile, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			log.Printf("Could not open log file (%s): %s\n Sending log messages to standard output instead.", filename, err)
		} else {
			l.file = logfile
		}
	}
	if l.file == nil {
		l.file = os.Stdout
	}

	l.csv = csv.NewWriter(l.file)
}

func (l *CSVLog) Log(data []string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.csv.Write(data)
	l.csv.Flush()
}

func logAccess(req *http.Request, resp *http.Response, contentLength int64, pruned bool, user string, tally map[rule]int, scores map[string]int, rule ACLActionRule, title string, ignored []string) []string {
	conf := getConfig()

	modified := ""
	if pruned {
		modified = "pruned"
	}

	status := 0
	if resp != nil {
		status = resp.StatusCode
	}

	if rule.Action == "" {
		rule.Action = "allow"
	}

	var contentType string
	if resp != nil {
		contentType = resp.Header.Get("Content-Type")
	}
	if ct2, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = ct2
	}

	var userAgent string
	if conf.LogUserAgent {
		userAgent = req.Header.Get("User-Agent")
	}

	if len(title) > 500 {
		title = title[:500]
	}

	logLine := toStrings(time.Now().Format("2006-01-02 15:04:05.000000"), user, rule.Action, req.URL, req.Method, status, contentType, contentLength, modified, listTally(stringTally(tally)), listTally(scores), rule.Conditions(), title, strings.Join(ignored, ","), userAgent, req.Proto, req.Referer(), platform(req.Header.Get("User-Agent")), downloadedFilename(resp), rule.Description)

	accessLog.Log(logLine)
	return logLine
}

func downloadedFilename(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	disposition := resp.Header.Get("Content-Disposition")
	if disposition == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		return ""
	}
	return params["filename"]
}

func Lce(err error) {
	log.Printf("unable to close rescourse: %s", err)
}

func (t TLSSession) logClose(err error, c bool, l string) {
	t.Errorf("unable to close connection: %s", err, c, l)
}
func (t TLSSession) Errorf(format string, err error, cachedCert bool, tlsFingerprint string) {
	t.Error(fmt.Errorf(format, err), cachedCert, tlsFingerprint)
}

func (t TLSSession) Error(err error, cachedCert bool, tlsFingerprint string) {
	logTLS(t.User, t.ServerAddr, t.SNI, err, cachedCert, tlsFingerprint)
}

func logTLS(user, serverAddr, serverName string, err error, cachedCert bool, tlsFingerprint string) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	cached := ""
	if cachedCert {
		cached = "cached certificate"
	}
	// "2006-01-02 15:04:05.000000"
	tlsLog.Log(toStrings(time.Now().Format("2006.01.02::15:04:05.000000"), user, serverName, serverAddr, errStr, cached, tlsFingerprint))
}

func logContent(u *url.URL, content []byte, scores map[string]int) {
	conf := getConfig()
	if conf.ContentLogDir == "" {
		return
	}

	filename := fmt.Sprintf("%x", md5.Sum(content))
	path := filepath.Join(conf.ContentLogDir, filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error creating content log file (%s): %v", path, err)
		return
	}
	defer f.Close()

	topCategory, topScore := "", 0
	for c, s := range scores {
		if s > topScore && conf.Categories[c] != nil && conf.Categories[c].action != ACL {
			topCategory = c
			topScore = s
		}
	}

	f.Write(content)
	contentLog.Log([]string{u.String(), filename, topCategory, strconv.Itoa(topScore)})
}

// toStrings converts its arguments into a slice of strings.
func toStrings(a ...interface{}) []string {
	result := make([]string, len(a))
	for i, x := range a {
		result[i] = fmt.Sprint(x)
	}
	return result
}

// stringTally returns a copy of tally with strings instead of rules as keys.
func stringTally(tally map[rule]int) map[string]int {
	st := make(map[string]int)
	for r, n := range tally {
		st[r.String()] = n
	}
	return st
}

// listTally sorts the tally and formats it as a comma-separated string.
func listTally(tally map[string]int) string {
	b := new(bytes.Buffer)
	for i, rule := range sortedKeys(tally) {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprint(b, rule, " ", tally[rule])
	}
	return b.String()
}

// logVerbose logs a message with log.Printf, but only if the --verbose flag
// is turned on for the category.
func logVerbose(messageCategory string, format string, v ...interface{}) {
	if getConfig().Verbose[messageCategory] {
		log.Printf(format, v...)
	}
}
