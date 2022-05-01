package main

type ExampleCom Script

func (ExampleCom) SSLBump(session *TLSSession) *TLSSession {
	return session
}

func (ExampleCom) FilterRequest(request *Request) *Request {
	return request
}
func (ExampleCom) FilterResponse(response *Response) *Response {
	return response
}
func (ExampleCom) Hosts() []string {
	return []string{"www.example.com"}
}
func (ExampleCom) Name() string {
	return "example.com"
}

func (ExampleCom) Description() string {
	return "an example handler"
}

type ExampleScript Script

func (ExampleScript) SSLBump(session *TLSSession) *TLSSession     { return session }
func (ExampleScript) FilterRequest(request *Request) *Request     { return request }
func (ExampleScript) FilterResponse(response *Response) *Response { return response }
func (ExampleScript) Hosts() []string                             { return []string{"example.com"} }
func (ExampleScript) Name() string                                { return "exa" }
func (ExampleScript) Description() string                         { return "example script description" }

type ExampleHandler ScriptHandler

func (ExampleHandler) SSLBump(session *TLSSession) *TLSSession     { return session }
func (ExampleHandler) FilterRequest(request *Request) *Request     { return request }
func (ExampleHandler) FilterResponse(response *Response) *Response { return response }
func (ExampleHandler) Hosts() []string                             { return []string{"Local"} }
func (ExampleHandler) Name() string                                { return "exa" }
func (ExampleHandler) Description() string                         { return "example script handler" }
