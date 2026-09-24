// Package httpx holds the JSON response envelope shared by all handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type Envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

func JSON(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func OK(w http.ResponseWriter, status int, message string, data any) {
	JSON(w, status, Envelope{Success: true, Message: message, Data: data})
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, Envelope{Success: false, Message: message})
}

func ValidationError(w http.ResponseWriter, errs map[string]string) {
	JSON(w, http.StatusUnprocessableEntity, Envelope{Success: false, Message: "validation failed", Errors: errs})
}

// Decode reads a JSON body, rejecting unknown fields and bodies over 1MB.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is empty")
		}
		return err
	}
	return nil
}
