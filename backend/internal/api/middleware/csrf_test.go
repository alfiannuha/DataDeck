package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFRejectsCrossOriginUnsafe(t *testing.T) {
	handler := CSRF([]string{"http://localhost:3000"})(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	cases := []struct {
		name   string
		method string
		origin string
		host   string
		want   int
	}{
		{"cross-origin POST rejected", http.MethodPost, "https://evil.example", "127.0.0.1:8080", http.StatusForbidden},
		{"cross-origin multipart POST rejected", http.MethodPost, "https://evil.example", "127.0.0.1:8080", http.StatusForbidden},
		{"same-origin POST allowed", http.MethodPost, "http://127.0.0.1:8080", "127.0.0.1:8080", http.StatusOK},
		{"allowlisted dev origin allowed", http.MethodPost, "http://localhost:3000", "127.0.0.1:8080", http.StatusOK},
		{"no origin (non-browser) allowed", http.MethodPost, "", "127.0.0.1:8080", http.StatusOK},
		{"cross-origin DELETE rejected", http.MethodDelete, "https://evil.example", "127.0.0.1:8080", http.StatusForbidden},
		{"cross-origin GET allowed", http.MethodGet, "https://evil.example", "127.0.0.1:8080", http.StatusOK},
		{"null origin rejected", http.MethodPost, "null", "127.0.0.1:8080", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/v1/x", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
