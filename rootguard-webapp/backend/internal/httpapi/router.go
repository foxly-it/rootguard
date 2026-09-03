// =====================================================
// File: backend/internal/httpapi/router.go
// Project: RootGuard WebApp
// Purpose: Central HTTP router configuration
//
// Responsibilities:
//
// - register API routes
// - expose health endpoints
// - expose dashboard endpoints
// - expose system overview endpoint
// - expose service detection endpoint
// - expose service control endpoint
// - serve frontend SPA
//
// Uses:
//
// - Go standard library net/http ServeMux
//
// =====================================================

package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/foxly-it/rootguard-webapp/backend/internal/api"
	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// =====================================================
// NewRouter
//
// Registers:
//
// /health
// /api/health
// /api/version
// /api/dashboard
// /api/system
// /api/services
// /api/service/{name}/{action}
//
// Also serves the frontend SPA.
// =====================================================

func NewRouter(core *coreclient.Client, sessionAuth *SessionAuth) http.Handler {

	mux := http.NewServeMux()
	dest := sessionAuth.guardDestructive

	// ==================================================
	// Health Endpoints
	// ==================================================

	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/api/health", HealthHandler)

	// ==================================================
	// Version Endpoint
	// ==================================================

	mux.HandleFunc("/api/version", VersionHandler)

	// ==================================================
	// Dashboard Endpoint
	// ==================================================

	mux.HandleFunc("/api/dashboard", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		api.HandleDashboard(w, r, core)

	})

	// ==================================================
	// System Overview Endpoint
	//
	// Provides aggregated Docker metrics:
	//
	// - docker version
	// - container count
	// - running containers
	// - total cpu
	// - total memory
	//
	// ==================================================

	mux.HandleFunc("/api/system", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		api.HandleSystem(w, r, core)

	})

	// ==================================================
	// Service Detection Endpoint
	//
	// GET /api/services
	//
	// Returns detected DNS related services
	// detected by the RootGuard service engine.
	//
	// Example response:
	//
	// [
	//   {
	//     "name": "AdGuard Home",
	//     "type": "binary",
	//     "status": "running"
	//   },
	//   {
	//     "name": "Unbound",
	//     "type": "binary",
	//     "status": "running"
	//   }
	// ]
	//
	// ==================================================

	mux.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		api.HandleServices(w, r, core)

	})

	mux.HandleFunc("GET /api/services/{name}/logs", func(w http.ResponseWriter, r *http.Request) {
		api.HandleServiceLogs(w, r, core)
	})

	// ==================================================
	// Service Control Endpoint
	//
	// Example:
	// POST /api/service/adguard/start
	// POST /api/service/unbound/restart
	//
	// ==================================================

	serviceAction := dest(auditServiceAction, func(w http.ResponseWriter, r *http.Request) {
		api.HandleServiceAction(w, r, core)
	})
	mux.HandleFunc("/api/service/", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		serviceAction(w, r)

	})

	mux.HandleFunc("GET /api/installation", func(w http.ResponseWriter, r *http.Request) {
		api.HandleInstallationStatus(w, r, core)
	})

	mux.HandleFunc("POST /api/installation/preflight", func(w http.ResponseWriter, r *http.Request) {
		api.HandleInstallationPreflight(w, r, core)
	})

	mux.HandleFunc("POST /api/installation/deploy", dest(auditInstallationDeploy, func(w http.ResponseWriter, r *http.Request) {
		api.HandleInstallationDeploy(w, r, core)
	}))

	mux.HandleFunc("GET /api/updates", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUpdateStatus(w, r, core)
	})

	mux.HandleFunc("POST /api/updates/check", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUpdateCheck(w, r, core)
	})

	mux.HandleFunc("POST /api/updates/{name}", dest(auditServiceUpdateStarted, func(w http.ResponseWriter, r *http.Request) {
		api.HandleUpdateService(w, r, core)
	}))
	mux.HandleFunc("GET /api/backups", func(w http.ResponseWriter, r *http.Request) {
		api.HandleBackupStatus(w, r, core)
	})
	mux.HandleFunc("PUT /api/backups/settings", dest(auditBackupSettingsChanged, func(w http.ResponseWriter, r *http.Request) {
		api.HandlePutBackupSettings(w, r, core)
	}))
	mux.HandleFunc("GET /api/cleanup/preview", func(w http.ResponseWriter, r *http.Request) {
		api.HandleCleanupPreview(w, r, core)
	})
	mux.HandleFunc("POST /api/cleanup", dest(auditCleanupRun, func(w http.ResponseWriter, r *http.Request) {
		api.HandleRunCleanup(w, r, core)
	}))
	mux.HandleFunc("POST /api/backups/export", dest(auditBackupExport, func(w http.ResponseWriter, r *http.Request) {
		api.HandleBackupExport(w, r, core)
	}))
	mux.HandleFunc("POST /api/backups/restore/preview", func(w http.ResponseWriter, r *http.Request) {
		api.HandleBackupRestore(w, r, core, true)
	})
	mux.HandleFunc("POST /api/backups/restore", dest(auditBackupRestore, func(w http.ResponseWriter, r *http.Request) {
		api.HandleBackupRestore(w, r, core, false)
	}))

	mux.HandleFunc("GET /api/control-plane-updates", func(w http.ResponseWriter, r *http.Request) {
		api.HandleControlPlaneUpdateStatus(w, r, core)
	})

	mux.HandleFunc("POST /api/control-plane-updates/check", func(w http.ResponseWriter, r *http.Request) {
		api.HandleControlPlaneUpdateCheck(w, r, core)
	})

	mux.HandleFunc("POST /api/control-plane-updates/install", dest(auditControlPlaneUpdateInstall, func(w http.ResponseWriter, r *http.Request) {
		api.HandleControlPlaneUpdateInstall(w, r, core)
	}))

	mux.HandleFunc("GET /api/updater-updates", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUpdaterSelfUpdateStatus(w, r, core)
	})

	mux.HandleFunc("POST /api/updater-updates/check", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUpdaterSelfUpdateCheck(w, r, core)
	})

	mux.HandleFunc("POST /api/updater-updates/install", dest(auditUpdaterSelfUpdateInstall, func(w http.ResponseWriter, r *http.Request) {
		api.HandleUpdaterSelfUpdateInstall(w, r, core)
	}))

	mux.HandleFunc("GET /api/attestation-proxy-updates", func(w http.ResponseWriter, r *http.Request) {
		api.HandleAttestationProxySelfUpdateStatus(w, r, core)
	})

	mux.HandleFunc("POST /api/attestation-proxy-updates/check", func(w http.ResponseWriter, r *http.Request) {
		api.HandleAttestationProxySelfUpdateCheck(w, r, core)
	})

	mux.HandleFunc("POST /api/attestation-proxy-updates/install", dest(auditAttestationProxySelfUpdateInstall, func(w http.ResponseWriter, r *http.Request) {
		api.HandleAttestationProxySelfUpdateInstall(w, r, core)
	}))

	putUnboundSettings := dest(auditUnboundSettingsApplied, func(w http.ResponseWriter, r *http.Request) {
		api.HandlePutUnboundSettings(w, r, core)
	})
	mux.HandleFunc("/api/unbound/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			api.HandleGetUnboundSettings(w, r, core)
		case http.MethodPut:
			putUnboundSettings(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("GET /api/unbound/config", func(w http.ResponseWriter, r *http.Request) {
		api.HandleGetUnboundConfiguration(w, r, core)
	})

	mux.HandleFunc("POST /api/unbound/preview", func(w http.ResponseWriter, r *http.Request) {
		api.HandlePreviewUnboundSettings(w, r, core)
	})

	mux.HandleFunc("GET /api/unbound/history", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundHistory(w, r, core)
	})

	mux.HandleFunc("POST /api/unbound/history/{id}/restore", dest(auditUnboundSettingsRestored, func(w http.ResponseWriter, r *http.Request) {
		api.HandleRestoreUnboundVersion(w, r, core)
	}))

	mux.HandleFunc("GET /api/unbound/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundDiagnostics(w, r, core)
	})
	mux.HandleFunc("GET /api/unbound/path-diagnostics", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundPathDiagnostics(w, r, core)
	})
	mux.HandleFunc("GET /api/unbound/diagnostic-logging", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundDiagnosticLoggingStatus(w, r, core)
	})
	mux.HandleFunc("POST /api/unbound/diagnostic-logging", dest(auditUnboundDiagnosticLoggingStarted, func(w http.ResponseWriter, r *http.Request) {
		api.HandleStartUnboundDiagnosticLogging(w, r, core)
	}))
	mux.HandleFunc("DELETE /api/unbound/diagnostic-logging", dest(auditUnboundDiagnosticLoggingStopped, func(w http.ResponseWriter, r *http.Request) {
		api.HandleStopUnboundDiagnosticLogging(w, r, core)
	}))

	mux.HandleFunc("GET /api/unbound/presets", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundPresets(w, r, core)
	})

	mux.HandleFunc("POST /api/unbound/advice", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundAdvice(w, r, core)
	})

	mux.HandleFunc("POST /api/unbound/forward-check", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundForwardCheck(w, r, core)
	})
	mux.HandleFunc("GET /api/unbound/network-capabilities", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundNetworkCapabilities(w, r, core)
	})
	mux.HandleFunc("POST /api/router-import/fritzbox/discover", func(w http.ResponseWriter, r *http.Request) {
		api.HandleFritzBoxDiscover(w, r, core)
	})
	mux.HandleFunc("POST /api/router-import/reverse-dns/discover", func(w http.ResponseWriter, r *http.Request) {
		api.HandleReverseDNSDiscover(w, r, core)
	})

	mux.HandleFunc("GET /api/unbound/custom", func(w http.ResponseWriter, r *http.Request) {
		api.HandleGetUnboundCustom(w, r, core)
	})

	mux.HandleFunc("POST /api/unbound/custom/preview", func(w http.ResponseWriter, r *http.Request) {
		api.HandlePreviewUnboundCustom(w, r, core)
	})

	mux.HandleFunc("PUT /api/unbound/custom", dest(auditUnboundCustomApplied, func(w http.ResponseWriter, r *http.Request) {
		api.HandlePutUnboundCustom(w, r, core)
	}))

	mux.HandleFunc("GET /api/unbound/directives", func(w http.ResponseWriter, r *http.Request) {
		api.HandleUnboundDirectives(w, r, core)
	})

	mux.HandleFunc("GET /api/unbound/export", func(w http.ResponseWriter, r *http.Request) {
		api.HandleGetUnboundExport(w, r, core)
	})

	mux.HandleFunc("POST /api/unbound/import/preview", func(w http.ResponseWriter, r *http.Request) {
		api.HandlePreviewUnboundImport(w, r, core)
	})

	mux.HandleFunc("POST /api/unbound/import", dest(auditUnboundImportApplied, func(w http.ResponseWriter, r *http.Request) {
		api.HandleApplyUnboundImport(w, r, core)
	}))

	mux.HandleFunc("POST /api/unbound/import-conf", func(w http.ResponseWriter, r *http.Request) {
		api.HandleClassifyUnboundImportConf(w, r, core)
	})

	mux.HandleFunc("/api/adguard/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		api.HandleGetAdGuardStatus(w, r, core)
	})

	bootstrapAdGuard := dest(auditAdGuardBootstrap, func(w http.ResponseWriter, r *http.Request) {
		api.HandleBootstrapAdGuard(w, r, core)
	})
	mux.HandleFunc("/api/adguard/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		bootstrapAdGuard(w, r)
	})

	mux.HandleFunc("/api/adguard/filter-report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		api.HandleGetAdGuardFilterReport(w, r, core)
	})

	setAdGuardFiltering := dest(auditAdGuardFilteringToggled, func(w http.ResponseWriter, r *http.Request) {
		api.HandleSetAdGuardFiltering(w, r, core)
	})
	mux.HandleFunc("/api/adguard/filtering", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		setAdGuardFiltering(w, r)
	})

	setAdGuardProtection := dest(auditAdGuardProtectionToggled, func(w http.ResponseWriter, r *http.Request) {
		api.HandleSetAdGuardProtection(w, r, core)
	})
	mux.HandleFunc("/api/adguard/protection", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		setAdGuardProtection(w, r)
	})

	mux.Handle("/adguard-ui/", core.AdGuardUIHandler())

	// ==================================================
	// Static Frontend (SPA)
	// ==================================================

	fileServer := http.FileServer(http.Dir("./web"))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// If request is API → ignore
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Root → index.html
		if r.URL.Path == "/" {

			http.ServeFile(
				w,
				r,
				filepath.Join("web", "index.html"),
			)

			return
		}

		assetPath := filepath.Join("web", filepath.Clean(r.URL.Path))
		if _, err := os.Stat(assetPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// BrowserRouter fallback for client-side routes.
		http.ServeFile(w, r, filepath.Join("web", "index.html"))

	})

	// Same-origin write protection is applied once, by the caller, around
	// this router together with SessionAuth.Handler - see main.go. Wrapping
	// it here too would be inert: that outer wrap already rejects any
	// cross-origin write before it reaches this handler at all.
	return mux
}
