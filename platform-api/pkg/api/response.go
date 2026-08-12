package api

import (
	"encoding/json"
	"net/http"
)

// Write serializes data as a JSON response with the given HTTP status code.
// If data is nil, only the status code is written (suitable for 204 No Content).
// Encoding is done before committing headers; if marshal fails the caller can
// still write an error response. If the write itself fails (headers already
// committed), the caller should log and move on — the connection is broken.
func Write(w http.ResponseWriter, status int, data any) error {
	if data == nil {
		w.WriteHeader(status)
		return nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(b)
	return err
}
