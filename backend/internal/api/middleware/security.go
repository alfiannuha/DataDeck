package middleware

import "net/http"

// SecurityHeaders applies conservative response headers appropriate to a
// local-first API. The API returns sensitive local data (schema, query results,
// history), so responses must not be cached by the browser or an intermediary.
//
// Deliberately omitted: HSTS (the daemon is loopback HTTP), CSP and
// X-Frame-Options (no HTML is served by the API), and COOP/COEP (no cross-origin
// isolation requirement). Adding them mechanically would provide no protection
// for this architecture.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Cache-Control", "no-store")
		header.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
