package proxy

import (
	loadbalancer "cloud-test-task/loadBalancer"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
)

// NewReverseProxy основная функция проксирования запросов
// если в бакете для текущего клиента есть токены, то запросы передаются в него, если нет, то передаются на следующий сервер
func NewReverseProxy(lb *loadbalancer.LoadBalancer) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director:     requestHandler(lb),
		ErrorHandler: errorHandler(),
	}
}

func requestHandler(balancer *loadbalancer.LoadBalancer) func(req *http.Request) {
	return func(req *http.Request) {
		backend, tokensEmpty := balancer.NextBackend()
		if backend == nil {
			if tokensEmpty {
				req.URL.Host = "rate-limited"
				return
			}
			req.URL.Host = ""
			return
		}
		req.URL.Scheme = backend.Url.Scheme
		req.URL.Host = backend.Url.Host
		log.Printf("Forwarding request to %v", backend.Url)
	}
}

func errorHandler() func(w http.ResponseWriter, r *http.Request, err error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if r.URL.Host == "" {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("Backends are not available"))
		} else if len(r.URL.Host) > 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limit exceeded"))
			log.Printf("[INFO] all buckets are empty")
			return
		} else {
			w.WriteHeader(r.Response.StatusCode)
			_, err := io.Copy(w, r.Response.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
	}
}
