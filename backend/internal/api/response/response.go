// Package response implements the standard DataDeck API envelope defined by
// docs/api-contract.md.
package response

import (
	"encoding/json"
	"net/http"
)

// Error codes defined by docs/api-contract.md.
const (
	CodeValidationError = "VALIDATION_ERROR"
	CodeInternalError   = "INTERNAL_ERROR"
)

// APIError is the error object carried by the standard envelope.
type APIError struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Position *int   `json:"position,omitempty"`
}

// Envelope is the standard JSON response wrapper for every API response.
type Envelope struct {
	Success bool           `json:"success"`
	Data    any            `json:"data"`
	Error   *APIError      `json:"error"`
	Meta    map[string]any `json:"meta"`
}

// ErrorEnvelope is the shape of a failed response. It exists so the OpenAPI
// schema for failures is accurate (data is always null).
type ErrorEnvelope struct {
	Success bool           `json:"success" example:"false"`
	Data    any            `json:"data"` // always null
	Error   *APIError      `json:"error"`
	Meta    map[string]any `json:"meta"`
}

// Success writes a 200 OK envelope with the given data payload.
func Success(w http.ResponseWriter, data any) {
	write(w, http.StatusOK, Envelope{
		Success: true,
		Data:    data,
		Error:   nil,
		Meta:    map[string]any{},
	})
}

// SuccessWithMeta writes a 200 OK envelope with data and pagination/other meta.
func SuccessWithMeta(w http.ResponseWriter, data any, meta map[string]any) {
	if meta == nil {
		meta = map[string]any{}
	}
	write(w, http.StatusOK, Envelope{
		Success: true,
		Data:    data,
		Error:   nil,
		Meta:    meta,
	})
}

// ValidationError writes a 400 Bad Request envelope.
func ValidationError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusBadRequest, CodeValidationError, message)
}

// InternalError writes a 500 Internal Server Error envelope. The caller is
// responsible for ensuring the message does not leak sensitive internals.
func InternalError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusInternalServerError, CodeInternalError, message)
}

// WriteError writes an error envelope with the given HTTP status and code.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteErrorObject(w, status, APIError{Code: code, Message: message})
}

// WriteErrorObject writes an error envelope from a fully formed APIError, for
// cases that carry extra detail such as a SQL error position.
func WriteErrorObject(w http.ResponseWriter, status int, apiErr APIError) {
	write(w, status, Envelope{
		Success: false,
		Data:    nil,
		Error:   &apiErr,
		Meta:    map[string]any{},
	})
}

func write(w http.ResponseWriter, status int, env Envelope) {
	body, err := json.Marshal(env)
	if err != nil {
		http.Error(w, `{"success":false,"data":null,"error":{"code":"INTERNAL_ERROR","message":"failed to encode response"},"meta":{}}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
