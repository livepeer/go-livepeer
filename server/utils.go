package server

// utils.go contains server utility functions.

import (
	"encoding/json"
	"net/http"
)

// Decoder for JSON requests.
func jsonDecoder[T any](req *T, r *http.Request) error {
	return json.NewDecoder(r.Body).Decode(req)
}
