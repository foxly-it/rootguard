package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/foxly-it/rootguard-core/internal/adguard"
	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/backuprestore"
	"github.com/foxly-it/rootguard-core/internal/controlplane"
	"github.com/foxly-it/rootguard-core/internal/docker"
	"github.com/foxly-it/rootguard-core/internal/installer"
	"github.com/foxly-it/rootguard-core/internal/routerimport"
	"github.com/foxly-it/rootguard-core/internal/stack"
	"github.com/foxly-it/rootguard-core/internal/unbound"
	"github.com/foxly-it/rootguard-core/internal/updater"
)

type Dependencies struct {
	Token             string
	Unbound           *unbound.Manager
	AdGuard           *adguard.Manager
	Installer         *installer.Manager
	Updater           *updater.Manager
	ControlPlane      *controlplane.Client
	UpdaterSelfUpdate *updater.Manager
	AdGuardDNSAddress string
	BackupExporter    *backupexport.Exporter
	BackupRestorer    *backuprestore.Manager
}

func RegisterRoutes(deps Dependencies) http.Handler {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/system", systemHandler)
	apiMux.HandleFunc("GET /api/docker/status", dockerStatusHandler)
	apiMux.HandleFunc("GET /api/stack/status", stackStatusHandler)
	apiMux.HandleFunc("GET /api/dashboard", dashboardHandler)
	apiMux.HandleFunc("GET /api/services", servicesHandler)
	apiMux.HandleFunc("GET /api/services/{name}/logs", serviceLogsHandler)
	apiMux.HandleFunc("POST /api/services/{name}/{action}", serviceActionHandler)
	apiMux.HandleFunc("GET /api/installation", installationStatusHandler(deps.Installer))
	apiMux.HandleFunc("POST /api/installation/preflight", installationPreflightHandler(deps.Installer))
	apiMux.HandleFunc("POST /api/installation/deploy", installationDeployHandler(deps.Installer))
	apiMux.HandleFunc("GET /api/updates", updateStatusHandler(deps.Updater))
	apiMux.HandleFunc("POST /api/updates/check", updateCheckHandler(deps.Updater))
	apiMux.HandleFunc("POST /api/updates/{name}", updateServiceHandler(deps.Updater))
	apiMux.HandleFunc("GET /api/backups", backupStatusHandler(deps.Updater))
	apiMux.HandleFunc("PUT /api/backups/settings", putBackupSettingsHandler(deps.Updater))
	apiMux.HandleFunc("GET /api/cleanup/preview", cleanupPreviewHandler(deps.Updater))
	apiMux.HandleFunc("POST /api/cleanup", runCleanupHandler(deps.Updater))
	apiMux.HandleFunc("POST /api/backups/export", backupExportHandler(deps.BackupExporter, deps.Updater))
	apiMux.HandleFunc("POST /api/backups/restore/preview", backupRestorePreviewHandler(deps.BackupRestorer))
	apiMux.HandleFunc("POST /api/backups/restore", backupRestoreHandler(deps.BackupRestorer, deps.Updater))
	apiMux.HandleFunc("GET /api/control-plane-updates", controlPlaneStatusHandler(deps.ControlPlane))
	apiMux.HandleFunc("POST /api/control-plane-updates/check", controlPlaneCheckHandler(deps.ControlPlane))
	apiMux.HandleFunc("POST /api/control-plane-updates/install", controlPlaneUpdateHandler(deps.ControlPlane))
	apiMux.HandleFunc("GET /api/updater-updates", updateStatusHandler(deps.UpdaterSelfUpdate))
	apiMux.HandleFunc("POST /api/updater-updates/check", updateCheckHandler(deps.UpdaterSelfUpdate))
	apiMux.HandleFunc("POST /api/updater-updates/install", updaterSelfUpdateInstallHandler(deps.UpdaterSelfUpdate, deps.ControlPlane))
	apiMux.HandleFunc("GET /api/unbound/settings", getUnboundSettingsHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/config", getUnboundConfigurationHandler(deps.Unbound))
	apiMux.HandleFunc("PUT /api/unbound/settings", putUnboundSettingsHandler(deps.Unbound))
	apiMux.HandleFunc("POST /api/unbound/preview", previewUnboundSettingsHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/history", unboundHistoryHandler(deps.Unbound))
	apiMux.HandleFunc("POST /api/unbound/history/{id}/restore", restoreUnboundVersionHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/export", getUnboundExportHandler(deps.Unbound))
	apiMux.HandleFunc("POST /api/unbound/import/preview", previewUnboundImportHandler(deps.Unbound))
	apiMux.HandleFunc("POST /api/unbound/import", applyUnboundImportHandler(deps.Unbound))
	apiMux.HandleFunc("POST /api/unbound/import-conf", classifyUnboundImportConfHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/diagnostics", unboundDiagnosticsHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/path-diagnostics", unboundPathDiagnosticsHandler(deps.Unbound, deps.AdGuardDNSAddress))
	apiMux.HandleFunc("GET /api/unbound/diagnostic-logging", unboundDiagnosticLoggingStatusHandler(deps.Unbound))
	apiMux.HandleFunc("POST /api/unbound/diagnostic-logging", startUnboundDiagnosticLoggingHandler(deps.Unbound))
	apiMux.HandleFunc("DELETE /api/unbound/diagnostic-logging", stopUnboundDiagnosticLoggingHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/presets", unboundPresetsHandler)
	apiMux.HandleFunc("POST /api/unbound/advice", unboundAdviceHandler)
	apiMux.HandleFunc("POST /api/unbound/forward-check", unboundForwardCheckHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/network-capabilities", unboundNetworkCapabilitiesHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/custom", getUnboundCustomHandler(deps.Unbound))
	apiMux.HandleFunc("POST /api/unbound/custom/preview", previewUnboundCustomHandler(deps.Unbound))
	apiMux.HandleFunc("PUT /api/unbound/custom", putUnboundCustomHandler(deps.Unbound))
	apiMux.HandleFunc("GET /api/unbound/directives", unboundDirectivesHandler)
	apiMux.HandleFunc("POST /api/router-import/fritzbox/discover", fritzBoxDiscoverHandler)
	apiMux.HandleFunc("POST /api/router-import/reverse-dns/discover", reverseDNSDiscoverHandler)
	apiMux.HandleFunc("GET /api/adguard/status", getAdGuardStatusHandler(deps.AdGuard))
	apiMux.HandleFunc("GET /api/adguard/filter-report", getAdGuardFilterReportHandler(deps.AdGuard))
	apiMux.HandleFunc("POST /api/adguard/filtering", setAdGuardFilteringHandler(deps.AdGuard))
	apiMux.HandleFunc("POST /api/adguard/protection", setAdGuardProtectionHandler(deps.AdGuard))
	apiMux.HandleFunc("POST /api/adguard/bootstrap", bootstrapAdGuardHandler(deps.AdGuard, deps.Installer))
	apiMux.Handle("/api/adguard/ui/", deps.AdGuard.UIHandler())

	root := http.NewServeMux()
	root.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	// Deliberately registered on root, not apiMux - it must NOT sit behind
	// requireBearerToken(deps.Token, ...) below, since deps.Token is Core's
	// own admin token (also handed to the WebApp and the updater), far more
	// powerful than anything this endpoint needs to expose. blockpage's
	// nginx authenticates with its own, much narrower service token
	// instead (checked inside the handler itself via
	// AdGuard.VerifyBlockpageServiceToken) - found in review: blockpage
	// used to hold the full AdGuard admin credentials directly (reversibly
	// encoded, not even hashed) just to make this same check_host call
	// itself; now only Core ever holds those. Go's ServeMux (1.22+)
	// dispatches by longest/most-specific pattern match regardless of
	// registration order, so this exact-path registration on root wins
	// over the broader "/api/" one below - the same trick /api/health
	// above already relies on.
	root.HandleFunc("GET /api/blockpage/reason", blockpageReasonHandler(deps.AdGuard))
	root.Handle("/api/", requireBearerToken(deps.Token, apiMux))
	return root
}

func blockpageReasonHandler(manager *adguard.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || !manager.VerifyBlockpageServiceToken(token) {
			writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		host := r.URL.Query().Get("host")
		reason, err := manager.ReasonForHost(r.Context(), host)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, adguard.ErrInvalidBlockpageHost) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"reason": reason})
	}
}

func unboundNetworkCapabilitiesHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, manager.NetworkCapabilities(r.Context()))
	}
}

func controlPlaneStatusHandler(client *controlplane.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := client.Status(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func controlPlaneCheckHandler(client *controlplane.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := client.Check(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

func controlPlaneUpdateHandler(client *controlplane.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := client.Update(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

// updaterSelfUpdateInstallHandler triggers the RootGuard Updater's own
// image update. Unlike updateServiceHandler, it first refuses if the
// control-plane updater it's about to replace is itself mid core/webapp
// check or update - the container swap would otherwise abort that
// operation. This is a UX guard, not a correctness requirement: an
// interrupted core/webapp update already recovers cleanly on the
// updater's own next start (the same path already exercised by a real
// process kill).
func updaterSelfUpdateInstallHandler(manager *updater.Manager, controlPlane *controlplane.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if status, err := controlPlane.Status(r.Context()); err == nil && (status.State == "checking" || status.State == "updating") {
			writeError(w, http.StatusConflict, fmt.Errorf("control-plane updater is busy with a core/webapp operation, try again once it finishes"))
			return
		}
		status, err := manager.StartUpdate("updater")
		if err != nil {
			switch {
			case errors.Is(err, updater.ErrBusy):
				writeError(w, http.StatusConflict, err)
			case errors.Is(err, updater.ErrUnknownService):
				writeError(w, http.StatusBadRequest, err)
			default:
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusAccepted, status)
	}
}

func updateStatusHandler(manager *updater.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, manager.Status())
	}
}

func updateCheckHandler(manager *updater.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		status, err := manager.StartCheck()
		if err != nil {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusAccepted, status)
	}
}

func updateServiceHandler(manager *updater.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := manager.StartUpdate(r.PathValue("name"))
		if err != nil {
			switch {
			case errors.Is(err, updater.ErrBusy):
				writeError(w, http.StatusConflict, err)
			case errors.Is(err, updater.ErrUnknownService):
				writeError(w, http.StatusBadRequest, err)
			default:
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusAccepted, status)
	}
}

func backupStatusHandler(manager *updater.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		status, err := manager.BackupStatus()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func putBackupSettingsHandler(manager *updater.Manager) http.HandlerFunc {
	type request struct {
		RetentionPerService int `json:"retention_per_service"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := decodeJSON[request](w, r, 4<<10)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		status, err := manager.SetBackupRetention(body.RetentionPerService)
		if err != nil {
			switch {
			case errors.Is(err, updater.ErrBusy):
				writeError(w, http.StatusConflict, err)
			case errors.Is(err, updater.ErrInvalidBackupRetention):
				writeError(w, http.StatusBadRequest, err)
			default:
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func cleanupPreviewHandler(manager *updater.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		preview, err := manager.PreviewCleanup(r.Context())
		if err != nil {
			if errors.Is(err, updater.ErrBusy) {
				writeError(w, http.StatusConflict, err)
			} else {
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, preview)
	}
}

func runCleanupHandler(manager *updater.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := manager.RunManualCleanup(r.Context())
		if err != nil {
			if errors.Is(err, updater.ErrBusy) {
				writeError(w, http.StatusConflict, err)
			} else {
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func backupExportHandler(exporter *backupexport.Exporter, manager *updater.Manager) http.HandlerFunc {
	type request struct {
		Passphrase string `json:"passphrase"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := decodeJSON[request](w, r, 4<<10)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := backupexport.ValidatePassphrase(body.Passphrase); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.rootguard.backup+age")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="rootguard-backup-%s.tar.gz.age"`, time.Now().UTC().Format("2006-01-02")))
		w.Header().Set("Cache-Control", "no-store")
		writer := &lazyResponseWriter{ResponseWriter: w}
		err = manager.RunExclusive("Verschlüsseltes Vollbackup wird erstellt.", func() error {
			return exporter.Export(r.Context(), body.Passphrase, writer)
		})
		if err != nil && !writer.wrote {
			if errors.Is(err, backupexport.ErrInvalidPassphrase) {
				writeError(w, http.StatusBadRequest, err)
			} else if errors.Is(err, backupexport.ErrBusy) || errors.Is(err, updater.ErrBusy) {
				writeError(w, http.StatusConflict, err)
			} else {
				writeError(w, http.StatusInternalServerError, err)
			}
		}
	}
}

type lazyResponseWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *lazyResponseWriter) Write(data []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(data)
}

func getUnboundConfigurationHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configuration, err := manager.ActiveConfiguration(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, configuration)
	}
}

func installationStatusHandler(manager *installer.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, manager.Status())
	}
}

func installationPreflightHandler(manager *installer.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config, ok := decodeInstallationConfig(w, r)
		if !ok {
			return
		}
		report := manager.Preflight(r.Context(), config)
		writeJSON(w, http.StatusOK, report)
	}
}

func installationDeployHandler(manager *installer.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config, ok := decodeInstallationConfig(w, r)
		if !ok {
			return
		}
		status, err := manager.Start(r.Context(), config)
		if err != nil {
			switch {
			case errors.Is(err, installer.ErrInvalidConfig):
				writeError(w, http.StatusUnprocessableEntity, err)
			case errors.Is(err, installer.ErrDeploying):
				writeError(w, http.StatusConflict, err)
			default:
				writeError(w, http.StatusInternalServerError, err)
			}
			return
		}
		writeJSON(w, http.StatusAccepted, status)
	}
}

func decodeInstallationConfig(w http.ResponseWriter, r *http.Request) (installer.Config, bool) {
	defer r.Body.Close()
	config, err := decodeJSON[installer.Config](w, r, 8<<10)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return installer.Config{}, false
	}
	return config, true
}

func getAdGuardStatusHandler(manager *adguard.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := manager.Status(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func getAdGuardFilterReportHandler(manager *adguard.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := manager.FilterReport(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

func setAdGuardFilteringHandler(manager *adguard.Manager) http.HandlerFunc {
	type request struct {
		Enabled *bool `json:"enabled"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		input, err := decodeJSON[request](w, r, 1<<10)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		// Found via code review: {} used to decode to Enabled=false with no
		// way to tell it apart from an explicit {"enabled":false}, silently
		// disabling filtering. Same class of bug as the protection endpoint
		// below.
		if input.Enabled == nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("enabled is required"))
			return
		}
		status, err := manager.SetFiltering(r.Context(), *input.Enabled)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

// adGuardProtectionDurations are the only durations the UI actually offers
// (Off/10 minutes/1 hour) - found via code review: without this allowlist,
// a request with enabled=true and a positive duration_seconds wasn't
// rejected here at all, AdGuard just returned an error for it that surfaced
// as a confusing 502 for what was really a bad client request.
var adGuardProtectionDurations = map[int64]bool{0: true, 600: true, 3600: true}

func validateAdGuardProtectionRequest(enabled *bool, durationSeconds int64) error {
	if enabled == nil {
		return fmt.Errorf("enabled is required")
	}
	if !adGuardProtectionDurations[durationSeconds] {
		return fmt.Errorf("duration_seconds must be one of 0, 600, 3600")
	}
	if *enabled && durationSeconds != 0 {
		return fmt.Errorf("duration_seconds must be 0 when enabling protection")
	}
	return nil
}

func setAdGuardProtectionHandler(manager *adguard.Manager) http.HandlerFunc {
	type request struct {
		Enabled         *bool `json:"enabled"`
		DurationSeconds int64 `json:"duration_seconds"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		input, err := decodeJSON[request](w, r, 1<<10)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := validateAdGuardProtectionRequest(input.Enabled, input.DurationSeconds); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		status, err := manager.SetProtection(r.Context(), *input.Enabled, time.Duration(input.DurationSeconds)*time.Second)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func bootstrapAdGuardHandler(manager *adguard.Manager, installer *installer.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var blockPageIP string
		if config := installer.Status().Config; config != nil && config.BlockpageEnabled {
			blockPageIP = config.DNSBindAddress
		}
		status, err := manager.Bootstrap(r.Context(), blockPageIP)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func systemHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"os": runtime.GOOS, "arch": runtime.GOARCH,
	})
}

func dockerStatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, docker.CheckDockerStatus())
}

func stackStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := stack.CheckStackStatus()
	stack.CheckStackAttestations(r.Context(), &status)
	writeJSON(w, http.StatusOK, status)
}

type dashboardResponse struct {
	Docker dashboardDocker `json:"docker"`
	DNS    dashboardDNS    `json:"dns"`
}

type dashboardDocker struct {
	CPU              float64 `json:"cpu"`
	Memory           uint64  `json:"memory"`
	MetricsAvailable bool    `json:"metrics_available"`
	Containers       int     `json:"containers"`
	Status           string  `json:"status"`
	CollectedAt      int64   `json:"collected_at"`
}

type dashboardDNS struct {
	Status   string `json:"status"`
	Resolver string `json:"resolver"`
	DNSSEC   bool   `json:"dnssec"`
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	status := stack.CollectStatus()
	metrics := stack.CollectMetrics(r.Context())
	running := 0
	if status.AdGuard.Running {
		running++
	}
	if status.Unbound.Running {
		running++
	}

	dockerHealth := "down"
	if running == 2 {
		dockerHealth = "healthy"
	} else if running > 0 {
		dockerHealth = "degraded"
	}

	dnsHealth := "down"
	if status.Unbound.Running && status.AdGuard.Running {
		dnsHealth = "healthy"
	} else if status.Unbound.Running || status.AdGuard.Running {
		dnsHealth = "degraded"
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		Docker: dashboardDocker{
			CPU: metrics.CPUPercent, Memory: metrics.MemoryBytes,
			MetricsAvailable: metrics.Available, Containers: running, Status: dockerHealth,
			CollectedAt: metrics.CollectedAt,
		},
		DNS: dashboardDNS{Status: dnsHealth, Resolver: "Unbound", DNSSEC: status.Unbound.Running},
	})
}

type serviceResponse struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	Health       string   `json:"health"`
	Image        string   `json:"image,omitempty"`
	ImageID      string   `json:"imageId,omitempty"`
	StartedAt    string   `json:"startedAt,omitempty"`
	RestartCount int      `json:"restartCount"`
	Ports        []string `json:"ports,omitempty"`
	Version      string   `json:"version,omitempty"`
	Revision     string   `json:"revision,omitempty"`
	Created      string   `json:"created,omitempty"`
	Source       string   `json:"source,omitempty"`
	Immutable    bool     `json:"immutable"`
	Metadata     string   `json:"metadata"`
	Attestation  string   `json:"attestation"`
	AttestedAt   string   `json:"attestedAt,omitempty"`
}

func servicesHandler(w http.ResponseWriter, r *http.Request) {
	status := stack.CheckStackStatus()
	stack.CheckStackAttestations(r.Context(), &status)
	writeJSON(w, http.StatusOK, []serviceResponse{
		serviceRuntimeResponse("core", "RootGuard Core", "Privileged orchestration and configuration", status.Core),
		serviceRuntimeResponse("webapp", "RootGuard WebApp", "Authenticated operator interface", status.WebApp),
		serviceRuntimeResponse("updater", "RootGuard Updater", "Independent control-plane update helper", status.Updater),
		serviceRuntimeResponse("adguard", "AdGuard Home", "DNS filtering, blocklists and client policies", status.AdGuard),
		serviceRuntimeResponse("unbound", "Unbound DNS", "Recursive resolver with DNSSEC validation", status.Unbound),
	})
}

func serviceRuntimeResponse(name, displayName, description string, info stack.ContainerInfo) serviceResponse {
	return serviceResponse{
		Name: name, DisplayName: displayName, Description: description,
		Status: runningStatus(info.Running), Health: info.Health,
		Image: info.Image, ImageID: info.ImageID, StartedAt: info.StartedAt,
		RestartCount: info.RestartCount, Ports: info.Ports, Version: info.Version,
		Revision: info.Revision, Created: info.Created, Source: info.Source,
		Immutable: info.Immutable, Metadata: info.Metadata,
		Attestation: info.Attestation, AttestedAt: info.AttestedAt,
	}
}

func runningStatus(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}

func serviceActionHandler(w http.ResponseWriter, r *http.Request) {
	serviceName := r.PathValue("name")
	action := r.PathValue("action")
	if err := stack.ControlService(r.Context(), serviceName, action); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, stack.ErrUnknownService) || errors.Is(err, stack.ErrUnknownAction) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"service": serviceName,
		"action":  action,
		"status":  "ok",
	})
}

func serviceLogsHandler(w http.ResponseWriter, r *http.Request) {
	logs, err := stack.ReadServiceLogs(r.Context(), r.PathValue("name"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, stack.ErrUnknownService) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func unboundDiagnosticLoggingStatusHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, manager.DiagnosticLoggingStatus())
	}
}

func startUnboundDiagnosticLoggingHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := manager.StartDiagnosticLogging(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func stopUnboundDiagnosticLoggingHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := manager.StopDiagnosticLogging(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}

func getUnboundSettingsHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		settings, err := manager.Load()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func putUnboundSettingsHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, ok := decodeUnboundSettings(w, r)
		if !ok {
			return
		}
		if err := manager.Apply(r.Context(), settings); err != nil {
			if errors.Is(err, unbound.ErrInvalidSettings) || errors.Is(err, unbound.ErrInvalidCustomConfig) {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func previewUnboundSettingsHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, ok := decodeUnboundSettings(w, r)
		if !ok {
			return
		}
		preview, err := manager.Preview(settings)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrInvalidSettings) || errors.Is(err, unbound.ErrInvalidCustomConfig) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, preview)
	}
}

func unboundHistoryHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		history, err := manager.History()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, history)
	}
}

func restoreUnboundVersionHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := manager.Restore(r.Context(), r.PathValue("id"))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrVersionNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func getUnboundExportHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		bundle, err := manager.Export()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, bundle)
	}
}

func previewUnboundImportHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bundle, ok := decodeUnboundBundle(w, r)
		if !ok {
			return
		}
		preview, err := manager.PreviewBundle(r.Context(), bundle.Settings, bundle.CustomConfig)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrInvalidSettings) || errors.Is(err, unbound.ErrInvalidCustomConfig) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, preview)
	}
}

func applyUnboundImportHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bundle, ok := decodeUnboundBundle(w, r)
		if !ok {
			return
		}
		if err := manager.ApplyBundle(r.Context(), bundle.Settings, bundle.CustomConfig); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrInvalidSettings) || errors.Is(err, unbound.ErrInvalidCustomConfig) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, bundle.Settings)
	}
}

func decodeUnboundBundle(w http.ResponseWriter, r *http.Request) (unbound.ConfigBundle, bool) {
	defer r.Body.Close()
	bundle, err := decodeJSON[unbound.ConfigBundle](w, r, int64(unbound.MaxCustomConfigBytes)+64<<10)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return unbound.ConfigBundle{}, false
	}
	if bundle.SchemaVersion != unbound.BundleSchemaVersion {
		err := fmt.Errorf("%w: got schema version %d, this RootGuard release supports %d", unbound.ErrIncompatibleBundle, bundle.SchemaVersion, unbound.BundleSchemaVersion)
		writeError(w, http.StatusBadRequest, err)
		return unbound.ConfigBundle{}, false
	}
	return bundle, true
}

type importConfRequest struct {
	Content string `json:"content"`
}

func classifyUnboundImportConfHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		request, err := decodeJSON[importConfRequest](w, r, int64(unbound.MaxCustomConfigBytes)+1024)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		result, err := manager.ClassifyImport(request.Content)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrInvalidCustomConfig) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func unboundDiagnosticsHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, manager.Diagnose(r.Context()))
	}
}

func unboundPathDiagnosticsHandler(manager *unbound.Manager, adguardAddress string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, manager.DiagnosePath(r.Context(), adguardAddress))
	}
}

func unboundPresetsHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, unbound.Presets())
}

func unboundAdviceHandler(w http.ResponseWriter, r *http.Request) {
	settings, ok := decodeUnboundSettings(w, r)
	if !ok {
		return
	}
	advice, err := unbound.Advise(settings)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, unbound.ErrInvalidSettings) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, advice)
}

type forwardCheckRequest struct {
	Zones []unbound.ForwardZone `json:"zones"`
}

func unboundForwardCheckHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		request, err := decodeJSON[forwardCheckRequest](w, r, 64<<10)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		checks, err := manager.CheckForwardTargets(r.Context(), request.Zones)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrInvalidSettings) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, checks)
	}
}

type fritzBoxDiscoverRequest struct {
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func fritzBoxDiscoverHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	request, err := decodeJSON[fritzBoxDiscoverRequest](w, r, 4<<10)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	client, err := routerimport.NewFritzBoxClient(request.Address)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := client.DiscoverHosts(r.Context(), routerimport.Credentials{Username: request.Username, Password: request.Password})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, routerimport.ErrRouterDiscovery) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type reverseDNSDiscoverRequest struct {
	Networks []string `json:"networks"`
}

func reverseDNSDiscoverHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	request, err := decodeJSON[reverseDNSDiscoverRequest](w, r, 4<<10)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := routerimport.NewReverseDNSDiscoverer().Discover(r.Context(), request.Networks)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, routerimport.ErrReverseDNSDiscovery) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type customConfigRequest struct {
	Content string `json:"content"`
}

func getUnboundCustomHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		document, err := manager.LoadCustom()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, document)
	}
}

func previewUnboundCustomHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := decodeCustomConfig(w, r)
		if !ok {
			return
		}
		preview, err := manager.PreviewCustom(r.Context(), request.Content)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrInvalidCustomConfig) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, preview)
	}
}

func putUnboundCustomHandler(manager *unbound.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request, ok := decodeCustomConfig(w, r)
		if !ok {
			return
		}
		document, err := manager.ApplyCustom(r.Context(), request.Content)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, unbound.ErrInvalidCustomConfig) {
				status = http.StatusBadRequest
			}
			writeError(w, status, err)
			return
		}
		writeJSON(w, http.StatusOK, document)
	}
}

func unboundDirectivesHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, unbound.DirectiveReferences())
}

func decodeCustomConfig(w http.ResponseWriter, r *http.Request) (customConfigRequest, bool) {
	defer r.Body.Close()
	request, err := decodeJSON[customConfigRequest](w, r, unbound.MaxCustomConfigBytes+1024)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return customConfigRequest{}, false
	}
	return request, true
}

func decodeUnboundSettings(w http.ResponseWriter, r *http.Request) (unbound.Settings, bool) {
	defer r.Body.Close()
	settings, err := decodeJSON[unbound.Settings](w, r, 64<<10)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return unbound.Settings{}, false
	}
	return settings, true
}

func requireBearerToken(expected string, next http.Handler) http.Handler {
	expectedHash := sha256.Sum256([]byte(expected))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		providedHash := sha256.Sum256([]byte(provided))
		if provided == "" || subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
			writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decodeJSON decodes exactly one JSON value of type T from r's body,
// rejecting unknown fields and any trailing data after that value - a bare
// Decode() call silently ignores everything past the first JSON value, so
// a body like {"enabled":true}{"anything"} would otherwise decode
// successfully with the second part just discarded. Replaces what used to
// be an identical decoder/DisallowUnknownFields/Decode block repeated at
// every handler in this file, none of which had the trailing-data check.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request, maxBytes int64) (T, error) {
	var value T
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	if decoder.More() {
		return value, errors.New("unexpected trailing data after JSON body")
	}
	return value, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
