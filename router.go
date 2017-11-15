package rest

import (
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type Router struct {
	Router *httprouter.Router
	logger Logger
}

func New() *Router {
	return &Router{
		Router: httprouter.New(),
	}
}

func (r *Router) SetLogger(log Logger) {
	r.logger = log
}

func (r *Router) Register(routes []Route) error {
	for _, route := range routes {
		switch route.Method() {
		case "HEAD":
			r.Router.HEAD(route.Path(), route.Handler())
		case "GET":
			r.Router.GET(route.Path(), route.Handler())
		case "POST":
			r.Router.POST(route.Path(), route.Handler())
		case "PUT":
			r.Router.PUT(route.Path(), route.Handler())
		case "DELETE":
			r.Router.DELETE(route.Path(), route.Handler())
		case "OPTIONS":
			r.Router.OPTIONS(route.Path(), route.Handler())
		case "PATCH":
			r.Router.PATCH(route.Path(), route.Handler())
		default:
			return errors.New("HTTP Method not supported {method: " + route.Method() + "; path: " + route.Path() + "}")
		}
	}
	return nil
}

func (r *Router) Listen(port int, withMiddleware func(http.Handler) http.Handler) error {
	var handler http.Handler
	handler = r.Router
	if nil != r.logger {
		handler = RequestLogger(r.logger, handler)
	}
	handler = URLContructor(withMiddleware(handler))

	if err := http.ListenAndServe(":"+strconv.Itoa(port), handler); nil != err {
		return errors.Wrapf(err, "Error while Listening on %d", port)
	}
	return nil
}
