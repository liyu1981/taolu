// Package logreq provides HTTP request logging middleware.
package logreq

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder captures the HTTP status code written by the handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware returns an http.Handler that logs each request's method, path,
// status code, and duration. Requests are logged to the default logger.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		duration := time.Since(start)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, duration)
	})
}
