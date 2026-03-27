package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func New(target *url.URL) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)
	p.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("proxy error to %s: %v", target.String(), err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream service unavailable"}`))
	}
	return p
}

func StripTrustedHeaders(req *http.Request) {
	req.Header.Del("X-User-ID")
	req.Header.Del("X-Role")
	req.Header.Del("X-Gateway-Secret")
}
