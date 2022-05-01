package main

import (
	"fmt"
	"log"
	"strings"
)

type Scripting interface {
	SSLBump(session *TLSSession) *TLSSession
	FilterRequest(request *Request) *Request
	FilterResponse(response *Response) *Response
	Hosts() []string
	Name() string
	Description() string
}

// Script blank simple script
type Script struct{ Scripting }

func (Script) SSLBump(session *TLSSession) *TLSSession     { return session }
func (Script) FilterRequest(request *Request) *Request     { return request }
func (Script) FilterResponse(response *Response) *Response { return response }
func (Script) Hosts() []string                             { return []string{} }
func (Script) Name() string                                { return "Default" }
func (Script) Description() string                         { return "Default" }

// ScriptHandler a blank example script handler
type ScriptHandler struct{ Scripting }

func (ScriptHandler) SSLBump(session *TLSSession) *TLSSession     { return session }
func (ScriptHandler) FilterRequest(request *Request) *Request     { return request }
func (ScriptHandler) FilterResponse(response *Response) *Response { return response }
func (ScriptHandler) Hosts() []string                             { return []string{} }
func (ScriptHandler) Name() string                                { return "Default" }
func (ScriptHandler) Description() string                         { return "Default" }

var Scripts = []Scripting{
	ExampleScript{},
}
var ScriptHandlers = []Scripting{
	ExampleHandler{},
}

type ScriptingHandler ScriptHandler

func CheckScripts() {
	log.Print("Running Script Handler Init, Checking Scripts")
	for _, script := range Scripts {
		CheckScript(script)
	}
	log.Print("All Scripts Have Passed Checking Script Handlers")
	for _, handler := range ScriptHandlers {
		CheckScript(handler)
	}
	log.Print("All Script Handlers Have Passed")
}

func (h ScriptingHandler) SSLBump(session *TLSSession) *TLSSession {
	// Find the first Script able to process host
	var sec = h.SelectScript(session.SNI)
	// run script
	session = sec.SSLBump(session)
	for _, handler := range h.SelectHandlers() {
		session = handler.SSLBump(session)
	}
	return session
}

func (h ScriptingHandler) FilterRequest(request *Request) *Request {
	return request
}
func (h ScriptingHandler) FilterResponse(response *Response) *Response {
	return response
}
func (h ScriptingHandler) Hosts() []string {
	return []string{}
}

func (h ScriptingHandler) Name() string {
	return "Root Handler"
}

func (h ScriptingHandler) Description() string {
	return "The Root Script Handler for redwood proxy"
}

// Script Handler Functions

func (h ScriptingHandler) SelectHandlers() []Scripting {
	var setS stringSet = getConfig().EnabledScriptHandlers
	var ret []Scripting
	for _, handler := range ScriptHandlers {
		if !setS.contains(handler.Name()) {
			continue
		}
		//if !MatchHost(script.Hosts(), host) {continue}
		ret = append(ret, handler)
	}
	if len(ret) == 0 {
		ret = append(ret, ScriptHandler{})
	}
	return ret
}

func (h ScriptingHandler) SelectScript(host string) Scripting {
	var setS stringSet = getConfig().EnabledScripts
	for _, script := range Scripts {
		if !setS.contains(script.Name()) {
			continue
		}
		if !MatchHost(script.Hosts(), host) {
			continue
		}
		return script
	}
	return Script{}
}

func MatchHost(hosts stringSet, host string) bool {
	// if enabled host list for script returns nil
	// interpret as it being disabled
	if hosts == nil {
		return false
	}
	if len(hosts) == 1 {
		return strings.HasSuffix(host, hosts[0])
	}
	return hosts.contains(host)
}

func CheckScript(s Scripting) {
	var doPanic bool
	var err = "unrecoverable error"
	var str = "scripting handler: script:[%s] has invalid configuration: "
	if s.Name() == "" {
		str += "Name() returned blank,"
		doPanic = true
	}
	if len(s.Hosts()) == 0 {
		str += "Hosts() returned []string{}: this configuration is reserved for the Root Script Handler,"
		doPanic = true
	}
	if len(s.Hosts()) == 1 && s.Hosts()[0] == "" {
		str += "Hosts() returned []string{\"\"}: host catch all is not allowed please use ScriptingHandler.RegisterHandler(Scripting), "
		doPanic = true
	}
	if doPanic {
		panic(fmt.Errorf(str+err, s.Name()))
	}
}
