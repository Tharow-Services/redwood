package main

import (
	"fmt"
	"log"
	"strings"
)

type Scripting interface {
	SSLBump(session **TLSSession) error
	FilterRequest(request **Request) error
	FilterResponse(response **Response) error
	Hosts() []string
	Name() string
	Description() string
}

// Script blank simple script
type Script struct{ Scripting }

func (Script) SSLBump(**TLSSession) error      { return nil }
func (Script) FilterRequest(**Request) error   { return nil }
func (Script) FilterResponse(**Response) error { return nil }
func (Script) Hosts() []string                 { return []string{} }
func (Script) Name() string                    { return "Default" }
func (Script) Description() string             { return "Default" }

// ScriptHandler a blank example script handler
type ScriptHandler struct{ Scripting }

func (ScriptHandler) SSLBump(**TLSSession) error      { return nil }
func (ScriptHandler) FilterRequest(**Request) error   { return nil }
func (ScriptHandler) FilterResponse(**Response) error { return nil }
func (ScriptHandler) Hosts() []string                 { return []string{} }
func (ScriptHandler) Name() string                    { return "Default" }
func (ScriptHandler) Description() string             { return "Default" }

var Scripts = []Scripting{
	ClassLink{},
}
var ScriptHandlers = []Scripting{
	ScriptHandler{},
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

func (h ScriptingHandler) SSLBump(session **TLSSession) error {
	var s = **session
	// Find the first Script able to process host
	var sec = h.SelectScript(s.SNI)
	// run script
	sec.SSLBump(session)
	for _, handler := range h.SelectHandlers() {
		handler.SSLBump(session)
	}
}

func (h ScriptingHandler) FilterRequest(request **Request) {
	var r = *request
	// find script to process host
	h.SelectScript(r.Request.Host).FilterRequest(request)
	// run other handlers
	for _, handler := range h.SelectHandlers() {
		handler.FilterRequest(request)
	}
}
func (h ScriptingHandler) FilterResponse(response **Response) {
	var r = *response
	// find script to process host
	h.SelectScript(r.Request.Request.Host).FilterResponse(response)
	// run other handlers
	for _, handler := range h.SelectHandlers() {
		handler.FilterResponse(response)
	}
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
	//log.Println("Select Script: enabled scripts", setS)
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
