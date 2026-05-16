package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func New(target *url.URL) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)
	director := p.Director
	p.Director = func(req *http.Request) {
		director(req)
		req.Host = target.Host
	}
	p.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Access-Control-Allow-Origin")
		resp.Header.Del("Access-Control-Allow-Credentials")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Expose-Headers")
		return nil
	}
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
