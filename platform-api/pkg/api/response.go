package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Write serializes data as a JSON response with the given HTTP status code.
// If data is nil, only the status code is written (suitable for 204 No Content).
// If marshaling fails, a 500 error response is written before the error is
// returned so the client never receives an empty default 200. If the write
// itself fails (headers already committed), the error is returned for the
// caller to log — the connection is already broken.
func Write(w http.ResponseWriter, status int, data any) error {
	if data == nil {
		w.WriteHeader(status)
		return nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		if werr := WriteError(w, ErrInternalMarshal); werr != nil {
			return fmt.Errorf("marshal: %w; write error response: %v", err, werr)
		}
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(b)
	return err
}
