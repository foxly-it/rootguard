package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// maxControlPlaneRequestBytes bounds the /api/control-plane/{check,update}
// request body - found in review: this was the only inbound, request-facing
// JSON decode in the whole repo with no size limit at all (every other one,
// e.g. rootguard-core/internal/api/routes.go's decodeJSON or
// rootguard-webapp's decodeStrictJSON, wraps the body in
// http.MaxBytesReader). The payload is just a small map of image
// references, so this is generous, not tight.
const maxControlPlaneRequestBytes = 4 << 10

// decodeTargetOverrides reads an optional {"target_images": {...}} JSON
// body; a missing/empty body is not an error and yields no overrides.
// DisallowUnknownFields/decoder.More() match the same strict-decode
// contract every other request body in this repo already gets.
func decodeTargetOverrides(w http.ResponseWriter, r *http.Request) (map[string]string, error) {
	var payload struct {
		TargetImages map[string]string `json:"target_images"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxControlPlaneRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	if decoder.More() {
		return nil, errors.New("unexpected trailing data after JSON body")
	}
	return payload.TargetImages, nil
}

func requireBearer(token string, next http.Handler) http.Handler {
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		actual := sha256.Sum256([]byte(provided))
		if provided == "" || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

// handleControlPlaneAction wraps StartCheck/StartUpdate - found in review:
// the two /api/control-plane/{check,update} handlers were byte-identical
// apart from which manager method they called, decode-error/errBusy/
// errTargetOverrideNotAllowlisted/other-error handling included.
func handleControlPlaneAction(action func(map[string]string) (status, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		overrides, err := decodeTargetOverrides(w, r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		next, err := action(overrides)
		if errors.Is(err, errBusy) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, errTargetOverrideNotAllowlisted) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, next)
	}
}
