package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleControlPlaneUpdateStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.ControlPlaneUpdateStatus)
}

func HandleControlPlaneUpdateCheck(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusAccepted, core.CheckControlPlaneUpdates)
}

func HandleControlPlaneUpdateInstall(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusAccepted, core.InstallControlPlaneUpdates)
}

func HandleUpdaterSelfUpdateStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UpdaterSelfUpdateStatus)
}

func HandleUpdaterSelfUpdateCheck(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusAccepted, core.CheckUpdaterSelfUpdate)
}

func HandleUpdaterSelfUpdateInstall(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusAccepted, core.InstallUpdaterSelfUpdate)
}

func HandleAttestationProxySelfUpdateStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.AttestationProxySelfUpdateStatus)
}

func HandleAttestationProxySelfUpdateCheck(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusAccepted, core.CheckAttestationProxySelfUpdate)
}

func HandleAttestationProxySelfUpdateInstall(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusAccepted, core.InstallAttestationProxySelfUpdate)
}
