package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestRecovererReturnsSanitizedEnvelope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(Recoverer(logger))
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("kaboom-secret") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "kaboom-secret") {
		t.Fatalf("panic value leaked to client body: %s", rec.Body.String())
	}

	var env struct {
		Success bool `json:"success"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if env.Success {
		t.Error("success = true, want false")
	}
	if env.Error == nil {
		t.Fatal("error = nil, want populated")
	}
	if env.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("error.code = %q, want INTERNAL_ERROR", env.Error.Code)
	}
	if env.Error.Message != "internal server error" {
		t.Errorf("error.message = %q, want sanitized message", env.Error.Message)
	}
}

func TestRequestLoggerDoesNotLogSensitiveData(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(RequestLogger(logger))
	r.Post("/submit", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/submit?token=query-secret", strings.NewReader(`{"password":"body-secret"}`))
	req.Header.Set("Authorization", "Bearer header-secret")
	r.ServeHTTP(httptest.NewRecorder(), req)

	logged := buf.String()
	for _, secret := range []string{"header-secret", "body-secret", "query-secret"} {
		if strings.Contains(logged, secret) {
			t.Errorf("request log leaked sensitive value %q: %s", secret, logged)
		}
	}
	if !strings.Contains(logged, "http_request") {
		t.Errorf("expected a structured request log line, got: %s", logged)
	}
}
