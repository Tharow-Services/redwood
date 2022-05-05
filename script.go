package main

import "log"

func sslBump(session *TLSSession) {
	switch session.SNI {
	case "www.tharow.net", "tharow.net":
		session.ServerAddr = "redwood.services"
	}
}
func FilterRequest(req *Request) {
	return
}

func filterResponse(res *Response) {
	switch res.Host() {
	case "meetlookup.com":
		meetLookup(res)
	case "myapps.classlink.io", "myapps.classlink.com":
		err := ClassLink{}.MyApps(res)
		if err != nil {
			log.Printf("classlink had an error: %s", err)
		}
	case "1637314617.rsc.cdn77.org":
		domainList(res)
	}
}

func domainList(res *Response) {
	res.Headers().Del("Access-Control-Allow-Origin")
	res.Headers().Add("Access-Control-Allow-Origin", "*")
	res.SetContent([]byte{'[', ']'}, "application/json")
}

func meetLookup(res *Response) {
	if res.Request.Request.Method == "GET" {
		res.Headers().Add("Access-Control-Allow-Origin", "*")
		res.Response.StatusCode = 200
		switch res.Request.Request.URL.Path {
		case "/geolocation/", "/geolocation/2250/":
			res.SetContent([]byte{'U', 'S'}, "text/plain")
		case "/shows/":
			{
				res.SetContent([]byte{'U', 'S'}, "text/plain")
			}
		}
	}
}
