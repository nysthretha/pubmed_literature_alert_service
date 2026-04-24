// Package apiutil provides shared helpers for HTTP endpoints: standardised
// error responses, JSON decoding, and pagination parsing.
//
// All error responses follow the shape documented in docs/ARCHITECTURE.md:
//
//	{"error": {"code": "...", "message": "...", "fields": [...]}}
package apiutil

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorCode enumerates the stable error codes emitted in 4xx/5xx responses.
type ErrorCode string

const (
	CodeValidation   ErrorCode = "validation_error"
	CodeNotFound     ErrorCode = "not_found"
	CodeConflict     ErrorCode = "conflict"
	CodeForbidden    ErrorCode = "forbidden"
	CodeUnauthorized ErrorCode = "unauthorized"
	CodeBadRequest   ErrorCode = "bad_request"
	CodeInternal     ErrorCode = "internal_error"
)

// FieldError describes a single validation failure for a single field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError is the JSON body for any 4xx/5xx response.
type APIError struct {
	Code    ErrorCode    `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

type errorEnvelope struct {
	Error APIError `json:"error"`
}

// WriteError emits a standardised error JSON body with the given status.
func WriteError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	writeErrorWithFields(w, status, code, message, nil)
}

// WriteValidation emits a 400 with the provided field-level errors.
func WriteValidation(w http.ResponseWriter, fields []FieldError) {
	msg := "request has validation errors"
	if len(fields) == 1 {
		msg = "request has 1 validation error"
	} else if len(fields) > 1 {
		msg = "request has " + itoa(len(fields)) + " validation errors"
	}
	writeErrorWithFields(w, http.StatusBadRequest, CodeValidation, msg, fields)
}

func WriteNotFound(w http.ResponseWriter, resource string) {
	WriteError(w, http.StatusNotFound, CodeNotFound, resource+" not found")
}

func WriteConflict(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusConflict, CodeConflict, message)
}

func WriteForbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "forbidden"
	}
	WriteError(w, http.StatusForbidden, CodeForbidden, message)
}

func WriteUnauthorized(w http.ResponseWriter) {
	WriteError(w, http.StatusUnauthorized, CodeUnauthorized, "unauthorized")
}

func WriteBadRequest(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusBadRequest, CodeBadRequest, message)
}

// WriteInternal logs the underlying error and returns a generic 500.
// The error message is never exposed to the client.
func WriteInternal(w http.ResponseWriter, err error, context string) {
	slog.Error(context, "err", err)
	WriteError(w, http.StatusInternalServerError, CodeInternal, "internal error")
}

// WriteJSON serialises v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErrorWithFields(w http.ResponseWriter, status int, code ErrorCode, message string, fields []FieldError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: APIError{Code: code, Message: message, Fields: fields},
	})
}

// itoa avoids pulling strconv into a tiny package.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
