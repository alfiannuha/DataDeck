package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func decode(t *testing.T, body []byte) (Envelope, map[string]json.RawMessage) {
	t.Helper()
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	return env, raw
}

func TestSuccessEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	Success(rec, map[string]string{"status": "healthy"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	env, raw := decode(t, rec.Body.Bytes())
	if !env.Success {
		t.Error("Success = false, want true")
	}
	if env.Error != nil {
		t.Errorf("Error = %v, want nil", env.Error)
	}
	if string(raw["error"]) != "null" {
		t.Errorf("raw error = %s, want null", raw["error"])
	}
	if string(raw["meta"]) != "{}" {
		t.Errorf("raw meta = %s, want {}", raw["meta"])
	}

	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data type = %T, want object", env.Data)
	}
	if data["status"] != "healthy" {
		t.Errorf("data.status = %v, want healthy", data["status"])
	}
}

func TestErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	InternalError(rec, "something went wrong")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	env, raw := decode(t, rec.Body.Bytes())
	if env.Success {
		t.Error("Success = true, want false")
	}
	if string(raw["data"]) != "null" {
		t.Errorf("raw data = %s, want null", raw["data"])
	}
	if env.Error == nil {
		t.Fatal("Error = nil, want populated")
	}
	if env.Error.Code != CodeInternalError {
		t.Errorf("Error.Code = %q, want %q", env.Error.Code, CodeInternalError)
	}
	if env.Error.Message != "something went wrong" {
		t.Errorf("Error.Message = %q", env.Error.Message)
	}
}

func TestValidationErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	ValidationError(rec, "connection_id is required")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	env, _ := decode(t, rec.Body.Bytes())
	if env.Success {
		t.Error("Success = true, want false")
	}
	if env.Error == nil || env.Error.Code != CodeValidationError {
		t.Fatalf("Error = %+v, want code %q", env.Error, CodeValidationError)
	}
}
