package rest

import (
	"net/http"
)

func RequestLogger(log Logger, h http.Handler) http.Handler {
	if nil == log {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.WithField("request", r).Printf("Request received")
		h.ServeHTTP(w, r)
	})
}

type Logger interface {
	WithField(string, interface{}) Logger
	Printf(string, ...interface{})
}
