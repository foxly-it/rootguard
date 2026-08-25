package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
	"github.com/foxly-it/rootguard-webapp/backend/internal/httpapi"
)

// healthcheckFlag lets compose.yaml's HEALTHCHECK exec this same binary
// instead of curl/wget - the distroless runtime image has neither, or any
// shell at all, so a "CMD wget ..." style check (which works for Core and
// Updater's docker:29-cli base) can never run here.
const healthcheckFlag = "-healthcheck"

// version and commit are injected at build time, e.g.
// go build -ldflags "-X main.version=v0.1.0 -X main.commit=abc123"
var version = "dev"
var commit = "unknown"

func init() {
	httpapi.Version = version
	httpapi.Commit = commit
}

func main() {
	port := getEnv("PORT", "8080")
	if len(os.Args) > 1 && os.Args[1] == healthcheckFlag {
		runHealthcheck(port)
	}
	coreToken := os.Getenv("ROOTGUARD_API_TOKEN")
	adminPassword := os.Getenv("ROOTGUARD_ADMIN_PASSWORD")
	if coreToken == "" {
		log.Fatal("ROOTGUARD_API_TOKEN must be set")
	}
	if adminPassword == "" {
		log.Fatal("ROOTGUARD_ADMIN_PASSWORD must be set")
	}
	requireSecretStrength("ROOTGUARD_API_TOKEN", coreToken, minTokenLength)
	requireSecretStrength("ROOTGUARD_ADMIN_PASSWORD", adminPassword, minPasswordLength)
	recoveryToken := os.Getenv("ROOTGUARD_RECOVERY_TOKEN")
	// Empty deliberately skips the check: it's how recovery is turned off
	// (see SessionAuth.recoveryEnabled), not a weak secret.
	if recoveryToken != "" {
		requireSecretStrength("ROOTGUARD_RECOVERY_TOKEN", recoveryToken, minTokenLength)
	}

	core := coreclient.New(
		getEnv("ROOTGUARD_CORE_URL", "http://rootguard-core:8081"),
		coreToken,
	)
	sessionAuth := httpapi.NewSessionAuth(
		getEnv("ROOTGUARD_ADMIN_USER", "admin"),
		adminPassword,
		recoveryToken,
		12*time.Hour,
		getEnv("ROOTGUARD_SESSION_FILE", "/var/lib/rootguard-sessions/sessions.json"),
	)
	router := httpapi.RequireSameOriginWrites(sessionAuth.Handler(httpapi.NewRouter(core, sessionAuth)))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Minute,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("RootGuard WebApp starting (version=%s, commit=%s)", version, commit)
		log.Printf("Listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	log.Println("Server stopped cleanly")
}

// runHealthcheck is a standalone mode invoked by compose.yaml's
// HEALTHCHECK: makes a real HTTP request against the running server's own
// /health endpoint (not just a process-liveness check) and exits 0/1
// accordingly, then terminates immediately - it never falls through to
// starting a second server.
func runHealthcheck(port string) {
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		os.Exit(1)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	os.Exit(0)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

const (
	minPasswordLength = 12
	minTokenLength    = 32
	// placeholderPrefix matches every secret value in .env.release.example
	// ("replace-with-a-long-random-token", "replace-with-a-strong-password",
	// ...) - all of them happen to be long enough to pass the length checks
	// above on their own, so an operator who copies the example file
	// without editing it would otherwise start up "successfully" with a
	// publicly known secret.
	placeholderPrefix = "replace-with-"
)

// requireSecretStrength exits the process if value is too short or is
// still an unedited .env.release.example placeholder - called for every
// secret read from the environment at startup, so a weak admin password or
// API token is caught immediately instead of only being enforced later
// (e.g. the 12-character minimum the account-settings endpoint already
// applies to a *changed* password, but never to the one it started with).
func requireSecretStrength(name, value string, minLength int) {
	if strings.HasPrefix(strings.ToLower(value), placeholderPrefix) {
		log.Fatalf("%s is still set to its .env.release.example placeholder value - set a real secret", name)
	}
	if len(value) < minLength {
		log.Fatalf("%s must be at least %d characters", name, minLength)
	}
}
