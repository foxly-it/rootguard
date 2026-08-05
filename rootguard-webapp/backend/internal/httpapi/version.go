// =====================================================
// File: backend/internal/httpapi/version.go
// Purpose: Expose backend build metadata to frontend
// =====================================================

package httpapi

import (
	"encoding/json"
	"net/http"
)

var Version = "dev"
var Commit = "unknown"

type VersionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func VersionHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	response := VersionResponse{
		Version: Version,
		Commit:  Commit,
	}

	_ = json.NewEncoder(w).Encode(response)
}
