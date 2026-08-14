package api

import (
	"context"
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// proxyCore runs a zero-argument Core client call and writes its result as
// JSON on success, or the standard mapped Core error (writeCoreError) on
// failure - the shape shared by every simple GET-and-relay or
// trigger-and-relay handler.
func proxyCore[T any](w http.ResponseWriter, r *http.Request, status int, fn func(context.Context) (T, error)) {
	result, err := fn(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, status, result)
}

// proxyFixed is proxyCore's counterpart for the handlers that report a
// failure with a single fixed status via http.Error instead of
// writeCoreError's Core-error-code mapping.
func proxyFixed[T any](w http.ResponseWriter, r *http.Request, errStatus int, fn func(context.Context) (T, error)) {
	result, err := fn(r.Context())
	if err != nil {
		http.Error(w, err.Error(), errStatus)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
