package rest

import (
	"context"
	"net/http"
)

const FULL_URL_KEY = "RequestURL"

func URLContructor(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.String()
		ctx := context.WithValue(r.Context(), FULL_URL_KEY, url)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
