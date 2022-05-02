package main

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/andybalholm/cascadia"
	"github.com/andybalholm/dhash"
	"github.com/golang/gddo/httputil"
	"github.com/golang/gddo/httputil/header"
	"github.com/klauspost/compress/gzip"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

type proxyHandler struct {
	// TLS is whether this is an HTTPS connection.
	TLS bool

	// tlsFingerprint is the JA3 TLS fingerprint of the client (if available).
	tlsFingerprint string

	// connectPort is the server port that was specified in a CONNECT request.
	connectPort string

	// user is a user that has already been authenticated.
	user string

	// rt is the RoundTripper that will be used to fulfill the requests.
	// If it is nil, a default Transport will be used.
	rt http.RoundTripper
}

var ip6Loopback = net.ParseIP("::1")

// lanAddress returns whether addr is in one of the LAN address ranges.
func lanAddress(addr string) bool {
	ip := net.ParseIP(addr)
	if ip4 := ip.To4(); ip4 != nil {
		switch ip4[0] {
		case 10, 127:
			return true
		case 172:
			return ip4[1]&0xf0 == 16
		case 192:
			return ip4[1] == 168
		}
		return false
	}

	// IPv6
	switch {
	case ip[0]&0xfe == 0xfc:
		return true
	case ip[0] == 0xfe && (ip[1]&0xfc) == 0x80:
		return true
	case ip.Equal(ip6Loopback):
		return true
	}

	return false
}

var titleSelector = cascadia.MustCompile("title")

func (h proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	activeConnections.Add(1)
	defer activeConnections.Done()

	// If a request is directed to Redwood, rather than proxied or intercepted,
	// it should be handled as an API request.
	if !h.TLS && r.URL.Host == "" && strings.Contains(r.Host, ":") {
		handleAPI(w, r)
		return
	}

	if len(r.URL.String()) > 10000 {
		http.Error(w, "URL too long", http.StatusRequestURITooLong)
		return
	}

	client := r.RemoteAddr
	host, _, err := net.SplitHostPort(client)
	if err == nil {
		client = host
	}

	authUser := ""

	if h.user != "" {
		authUser = h.user
	} else if r.Header.Get("Proxy-Authorization") != "" {
		user, pass, ok := ProxyCredentials(r)
		if ok {
			if getConfig().ValidCredentials(user, pass) {
				authUser = user
			} else {
				log.Printf("Incorrect username or password from %v: %s:%s", r.RemoteAddr, user, pass)
			}
		} else {
			log.Printf("Invalid Proxy-Authorization header from %v: %q", r.RemoteAddr, r.Header.Get("Proxy-Authorization"))
		}
	} else if u, ok := getConfig().IPToUser[client]; ok {
		authUser = u
	} else if lanAddress(client) {
		authUser = fmt.Sprintf("local/%s", client)
	} else {
		authUser = "Unknown/Anonymous"
	}

	// Reconstruct the URL if it is incomplete (i.e. on a transparent proxy).
	if r.URL.Scheme == "" {
		if h.TLS {
			r.URL.Scheme = "https"
		} else {
			r.URL.Scheme = "http"
		}
	}
	if r.URL.Host == "" {
		if r.Host != "" {
			r.URL.Host = r.Host
		} else {
			log.Printf("Request from %s has no host in URL: %v", client, r.URL)
			// Delay a while since some programs really hammer us with this kind of request.
			time.Sleep(time.Second)
			http.Error(w, "No host in request URL, and no Host header.", http.StatusBadRequest)
			return
		}
	}

	if realHost, ok := getConfig().VirtualHosts[r.Host]; ok {
		r.Host = realHost
		r.URL.Host = realHost
	}

	user := client
	if authUser != "" {
		user = authUser
	}

	// Handle IPv6 hostname without brackets in CONNECT request.
	if r.Method == "CONNECT" {
		hostport := r.URL.Host
		_, _, err := net.SplitHostPort(hostport)
		if err, ok := err.(*net.AddrError); ok && err.Err == "too many colons in address" {
			colon := strings.LastIndex(hostport, ":")
			host, port := hostport[:colon], hostport[colon+1:]
			if ip := net.ParseIP(host); ip != nil {
				r.URL.Host = net.JoinHostPort(host, port)
			}
		}
	}

	// Some proxy interception programs send HTTP traffic as CONNECT requests
	// for port 80.
	if _, port, err := net.SplitHostPort(r.URL.Host); err == nil && port == "80" && r.Method == "CONNECT" {
		conn, err := newHijackedConn(w)
		if err != nil {
			log.Printf("Error hijacking connection for CONNECT request to %s: %v", r.URL.Host, err)
			panic(http.ErrAbortHandler)
		}
		_, err = fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		if err != nil {
			log.Print(fmt.Errorf("error printing to connection: %s", err))
		}

		server := &http.Server{
			Handler: proxyHandler{
				TLS:         false,
				connectPort: port,
				user:        authUser,
				rt:          h.rt,
			},
			IdleTimeout: getConfig().CloseIdleConnections,
		}
		err = server.Serve(&singleListener{conn: conn})
		if err != nil {
			log.Print(fmt.Errorf("single server listener error: %s", err))
		}
		return
	}

	if h.tlsFingerprint != "" {
		r = r.WithContext(context.WithValue(r.Context(), tlsFingerprintKey{}, h.tlsFingerprint))
	}

	request := &Request{
		Request:  r,
		User:     authUser,
		ClientIP: client,
	}

	filterRequest(request, !h.TLS)

	if request.Action.Action == "require-auth" {
		send407(w)
		log.Printf("Missing required proxy authentication from %v to %v", r.RemoteAddr, r.URL)
		return
	}

	if r.Method == "CONNECT" && getConfig().TLSReady {
		// SSLBump takes priority overy any action besides require-auth, because showing a block page
		// doesn't work till after the connection is bumped.
		conn, err := newHijackedConn(w)
		if err != nil {
			log.Printf("Error hijacking connection for CONNECT request to %s: %v", r.URL.Host, err)
			panic(http.ErrAbortHandler)
		}
		_, err = fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		if err != nil {
			log.Print(fmt.Errorf("error printing to connection: %s", err))
		}
		SSLBump(conn, r.URL.Host, user, authUser, r)
		return
	}

	switch request.Action.Action {
	case "block":
		showBlockPage(w, r, nil, user, request.Tally, request.Scores.data, request.Action)
		logAccess(r, nil, 0, false, user, request.Tally, request.Scores.data, request.Action, "", request.Ignored)
		return
	case "block-invisible":
		showInvisibleBlock(w)
		logAccess(r, nil, 0, false, user, request.Tally, request.Scores.data, request.Action, "", request.Ignored)
		return
	}

	if r.Host == localServer {
		logAccess(r, nil, 0, false, user, request.Tally, request.Scores.data, request.Action, "", request.Ignored)
		getConfig().ServeMux.ServeHTTP(w, r)
		return
	}

	if r.Method == "CONNECT" {
		// …and not TLSReady
		conn, err := newHijackedConn(w)
		if err != nil {
			log.Printf("Error hijacking connection for CONNECT request to %s: %v", r.URL.Host, err)
			panic(http.ErrAbortHandler)
		}

		_, err = fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		if err != nil {
			log.Print(fmt.Errorf("error printing to connection: %s", err))
		}
		logAccess(r, nil, 0, false, user, request.Tally, request.Scores.data, request.Action, "", request.Ignored)
		connectDirect(conn, r.URL.Host, nil, dialer)
		return
	}

	if r.Header.Get("Upgrade") == "websocket" {
		logAccess(r, nil, 0, false, user, request.Tally, request.Scores.data, request.Action, "", request.Ignored)
		h.makeWebsocketConnection(w, r)
		return
	}

	if len(r.Header["X-Forwarded-For"]) >= 10 {
		w.Header().Set("Connection", "close")
		http.Error(w, "Proxy forwarding loop", http.StatusBadRequest)
		log.Printf("Proxy forwarding loop from %s to %v", r.Header.Get("X-Forwarded-For"), r.URL)
		return
	}

	{
		conf := getConfig()
		headerRule, _ := conf.ChooseACLCategoryAction(request.ACLs.data, request.Scores.data, conf.Threshold, "disable-proxy-headers")
		if headerRule.Action != "disable-proxy-headers" {
			viaHosts := r.Header["Via"]
			viaHosts = append(viaHosts, strings.TrimPrefix(r.Proto, "HTTP/")+" Redwood")
			r.Header.Set("Via", strings.Join(viaHosts, ", "))
			r.Header.Add("X-Forwarded-For", client)
		}
	}

	// Limit Accept-Encoding header to encodings we can handle.
	acceptEncoding := header.ParseAccept(r.Header, "Accept-Encoding")
	filteredEncodings := make([]header.AcceptSpec, 0, len(acceptEncoding))
	for _, a := range acceptEncoding {
		switch a.Value {
		case "br", "gzip", "deflate":
			filteredEncodings = append(filteredEncodings, a)
		}
	}
	switch {
	case len(filteredEncodings) == 0:
		r.Header.Del("Accept-Encoding")
	case len(filteredEncodings) != len(acceptEncoding):
		specs := make([]string, len(filteredEncodings))
		for i, a := range filteredEncodings {
			if a.Q == 1 {
				specs[i] = a.Value
			} else {
				specs[i] = fmt.Sprintf("%s;q=%f", a.Value, a.Q)
			}
		}
		r.Header.Set("Accept-Encoding", strings.Join(specs, ", "))
	}

	getConfig().changeQuery(r.URL)

	var rt http.RoundTripper
	switch {
	case r.URL.Scheme == "ftp":
		rt = FTPTransport{}
	case h.rt != nil:
		rt = h.rt
	default:
		rt = httpTransport
	}

	// Some HTTP/2 servers don't like having a body on a GET request, even if
	// it is empty.
	if r.ContentLength == 0 {
		Lce(r.Body.Close())
		r.Body = nil
	}

	removeHopByHopHeaders(r.Header)
	resp, err := rt.RoundTrip(r)

	if err == context.Canceled {
		return
	}
	if err != nil {
		showErrorPage(w, r, err)
		log.Printf("error fetching %s: %s", r.URL, err)
		logAccess(r, nil, 0, false, user, request.Tally, request.Scores.data, request.Action, "", request.Ignored)
		return
	}
	defer func(Body io.ReadCloser) {
		Lce(Body.Close())
	}(resp.Body)

	// Prevent switching to QUIC.
	resp.Header.Del("Alternate-Protocol")
	resp.Header.Del("Alt-Svc")

	removeHopByHopHeaders(resp.Header)

	// Yet another workaround for https://github.com/golang/go/issues/31753
	if resp.Header.Get("Content-Type") == "" && resp.Header.Get("Content-Encoding") == "gzip" && r.Method != "HEAD" {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.Printf("Error creating gzip reader for %v: %v", r.URL, err)
		} else {
			resp.Body = gzr
			resp.Header.Del("Content-Encoding")
		}
	}

	response := &Response{
		Request:  request,
		Response: resp,
	}
	response.Scores = request.Scores
	response.Tally = make(map[rule]int)
	for k, v := range request.Tally {
		response.Tally[k] = v
	}

	var scanAction ACLActionRule
	{
		conf := getConfig()
		respACLs := conf.ACLs.responseACLs(resp)
		response.ACLs.data = unionACLSets(request.ACLs.data, respACLs)

		headerRule, _ := conf.ChooseACLCategoryAction(response.ACLs.data, response.Scores.data, conf.Threshold, "disable-proxy-headers")
		if headerRule.Action != "disable-proxy-headers" {
			viaHosts := resp.Header["Via"]
			viaHosts = append(viaHosts, strings.TrimPrefix(resp.Proto, "HTTP/")+" Redwood")
			resp.Header.Set("Via", strings.Join(viaHosts, ", "))
		}

		var possibleActions []string
		if r.Method != "HEAD" {
			possibleActions = append(possibleActions, "hash-image", "phrase-scan")
		}

		scanAction, _ = conf.ChooseACLCategoryAction(response.ACLs.data, response.Scores.data, conf.Threshold, possibleActions...)
	}

	switch scanAction.Action {
	case "phrase-scan":
		if err := doPhraseScan(response); err != nil {
			showErrorPage(w, r, err)
			return
		}

	case "hash-image":
		if err := doImageHash(response); err != nil {
			showErrorPage(w, r, err)
			return
		}
	}

	response.Scores.data = getConfig().categoryScores(response.Tally)

	contentRule, _ := getConfig().ChooseACLCategoryAction(response.ACLs.data, response.Scores.data, 1, "log-content")
	if contentRule.Action == "log-content" {
		content, _ := response.Content(math.MaxInt)
		if content != nil {
			logContent(r.URL, content, response.Scores.data)
		}
	}

	response.PossibleActions = []string{"allow", "block", "block-invisible"}

	getConfig().RootScriptHandler.FilterResponse(&response)

	response.chooseAction()

	switch response.Action.Action {
	case "block":
		showBlockPage(w, r, resp, user, response.Tally, response.Scores.data, response.Action)
		logAccess(r, resp, 0, response.Modified, user, response.Tally, response.Scores.data, response.Action, response.PageTitle, response.Ignored)
		return
	case "block-invisible":
		showInvisibleBlock(w)
		logAccess(r, resp, 0, response.Modified, user, response.Tally, response.Scores.data, response.Action, response.PageTitle, response.Ignored)
		return
	}

	if response.Response.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(response.Response.ContentLength, 10))
	}
	copyResponseHeader(w, resp)
	n, err := io.Copy(w, response.Response.Body)
	if err != nil && err != context.Canceled {
		log.Printf("error while copying response (URL: %s): %s", r.URL, err)
	}

	logAccess(r, resp, n, response.Modified, user, response.Tally, response.Scores.data, response.Action, response.PageTitle, response.Ignored)
}

func filterRequest(req *Request, checkAuth bool) {
	r := req.Request

	req.Tally = getConfig().URLRules.MatchingRules(r.URL)
	req.Scores.data = getConfig().categoryScores(req.Tally)

	for _, classifier := range getConfig().ExternalClassifiers {
		v := make(url.Values)
		v.Set("url", r.URL.String())
		v.Set("method", r.Method)
		cr, err := clientWithExtraRootCerts.PostForm(classifier, v)
		if err != nil {
			log.Printf("Error checking external-classifier (%s): %v", classifier, err)
			continue
		}
		if cr.StatusCode != 200 {
			log.Printf("Bad HTTP status checking external-classifier (%s): %s", classifier, cr.Status)
			continue
		}
		jd := json.NewDecoder(cr.Body)
		externalScores := make(map[string]int)
		err = jd.Decode(&externalScores)
		Lce(cr.Body.Close())
		if err != nil {
			log.Printf("Error decoding response from external-classifier (%s): %v", classifier, err)
			continue
		}
		if req.Scores.data == nil {
			req.Scores.data = make(map[string]int)
		}
		for k, v := range externalScores {
			req.Scores.data[k] += v
		}
	}

	req.ACLs.data = getConfig().ACLs.requestACLs(r, req.User)
	req.PossibleActions = []string{
		"allow",
		"block",
		"block-invisible",
	}
	if req.User == "" && checkAuth {
		req.PossibleActions = append(req.PossibleActions, "require-auth")
	}

	getConfig().RootScriptHandler.FilterRequest(&req)

	req.chooseAction()
}

func doPhraseScan(response *Response) error {
	content, err := response.Content(getConfig().MaxContentScanSize)
	if err != nil {
		return err
	}
	if content != nil {
		conf := getConfig()
		contentType := response.Response.Header.Get("Content-Type")
		_, cs, _ := charset.DetermineEncoding(content, contentType)
		var doc *html.Node
		modified := false
		if strings.Contains(contentType, "html") {
			if conf.LogTitle {
				doc, err = parseHTML(content, cs)
				if err != nil {
					log.Printf("Error parsing HTML from %s: %s", response.Request.Request.URL, err)
				} else {
					t := titleSelector.MatchFirst(doc)
					if t != nil {
						if titleText := t.FirstChild; titleText != nil && titleText.Type == html.TextNode {
							response.PageTitle = strings.Replace(strings.TrimSpace(titleText.Data), "\n", " ", -1)
						}
					}
				}
			}

			modified = conf.pruneContent(response.Request.Request.URL, &content, cs, &doc)
			if modified {
				cs = "utf-8"
			}
		}

		conf.scanContent(content, contentType, cs, response.Tally)

		if strings.Contains(contentType, "html") {
			aclsWithCategories := copyACLSet(response.ACLs.data)
			for name, score := range response.Scores.data {
				if category, ok := conf.Categories[name]; ok && category.action == ACL && score > 0 {
					aclsWithCategories[name] = true
				}
			}
			modifiedAfterScan := conf.doFilteredPruning(response.Request.Request.URL, content, cs, aclsWithCategories, &doc)

			censorRule, _ := conf.ChooseACLCategoryAction(response.ACLs.data, response.Scores.data, conf.Threshold, "censor-words")
			if censorRule.Action == "censor-words" {
				if doc == nil {
					doc, _ = parseHTML(content, cs)
				}
				if censorHTML(doc, conf.CensoredWords) {
					modifiedAfterScan = true
				}
			}

			if modifiedAfterScan {
				b := new(bytes.Buffer)
				if err := html.Render(b, doc); err != nil {
					log.Printf("Error rendering modified content from %s: %v", response.Request.Request.URL, err)
				} else {
					content = b.Bytes()
					modified = true
				}
			}
			if modified {
				response.SetContent(content, "text/html; charset=utf-8")
			}
		}
	}
	return nil
}

func doImageHash(response *Response) error {
	content, err := response.Content(getConfig().MaxContentScanSize)
	if err != nil {
		return err
	}
	if content != nil {
		conf := getConfig()
		response.image, _, err = image.Decode(bytes.NewReader(content))
		if err != nil {
			log.Printf("Error decoding image from %v: %v", response.Request.Request.URL, err)
			return nil
		}
		hash := dhash.New(response.image)

		for _, h := range conf.ImageHashes {
			distance := dhash.Distance(hash, h.Hash)
			if distance <= h.Threshold || h.Threshold == -1 && distance <= conf.DhashThreshold {
				response.Tally[rule{imageHash, h.String()}]++
			}
		}
	}
	return nil
}

// Thumbnail returns a JPEG thumbnail of the image in the response body.
// It will be no more than maxSize pixels in width or height.
// If the response body isn't a supported image type, or if it is
// longer than MaxContentScanSize, Thumbnail returns nil.
func (resp *Response) Thumbnail(maxSize int) []byte {
	if resp.image == nil {
		content, err := resp.Content(getConfig().MaxContentScanSize)
		if err != nil {
			log.Printf("Error downloading image from %v to make thumbnail: %v", resp.Request.Request.URL, err)
		}
		if content == nil {
			return nil
		}
		resp.image, _, err = image.Decode(bytes.NewReader(content))
		if err != nil {
			log.Printf("Error decoding image from %v: %v", resp.Request.Request.URL, err)
			return nil
		}
	}

	img := resp.image
	sb := img.Bounds()
	size := sb.Dx()
	if dy := sb.Dy(); dy > size {
		size = dy
	}
	if size > maxSize {
		ratio := float64(maxSize) / float64(size)
		dst := image.NewRGBA(image.Rect(0, 0, int(ratio*float64(sb.Dx())), int(ratio*float64(sb.Dy()))))
		draw.BiLinear.Scale(dst, dst.Bounds(), img, sb, draw.Over, nil)
		img = dst
	}

	b := new(bytes.Buffer)
	err := jpeg.Encode(b, img, &jpeg.Options{Quality: 80})
	if err != nil {
		return nil
	}
	return b.Bytes()
}

// copyResponseHeader writes resp's header and status code to w.
func copyResponseHeader(w http.ResponseWriter, resp *http.Response) {
	newHeader := w.Header()
	for key, values := range resp.Header {
		if key == "Content-Length" {
			continue
		}
		for _, v := range values {
			newHeader.Add(key, v)
		}
	}

	if resp.Close {
		newHeader.Add("Connection", "close")
	}

	statusCode := resp.StatusCode
	if statusCode < 100 || statusCode >= 600 {
		statusCode = http.StatusBadGateway
	}
	w.WriteHeader(statusCode)
}

// A hijackedConn is a connection that has been hijacked (to fulfill a CONNECT
// request).
type hijackedConn struct {
	net.Conn
	io.Reader
}

func (hc *hijackedConn) Read(b []byte) (int, error) {
	return hc.Reader.Read(b)
}

func newHijackedConn(w http.ResponseWriter) (*hijackedConn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("connection doesn't support hijacking")
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	err = bufrw.Flush()
	if err != nil {
		Lce(conn.Close())
		return nil, err
	}
	return &hijackedConn{
		Conn:   conn,
		Reader: bufrw.Reader,
	}, nil
}

func (h proxyHandler) makeWebsocketConnection(w http.ResponseWriter, r *http.Request) {
	addr := r.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		// There is no port specified; we need to add it.
		port := h.connectPort
		if port == "" {
			port = "80"
		}
		addr = net.JoinHostPort(addr, port)
	}

	var err error = nil
	var serverConn net.Conn
	if ct, ok := h.rt.(*connTransport); ok {
		serverConn = ct.Conn
	} else if h.TLS {
		serverConn, err = dialWithExtraRootCerts("tcp", addr)
	} else {
		serverConn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		log.Printf("Error making websocket connection to %s: %v", addr, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Some servers are very particular about the
	// capitalization of the special WebSocket headers.
	for k, v := range r.Header {
		if strings.HasPrefix(k, "Sec-Websocket-") {
			newKey := "Sec-WebSocket-" + strings.TrimPrefix(k, "Sec-Websocket-")
			delete(r.Header, k)
			r.Header[newKey] = v
		}
	}

	err = r.Write(serverConn)
	if err != nil {
		log.Printf("Error sending websocket request to %s: %v", addr, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("Couldn't hijack client connection for websocket to %s", addr)
		http.Error(w, "Couldn't create a websocket connection", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		log.Printf("Error hijacking client connection for websocket to %s: %v", addr, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		_, err := io.Copy(conn, serverConn)
		if err != nil {
			logVerbose("debug", "unable to copy serverConnection to Connection: %s", err)
		}
		Lce(conn.Close())
	}()
	_, err = io.Copy(serverConn, bufrw)
	if err != nil {
		logVerbose("debug", "unable to copy bufrw to serverConnection: %s", err)
	}
	Lce(serverConn.Close())
}

var hopByHop = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// removeHopByHopHeaders removes header fields listed in
// http://tools.ietf.org/html/draft-ietf-httpbis-p1-messaging-14#section-7.1.3.1
func removeHopByHopHeaders(h http.Header) {
	toRemove := hopByHop
	if c := h.Get("Connection"); c != "" {
		for _, key := range strings.Split(c, ",") {
			toRemove = append(toRemove, strings.TrimSpace(key))
		}
	}
	for _, key := range toRemove {
		h.Del(key)
	}
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away. (Copied from net/http package)
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	err = tc.SetKeepAlive(true)
	if err != nil {
		logVerbose("debug", "tcp keep alive listener unable to be set true: %s", err)
	}
	err = tc.SetKeepAlivePeriod(3 * time.Minute)
	if err != nil {
		logVerbose("debug", "tcp keep alive listener unable to be set keep alive period: %s", err)
	}
	return tc, nil
}

// A swallowErrorsWriter wraps an io.Writer so that writes always "succeed".
type swallowErrorsWriter struct {
	w   io.Writer
	err error
}

func (s *swallowErrorsWriter) Write(p []byte) (n int, err error) {
	if s.err == nil {
		n, err = s.w.Write(p)
		if err != nil {
			s.err = err
		}
	}
	return len(p), nil
}

// A Request is the parameter for the Starlark filter_request function.
type Request struct {
	Request  *http.Request
	User     string
	ClientIP string

	scoresAndACLs

	frozen bool
}

// A Response is the parameter for the Starlark filter_response function.
type Response struct {
	Request  *Request
	Response *http.Response
	scoresAndACLs

	Modified bool

	// PageTitle is the content of the page's title tag.
	// It is filled in by doPhraseScan.
	PageTitle string

	image image.Image

	frozen bool
}

func (resp *Response) Headers() *http.Header {
	return &resp.Response.Header
}

func (resp *Response) Status() *int {
	return &resp.Response.StatusCode
}

func (resp *Response) Body() ([]byte, error) {
	return resp.Content(getConfig().MaxContentScanSize)
}

// Content reads the response body and returns it as a byte slice.
// If the Content-Encoding header indicates that the body is compressed,
// it will be decompressed.
// If the length of the body is more than maxLen, or it is a response to a HEAD
// request, it will return nil, nil.
func (resp *Response) Content(maxLen int) ([]byte, error) {
	if resp.Response.ContentLength > int64(maxLen) || resp.Request.Request.Method == "HEAD" {
		return nil, nil
	}

	lr := &io.LimitedReader{
		R: resp.Response.Body,
		N: int64(maxLen),
	}
	content, err := ioutil.ReadAll(lr)

	// Servers that use broken chunked Transfer-Encoding can give us unexpected EOFs,
	// even if we got all the content.
	if err == io.ErrUnexpectedEOF && resp.Response.ContentLength == -1 {
		err = nil
	}
	if err != nil {
		return nil, err
	}

	if lr.N == 0 {
		// We read maxLen without reaching the end.
		resp.Response.Body = io.NopCloser(io.MultiReader(bytes.NewReader(content), resp.Response.Body))
		return nil, nil
	}

	if resp.Response.Header.Get("Content-Encoding") == "" {
		resp.Response.ContentLength = int64(len(content))
	}
	resp.Response.Body = io.NopCloser(bytes.NewReader(content))

	if ce := resp.Response.Header.Get("Content-Encoding"); ce != "" && len(content) > 0 {
		br := bytes.NewReader(content)
		var decompressor io.Reader
		switch ce {
		case "br":
			decompressor = brotli.NewReader(br)
		case "deflate":
			decompressor = flate.NewReader(br)
		case "gzip":
			decompressor, err = gzip.NewReader(br)
			if err != nil {
				log.Printf("Error creating gzip.Reader for %v: %v", resp.Request.Request.URL, err)
				// At this point, decompressor is io.Reader(*gzip.Reader(nil)),
				// but it needs to be just io.Reader(nil), or we won't catch that it's nil
				// later on.
				decompressor = nil
			}
		default:
			log.Printf("Unrecognized Content-Encoding (%q) at %v", ce, resp.Request.Request.URL)
		}
		if decompressor != nil {
			decompressed, err := ioutil.ReadAll(decompressor)
			if err != nil {
				log.Printf("Error decompressing response body from %v: %v", resp.Request.Request.URL, err)
			} else {
				return decompressed, nil
			}
		}
	}

	return content, nil
}

// SetContent replaces the request body with the provided content, and sets
// the Content-Type header.
func (resp *Response) SetContent(data []byte, contentType string) {
	resp.Response.Header.Set("Content-Type", contentType)
	resp.Response.Header.Del("Content-Encoding")
	resp.Modified = true

	if len(data) > 1000 {
		encoding := httputil.NegotiateContentEncoding(resp.Request.Request, []string{"br", "gzip"})
		buf := new(bytes.Buffer)
		var compressor io.WriteCloser
		var err error
		switch encoding {
		case "br":
			compressor = brotli.NewWriterOptions(buf, brotli.WriterOptions{Quality: getConfig().BrotliLevel})
		case "gzip":
			compressor, err = gzip.NewWriterLevel(buf, getConfig().GZIPLevel)
			if err != nil {
				log.Println("Error creating gzip compressor:", err)
				compressor = nil
			}
		}
		if compressor != nil {
			_, err = compressor.Write(data)
			if err != nil {
				logVerbose("debug", "compressor unable ot write data: %s", err)
			}
			if err := compressor.Close(); err == nil {
				resp.Response.Body = io.NopCloser(buf)
				resp.Response.Header.Set("Content-Encoding", encoding)
				resp.Response.ContentLength = -1
				return
			}
		}
	}

	resp.Response.ContentLength = int64(len(data))
	resp.Response.Body = io.NopCloser(bytes.NewReader(data))
}
