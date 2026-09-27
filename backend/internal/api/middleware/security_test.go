package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(okHandler())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-store",
		"Referrer-Policy":        "no-referrer",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestSecurityHeadersDoNotAddIrrelevantHeaders(t *testing.T) {
	handler := SecurityHeaders(okHandler())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	// The API serves no HTML and is loopback HTTP; these would be mechanical.
	for _, header := range []string{"Strict-Transport-Security", "Content-Security-Policy", "X-Frame-Options"} {
		if got := rec.Header().Get(header); got != "" {
			t.Errorf("%s = %q, want unset", header, got)
		}
	}
}
