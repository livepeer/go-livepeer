package server

// utils.go contains server utility functions.

import (
	"encoding/json"
	"net/http"

	"github.com/oapi-codegen/runtime"
)

// Decoder for JSON requests.
func jsonDecoder[T any](req *T, r *http.Request) error {
	return json.NewDecoder(r.Body).Decode(req)
}

// Decoder for Multipart requests.
func multipartDecoder[T any](req *T, r *http.Request) error {
	multiRdr, err := r.MultipartReader()
	if err != nil {
		return err
	}
	return runtime.BindMultipart(req, *multiRdr)
}
