package api

import (
	"encoding/json"
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleGetAdGuardStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyFixed(w, r, http.StatusBadGateway, core.AdGuardStatus)
}

func HandleBootstrapAdGuard(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyFixed(w, r, http.StatusBadGateway, core.BootstrapAdGuard)
}

func HandleGetAdGuardFilterReport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyFixed(w, r, http.StatusBadGateway, core.AdGuardFilterReport)
}

func HandleSetAdGuardFiltering(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	// Found via code review: {} used to decode to Enabled=false with no way
	// to tell it apart from an explicit {"enabled":false}, silently
	// disabling filtering - same class of bug as the protection endpoint
	// below. decoder.More() rejects a second JSON value appended after the
	// first, which a bare Decode call would otherwise silently ignore.
	if input.Enabled == nil {
		http.Error(w, "enabled is required", http.StatusBadRequest)
		return
	}
	if decoder.More() {
		http.Error(w, "unexpected trailing data after the JSON body", http.StatusBadRequest)
		return
	}
	status, err := core.SetAdGuardFiltering(r.Context(), *input.Enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// adGuardProtectionDurations mirrors the same allowlist enforced on the
// Core side (rootguard-core/internal/api/routes.go) - defense in depth so a
// malformed request never even reaches Core. Found via code review: {}
// used to decode to enabled=false with no way to tell it apart from an
// explicit request, silently disabling protection indefinitely.
var adGuardProtectionDurations = map[int64]bool{0: true, 600: true, 3600: true}

func HandleSetAdGuardProtection(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	var input struct {
		Enabled         *bool `json:"enabled"`
		DurationSeconds int64 `json:"duration_seconds"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if input.Enabled == nil {
		http.Error(w, "enabled is required", http.StatusBadRequest)
		return
	}
	if !adGuardProtectionDurations[input.DurationSeconds] {
		http.Error(w, "duration_seconds must be one of 0, 600, 3600", http.StatusBadRequest)
		return
	}
	if *input.Enabled && input.DurationSeconds != 0 {
		http.Error(w, "duration_seconds must be 0 when enabling protection", http.StatusBadRequest)
		return
	}
	if decoder.More() {
		http.Error(w, "unexpected trailing data after the JSON body", http.StatusBadRequest)
		return
	}
	status, err := core.SetAdGuardProtection(r.Context(), *input.Enabled, input.DurationSeconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, status)
}
