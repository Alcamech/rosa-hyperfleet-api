package api

import (
	"encoding/json"
	"net/http"
)

// ErrInternalMarshal is written when response serialization fails before any
// headers have been committed, ensuring the client receives a proper 500 instead
// of an empty default 200.
var ErrInternalMarshal APIError

func init() {
	ErrInternalMarshal = APIError{
		Code:       "INTERNAL-001",
		HTTPStatus: http.StatusInternalServerError,
		Message:    "internal server error",
	}
	var err error
	fallbackBody, err = json.Marshal(struct {
		Kind string `json:"kind"`
		APIError
	}{Kind: "Error", APIError: ErrInternalMarshal})
	if err != nil {
		panic("api: failed to marshal fallback error body: " + err.Error())
	}
}
