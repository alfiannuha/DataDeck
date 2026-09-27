package middleware

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// RequestIDHeader is the response header that carries the request correlation id.
const RequestIDHeader = "X-Request-ID"

// RequestID assigns a request id (honoring an incoming X-Request-ID header,
// otherwise generating one), stores it in the request context and echoes it
// back on the response.
func RequestID(next http.Handler) http.Handler {
	return chimiddleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(RequestIDHeader, chimiddleware.GetReqID(r.Context()))
		next.ServeHTTP(w, r)
	}))
}
