package rest_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/normegil/rest"
)

func TestURLConstructor(t *testing.T) {
	testcases := []struct {
		method string
		url    string
	}{
		{"GET", "http://localhost/"},
		{"GET", "http://www.google.be"},
		{"GET", "http://user@pass:www.google.be"},
		{"GET", "ftp://user@pass:www.google.be"},
		{"GET", "http://127.0.0.1/resource"},
		{"GET", "https://127.0.0.1/resource"},
		{"GET", "http://127.0.0.1/resource/2ndlevel"},
		{"POST", "http://127.0.0.1/resource/2ndlevel"},
		{"PUT", "http://127.0.0.1/resource/2ndlevel"},
		{"PATCH", "http://127.0.0.1/resource/2ndlevel"},
		{"DELETE", "http://127.0.0.1/resource/2ndlevel"},
	}
	for _, testdata := range testcases {
		t.Run(testdata.method+": "+testdata.url, func(t *testing.T) {
			request := httptest.NewRequest(testdata.method, testdata.url, strings.NewReader(""))
			result := httptest.NewRecorder()
			handler := rest.URLContructor(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				url := r.Context().Value(rest.FULL_URL_KEY)
				if nil == url {
					t.Fatal("Could not load url from the Context attached to the Request")
				}
				if testdata.url != url {
					t.Errorf("URL extracted (%s) doesn't meet the expected result (%s)", url, testdata.url)
				}
			}))
			handler.ServeHTTP(result, request)
		})
	}
}
