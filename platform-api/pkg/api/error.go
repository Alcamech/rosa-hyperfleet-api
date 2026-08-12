package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIError is a typed error response. HTTPStatus drives the HTTP status code;
// Code, Message, and optional Errors are serialized to JSON under "kind":"Error".
// Reason, when set, is the fmt template used by WithReason() to build dynamic Errors.
type APIError struct {
	Code       string `json:"code"`
	HTTPStatus int    `json:"-"`
	Message    string `json:"reason"`
	Errors     any    `json:"errors,omitempty"`
	Reason     string `json:"-"`
}

// WithErrors returns a copy of e with Errors set to v for structured payloads
// (e.g. a slice of field-level validation errors).
func (e APIError) WithErrors(v any) APIError {
	e.Errors = v
	return e
}

// WithReason returns a copy of e with Errors set by applying e.Reason to args
// via fmt.Errorf. Panics if e.Reason is empty so misconfiguration is caught at
// test time.
func (e APIError) WithReason(args ...any) APIError {
	if e.Reason == "" {
		panic(fmt.Sprintf("api: WithReason() called on %q which has no Reason template", e.Code))
	}
	e.Errors = fmt.Errorf(e.Reason, args...)
	return e
}

// WriteError serializes def as a JSON error response. The return value follows
// the same contract as Write: a marshal failure is returned before headers are
// committed; a write failure after WriteHeader is unrecoverable but still
// returned so the caller can log it.
//
// When Errors implements error and marshals to "{}" or "null" (plain errors),
// reason is derived from Errors.Error() and the errors field is suppressed.
// For structured Errors (exported fields), the static Message is kept and
// Errors serializes as-is.
func WriteError(w http.ResponseWriter, def APIError) error {
	if err, ok := def.Errors.(error); ok {
		b, merr := json.Marshal(def.Errors)
		if merr != nil {
			writeFallback(w)
			return merr
		}
		if len(b) == 0 || string(b) == "{}" || string(b) == "null" {
			// Plain error: derive reason from message, suppress empty errors field.
			def.Message = err.Error()
			def.Errors = nil
		}
		// Structured error: keep the static Message and let Errors serialize as-is.
	}
	b, err := json.Marshal(struct {
		Kind string `json:"kind"`
		APIError
	}{Kind: "Error", APIError: def})
	if err != nil {
		writeFallback(w)
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(def.HTTPStatus)
	_, err = w.Write(b)
	return err
}

// fallbackBody is the pre-marshaled form of ErrInternalMarshal, populated by
// errorcodes.go's init() after ErrInternalMarshal is set.
var fallbackBody []byte

// writeFallback writes a 500 JSON body when normal serialization has failed.
// It must not call Write or WriteError to avoid circular/recursive calls.
func writeFallback(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(fallbackBody)
}
