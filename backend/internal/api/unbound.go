package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleGetUnboundSettings(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	settings, err := core.UnboundSettings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func HandleGetUnboundConfiguration(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	configuration, err := core.UnboundActiveConfiguration(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, configuration)
}

func HandlePutUnboundSettings(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var settings coreclient.UnboundSettings
	if err := decoder.Decode(&settings); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := core.UpdateUnboundSettings(r.Context(), settings)
	if err != nil {
		status := http.StatusBadGateway
		var apiError *coreclient.APIError
		if errors.As(err, &apiError) && apiError.StatusCode >= 400 && apiError.StatusCode < 500 {
			status = apiError.StatusCode
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func HandlePreviewUnboundSettings(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	settings, ok := decodeUnboundSettings(w, r)
	if !ok {
		return
	}
	preview, err := core.PreviewUnboundSettings(r.Context(), settings)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func HandleUnboundHistory(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	history, err := core.UnboundHistory(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func HandleRestoreUnboundVersion(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	settings, err := core.RestoreUnboundVersion(r.Context(), r.PathValue("id"))
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func HandleUnboundDiagnostics(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	report, err := core.UnboundDiagnostics(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func HandleUnboundPresets(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	presets, err := core.UnboundPresets(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, presets)
}

func HandleUnboundAdvice(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	settings, ok := decodeUnboundSettings(w, r)
	if !ok {
		return
	}
	advice, err := core.UnboundAdvice(r.Context(), settings)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, advice)
}

type forwardCheckRequest struct {
	Zones []coreclient.UnboundForwardZone `json:"zones"`
}

func HandleUnboundForwardCheck(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var request forwardCheckRequest
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	checks, err := core.CheckUnboundForwardTargets(r.Context(), request.Zones)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, checks)
}

func HandleUnboundNetworkCapabilities(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	result, err := core.UnboundNetworkCapabilities(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type customConfigRequest struct {
	Content string `json:"content"`
}

func HandleGetUnboundCustom(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	document, err := core.UnboundCustom(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func HandlePreviewUnboundCustom(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	request, ok := decodeCustomConfig(w, r)
	if !ok {
		return
	}
	preview, err := core.PreviewUnboundCustom(r.Context(), request.Content)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func HandlePutUnboundCustom(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	request, ok := decodeCustomConfig(w, r)
	if !ok {
		return
	}
	document, err := core.UpdateUnboundCustom(r.Context(), request.Content)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func HandleUnboundDirectives(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	directives, err := core.UnboundDirectives(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, directives)
}

func decodeCustomConfig(w http.ResponseWriter, r *http.Request) (customConfigRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65<<10))
	decoder.DisallowUnknownFields()
	var request customConfigRequest
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return customConfigRequest{}, false
	}
	return request, true
}

func decodeUnboundSettings(w http.ResponseWriter, r *http.Request) (coreclient.UnboundSettings, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var settings coreclient.UnboundSettings
	if err := decoder.Decode(&settings); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return coreclient.UnboundSettings{}, false
	}
	return settings, true
}

func writeCoreError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiError *coreclient.APIError
	if errors.As(err, &apiError) && apiError.StatusCode >= 400 && apiError.StatusCode < 500 {
		status = apiError.StatusCode
	}
	http.Error(w, err.Error(), status)
}
