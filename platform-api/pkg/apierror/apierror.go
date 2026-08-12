package apierror

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
		panic(fmt.Sprintf("apierror: WithReason() called on %q which has no Reason template", e.Code))
	}
	e.Errors = fmt.Errorf(e.Reason, args...)
	return e
}

// Write serializes def as a JSON error response.
//
// When Errors implements error, reason is derived from Errors.Error() so the
// top-level field always carries full detail. If the concrete Errors value has
// no exported fields (e.g. errors.New, fmt.Errorf) its JSON representation
// would be "{}", which adds no value; Write suppresses it from the output so
// that clients only see the populated reason and not an empty errors object.
func Write(w http.ResponseWriter, def APIError) {
	if err, ok := def.Errors.(error); ok {
		b, _ := json.Marshal(def.Errors)
		if len(b) == 0 || string(b) == "{}" || string(b) == "null" {
			// Plain error: derive reason from message, suppress empty errors field.
			def.Message = err.Error()
			def.Errors = nil
		}
		// Structured error: keep the static Message and let Errors serialize as-is.
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(def.HTTPStatus)
	_ = json.NewEncoder(w).Encode(struct {
		Kind string `json:"kind"`
		APIError
	}{Kind: "Error", APIError: def})
}
