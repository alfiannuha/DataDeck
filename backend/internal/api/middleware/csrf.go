package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// CSRF rejects browser-originated state-changing requests whose Origin is
// neither the server's own origin (same-origin) nor an explicit allowlisted
// origin. CORS alone is not sufficient: a cross-origin `multipart/form-data`
// POST is a CORS "simple request" that a browser will send without a preflight,
// so the side effect would reach the handler even though the response is
// unreadable. This middleware blocks that path.
//
// Requests with no Origin header (non-browser clients: curl, CI, server-to-server)
// are allowed; the local-first threat model is a malicious web page, which always
// sends Origin for unsafe methods. HTTP safe methods and OPTIONS are not filtered.
func CSRF(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isUnsafeMethod(r.Method) {
				if origin := r.Header.Get("Origin"); origin != "" {
					if _, ok := allowed[origin]; !ok && !sameOrigin(origin, r.Host) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"success":false,"data":null,"error":{"code":"CROSS_ORIGIN_REJECTED","message":"cross-origin request rejected"},"meta":{}}`))
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// sameOrigin reports whether the Origin header matches the request Host
// (same-origin requests from the embedded frontend). Comparison is
// case-insensitive on the host:port authority.
func sameOrigin(origin, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsed.Host != "" && strings.EqualFold(parsed.Host, host)
}
