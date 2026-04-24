package apiutil

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// DecodeJSON reads the request body as JSON into dst. On failure, writes an
// appropriate 400 response and returns false; callers should return early.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err != nil {
		switch {
		case errors.Is(err, io.EOF):
			WriteBadRequest(w, "request body is empty")
		default:
			WriteBadRequest(w, "invalid JSON: "+err.Error())
		}
		return false
	}
	// Ensure there's nothing after the body.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		WriteBadRequest(w, "request body must contain a single JSON object")
		return false
	}
	return true
}
