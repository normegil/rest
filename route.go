package rest

import "github.com/julienschmidt/httprouter"

type Path string

type Method string

const (
	HEAD    = Method("HEAD")
	GET     = Method("GET")
	POST    = Method("POST")
	PUT     = Method("PUT")
	DELETE  = Method("DELETE")
	OPTIONS = Method("OPTIONS")
	PATCH   = Method("PATCH")
)

type Route interface {
	Method() Method
	Path() Path
	Handler() httprouter.Handle
}

type HttpRoute struct {
	method  Method
	path    Path
	handler httprouter.Handle
}

func (r HttpRoute) Method() Method {
	return r.method
}

func (r HttpRoute) Path() Path {
	return r.path
}

func (r HttpRoute) Handler() httprouter.Handle {
	return r.handler
}

func NewRoute(method Method, path Path, handler httprouter.Handle) *HttpRoute {
	return &HttpRoute{
		method:  method,
		path:    path,
		handler: handler,
	}
}
