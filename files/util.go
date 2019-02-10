package files

import (
	"encoding/base64"
	"io"
	"net/http"
	"path"
)

func b64tob64url(str string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func b64urltob64(str string) (string, error) {
	bytes, err := base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func noDirListing(handler http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Clean(r.URL.Path) == "/" {
			http.NotFound(w, r)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

var robotstxt = `
User-agent: *
Disallow: /
`

func robots(handler http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Clean(r.URL.Path) == "/robots.txt" {
			io.WriteString(w, robotstxt)
			return
		}

		handler.ServeHTTP(w, r)
	})
}
