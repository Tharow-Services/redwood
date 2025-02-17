package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tharow-services/redwood/efs"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Functions for displaying block pages.

// transparent1x1 is a single-pixel transparent GIF file.
const transparent1x1 = "GIF89a\x10\x00\x10\x00\x80\xff\x00\xc0\xc0\xc0\x00\x00\x00!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x10\x00\x10\x00\x00\x02\x0e\x84\x8f\xa9\xcb\xed\x0f\xa3\x9c\xb4\u068b\xb3>\x05\x00;"

func (conf *config) loadBlockPage(path string) error {
	if strings.HasPrefix(path, "http") {
		conf.BlockTemplate = nil
		conf.BlockpageURL = path
		return nil
	}

	bt := template.New("block-page")
	content, err := efs.ReadFile(path)

	if err != nil {
		return fmt.Errorf("error loading block page template: %v", err)
	}
	_, err = bt.Parse(string(content))
	if err != nil {
		return fmt.Errorf("error parsing block page template: %v", err)
	}

	conf.BlockTemplate = bt
	conf.BlockpageURL = ""
	return nil
}

type blockData struct {
	URL             string
	Categories      string
	Category        string
	Conditions      string
	User            string
	Tally           string
	Scores          string
	RuleDescription string
	Referer         string
	Request         *http.Request
	Response        *http.Response
}

func (conf *config) aclDescription(name string) string {
	cat, ok := conf.Categories[name]
	if ok {
		return cat.description
	}

	d, ok := conf.ACLs.Descriptions[name]
	if ok {
		return d
	}

	return name
}

// Convert rule conditions into category descriptions as much as possible.
func (conf *config) aclDescriptions(rule ACLActionRule) []string {
	var categories []string
	for _, acl := range rule.Needed {
		categories = append(categories, conf.aclDescription(acl))
	}
	for _, acl := range rule.Disallowed {
		categories = append(categories, "not "+conf.aclDescription(acl))
	}

	return categories
}

func showURLValue(u *url.URL) string {
	var ret string = u.Host + u.Path
	if u.RawQuery != "" {
		ret += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		ret += "#" + u.Fragment
	}
	return ret
}

// showBlockPage shows a block page for a page that was blocked by an ACL.
func showBlockPage(w http.ResponseWriter, r *http.Request, resp *http.Response, user string, tally map[rule]int, scores map[string]int, rule ACLActionRule) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Redwood-Block-Page", "403 Access Denied")

	c := getConfig()
	switch {
	case c.BlockTemplate != nil:
		data := blockData{
			URL:             showURLValue(r.URL),
			Conditions:      rule.Conditions(),
			User:            user,
			Tally:           listTally(stringTally(tally)),
			Scores:          listTally(scores),
			Categories:      strings.Join(c.aclDescriptions(rule), ", "),
			Category:        c.aclDescriptions(rule)[0],
			RuleDescription: rule.Description,
			Referer:         r.Referer(),
			Request:         r,
			Response:        resp,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)

		err := c.BlockTemplate.Execute(w, data)
		if err != nil {
			log.Println("Error filling in block page template:", err)
		}

	case c.BlockpageURL != "":
		d := map[string]interface{}{
			"url":            r.URL.String(),
			"rule":           rule,
			"user":           user,
			"tally":          stringTally(tally),
			"scores":         scores,
			"categories":     c.aclDescriptions(rule),
			"category":       c.aclDescriptions(rule)[0],
			"method":         r.Method,
			"referer":        r.Referer(),
			"request-header": r.Header,
		}
		if resp != nil {
			d["response-header"] = resp.Header
		}
		data, err := json.Marshal(d)
		if err != nil {
			log.Println("Error generating JSON info for block page:", err)
			http.Error(w, "", http.StatusForbidden)
			return
		}

		blockResp, err := http.Post(c.BlockpageURL, "application/json", bytes.NewReader(data))
		if err != nil {
			log.Printf("Error fetching blockpage from %s: %v", c.BlockpageURL, err)
			http.Error(w, "", http.StatusForbidden)
			return
		}
		defer blockResp.Body.Close()

		removeHopByHopHeaders(blockResp.Header)
		if blockResp.ContentLength > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(blockResp.ContentLength, 10))
		}
		blockResp.StatusCode = http.StatusForbidden
		copyResponseHeader(w, blockResp)
		_, err = io.Copy(w, blockResp.Body)
		if err != nil {
			panic(http.ErrAbortHandler)
		}

	default:
		http.Error(w, "", http.StatusForbidden)
		return
	}
}

// showInvisibleBlock blocks the request with an invisible image.
func showInvisibleBlock(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, transparent1x1)
}

func (conf *config) loadErrorPage(path string) error {
	if strings.HasPrefix(path, "http") {
		conf.ErrorTemplate = nil
		conf.ErrorURL = path
		return nil
	}

	bt := template.New("error-page")
	content, err := efs.ReadFile(path)

	if err != nil {
		return fmt.Errorf("error loading error page template: %v", err)
	}
	_, err = bt.Parse(string(content))
	if err != nil {
		return fmt.Errorf("error parsing error page template: %v", err)
	}

	conf.ErrorTemplate = bt
	conf.ErrorURL = ""
	return nil
}

// showErrorPage shows an error page for a request that failed (as we were
// fetching it from the origin server).
func showErrorPage(w http.ResponseWriter, r *http.Request, pageError error) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	c := getConfig()
	d := map[string]interface{}{
		"url":   r.URL.String(),
		"error": pageError.Error(),
	}

	var dnsError *net.DNSError
	if errors.As(pageError, &dnsError) {
		d["dns error"] = dnsError
	}

	w.Header().Set("Connection", "close")

	switch {
	case c.ErrorTemplate != nil:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)

		err := c.ErrorTemplate.Execute(w, d)
		if err != nil {
			log.Println("Error filling in error page template:", err)
			panic(http.ErrAbortHandler)
		}

	case c.ErrorURL != "":
		data, err := json.Marshal(d)
		if err != nil {
			log.Println("Error generating JSON info for error page:", err)
			http.Error(w, pageError.Error(), http.StatusBadGateway)
			return
		}

		errorResp, err := clientWithExtraRootCerts.Post(c.ErrorURL, "application/json", bytes.NewReader(data))
		if err != nil {
			log.Printf("Error fetching  error page from %s: %v", c.ErrorURL, err)
			http.Error(w, pageError.Error(), http.StatusBadGateway)
			return
		}
		defer errorResp.Body.Close()

		removeHopByHopHeaders(errorResp.Header)
		if errorResp.ContentLength > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(errorResp.ContentLength, 10))
		}
		errorResp.StatusCode = http.StatusBadGateway
		copyResponseHeader(w, errorResp)
		_, err = io.Copy(w, errorResp.Body)
		if err != nil {
			panic(http.ErrAbortHandler)
		}

	default:
		http.Error(w, pageError.Error(), http.StatusBadGateway)
		return
	}
}
