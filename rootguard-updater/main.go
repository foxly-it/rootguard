// rootguard-updater is the control-plane updater: a small, standalone
// binary that updates RootGuard Core and the WebApp atomically, since
// neither can safely update itself while it's the thing serving the
// request that triggered the update. Split across a few files by concern
// rather than kept in one - found in review this session (a code-
// compression suggestion from a security audit): manager.go holds the
// state machine and its persisted status; image.go the
// digest/version-resolution helpers; http.go the HTTP-layer glue; docker.go
// the raw docker-CLI/filesystem primitives; attestation.go (pre-existing)
// the cosign check. All still package main - this binary has no other
// consumer, so real sub-packages would add import-cycle bookkeeping for no
// reuse benefit; multiple files in one package gets the same readability
// win this suggestion was actually about.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	token := os.Getenv("ROOTGUARD_UPDATER_TOKEN")
	if token == "" {
		log.Fatal("ROOTGUARD_UPDATER_TOKEN must be set")
	}
	requireSecretStrength("ROOTGUARD_UPDATER_TOKEN", token, minTokenLength)
	if err := prepareSessionVolume(envOrDefault("ROOTGUARD_SESSION_DIR", "/var/lib/rootguard-sessions")); err != nil {
		log.Fatalf("prepare WebApp session volume: %v", err)
	}
	manager := newManager(
		envOrDefault("ROOTGUARD_UPDATER_DATA_DIR", "/var/lib/rootguard/control-plane-updater"),
		envOrDefault("ROOTGUARD_COMPOSE_FILE", "/opt/rootguard/compose.yaml"),
		envOrDefault("ROOTGUARD_COMPOSE_PROJECT", "rootguard"),
		[]serviceSpec{
			{
				Name: "core", DisplayName: "RootGuard Core", Container: "rootguard-core",
				TargetImage: envOrDefault("ROOTGUARD_CORE_UPDATE_IMAGE", "ghcr.io/foxly-it/rootguard-core:latest"),
				HealthURL:   "http://core:8081/api/health",
			},
			{
				Name: "webapp", DisplayName: "RootGuard WebApp", Container: "rootguard-webapp",
				TargetImage: envOrDefault("ROOTGUARD_WEBAPP_UPDATE_IMAGE", "ghcr.io/foxly-it/rootguard-webapp:latest"),
				HealthURL:   "http://webapp:8080/health",
			},
		},
		runDocker,
	)
	manager.skipPull = strings.EqualFold(os.Getenv("ROOTGUARD_UPDATER_SKIP_PULL"), "true")
	// Same test-only escape hatch as ROOTGUARD_UPDATER_SKIP_PULL right
	// above, for the same reason: integration/run.sh's E2E fixtures are
	// locally built images with no real cosign attestation to check -
	// unlike SKIP_PULL, this one disables a real security control, so it's
	// worth spelling out plainly: any deployment that sets this in
	// production loses the activation gate this update added entirely.
	// Nothing in this codebase sets it outside integration/compose.e2e.yaml.
	if strings.EqualFold(os.Getenv("ROOTGUARD_UPDATER_SKIP_ATTESTATION"), "true") {
		manager.attestationVerifier = func(context.Context, string, string) error { return nil }
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/control-plane/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, manager.Status())
	})
	mux.HandleFunc("POST /api/control-plane/check", func(w http.ResponseWriter, r *http.Request) {
		overrides, err := decodeTargetOverrides(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		next, err := manager.StartCheck(overrides)
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
	})
	mux.HandleFunc("POST /api/control-plane/update", func(w http.ResponseWriter, r *http.Request) {
		overrides, err := decodeTargetOverrides(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		next, err := manager.StartUpdate(overrides)
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
	})

	server := &http.Server{
		Addr:              ":8082",
		Handler:           requireBearer(token, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Print("RootGuard control-plane updater listening on :8082")
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func prepareSessionVolume(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	if err := os.Chown(path, 65532, 65532); err != nil {
		return err
	}
	return os.Chmod(path, 0700)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

const (
	minTokenLength = 32
	// placeholderPrefix matches every secret value in .env.release.example
	// ("replace-with-a-long-random-token", ...) - compose.release.yaml
	// derives ROOTGUARD_UPDATER_TOKEN from that same ROOTGUARD_API_TOKEN
	// value, so an unedited placeholder ends up here too.
	placeholderPrefix = "replace-with-"
)

// requireSecretStrength exits the process if value is too short or is
// still an unedited .env.release.example placeholder.
func requireSecretStrength(name, value string, minLength int) {
	if strings.HasPrefix(strings.ToLower(value), placeholderPrefix) {
		log.Fatalf("%s is still set to its .env.release.example placeholder value - set a real secret", name)
	}
	if len(value) < minLength {
		log.Fatalf("%s must be at least %d characters", name, minLength)
	}
}
