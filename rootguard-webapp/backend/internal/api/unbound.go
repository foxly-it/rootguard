package api

import (
	"errors"
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleGetUnboundSettings(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyFixed(w, r, http.StatusBadGateway, core.UnboundSettings)
}

func HandleGetUnboundConfiguration(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UnboundActiveConfiguration)
}

func HandlePutUnboundSettings(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	settings, ok := decodeUnboundSettings(w, r)
	if !ok {
		return
	}
	updated, err := core.UpdateUnboundSettings(r.Context(), settings)
	if err != nil {
		writeCoreError(w, err)
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
	proxyCore(w, r, http.StatusOK, core.UnboundHistory)
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
	proxyCore(w, r, http.StatusOK, core.UnboundDiagnostics)
}

func HandleUnboundPathDiagnostics(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UnboundPathDiagnostics)
}

func HandleUnboundDiagnosticLoggingStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UnboundDiagnosticLoggingStatus)
}

func HandleStartUnboundDiagnosticLogging(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.StartUnboundDiagnosticLogging)
}

func HandleStopUnboundDiagnosticLogging(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.StopUnboundDiagnosticLogging)
}

func HandleUnboundPresets(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UnboundPresets)
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
	request, err := decodeJSON[forwardCheckRequest](w, r, 64<<10)
	if err != nil {
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
	proxyCore(w, r, http.StatusOK, core.UnboundNetworkCapabilities)
}

type customConfigRequest struct {
	Content string `json:"content"`
}

func HandleGetUnboundCustom(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UnboundCustom)
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

func HandleGetUnboundExport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UnboundExport)
}

func HandlePreviewUnboundImport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	bundle, ok := decodeUnboundBundle(w, r)
	if !ok {
		return
	}
	preview, err := core.PreviewUnboundImport(r.Context(), bundle)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func HandleApplyUnboundImport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	bundle, ok := decodeUnboundBundle(w, r)
	if !ok {
		return
	}
	settings, err := core.ApplyUnboundImport(r.Context(), bundle)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func decodeUnboundBundle(w http.ResponseWriter, r *http.Request) (coreclient.UnboundConfigBundle, bool) {
	defer r.Body.Close()
	bundle, err := decodeJSON[coreclient.UnboundConfigBundle](w, r, 130<<10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return coreclient.UnboundConfigBundle{}, false
	}
	return bundle, true
}

func HandleClassifyUnboundImportConf(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	request, ok := decodeCustomConfig(w, r)
	if !ok {
		return
	}
	result, err := core.ClassifyUnboundImportConf(r.Context(), request.Content)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func HandleUnboundDirectives(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UnboundDirectives)
}

func decodeCustomConfig(w http.ResponseWriter, r *http.Request) (customConfigRequest, bool) {
	defer r.Body.Close()
	request, err := decodeJSON[customConfigRequest](w, r, 65<<10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return customConfigRequest{}, false
	}
	return request, true
}

func decodeUnboundSettings(w http.ResponseWriter, r *http.Request) (coreclient.UnboundSettings, bool) {
	defer r.Body.Close()
	settings, err := decodeJSON[coreclient.UnboundSettings](w, r, 64<<10)
	if err != nil {
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
