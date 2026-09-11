package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, value any) {
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
	WriteJSON(w, status, result)
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
	WriteJSON(w, http.StatusOK, result)
}

// DecodeJSON decodes exactly one JSON value of type T from r's body,
// rejecting unknown fields and any trailing data after that value - a bare
// Decode() call silently ignores everything past the first JSON value, so
// a body like {"enabled":true}{"anything"} would otherwise decode
// successfully with the second part just discarded. Replaces what used to
// be an identical decoder/DisallowUnknownFields/Decode block repeated at
// every handler in this package.
func DecodeJSON[T any](w http.ResponseWriter, r *http.Request, maxBytes int64) (T, error) {
	var value T
	err := DecodeJSONInto(w, r, maxBytes, &value)
	return value, err
}

// DecodeJSONInto is DecodeJSON's out-parameter counterpart, for callers
// decoding into an existing/shared struct value instead of a fresh local
// one - found in review: rootguard-webapp/backend/internal/httpapi had its
// own byte-identical copy of this exact strict-decode logic
// (decodeStrictJSON), duplicated because httpapi's handlers commonly
// decode into a pre-declared `var input someRequest` rather than taking
// DecodeJSON's return value directly.
func DecodeJSONInto(w http.ResponseWriter, r *http.Request, maxBytes int64, v any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("unexpected trailing data after JSON body")
	}
	return nil
}
