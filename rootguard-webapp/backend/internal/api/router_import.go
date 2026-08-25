package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

type fritzBoxDiscoverRequest struct {
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func HandleFritzBoxDiscover(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	defer r.Body.Close()
	request, err := decodeJSON[fritzBoxDiscoverRequest](w, r, 4<<10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := core.DiscoverFritzBoxHosts(r.Context(), request.Address, request.Username, request.Password)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type reverseDNSDiscoverRequest struct {
	Networks []string `json:"networks"`
}

func HandleReverseDNSDiscover(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	defer r.Body.Close()
	request, err := decodeJSON[reverseDNSDiscoverRequest](w, r, 4<<10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := core.DiscoverReverseDNSHosts(r.Context(), request.Networks)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
