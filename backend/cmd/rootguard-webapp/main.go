// =====================================================
// File: backend/cmd/rootguard-webapp/main.go
// Project: RootGuard WebApp
// Purpose: Application entrypoint
// Notes:
// - Injects build metadata via -ldflags
// - Registers graceful shutdown
// - Syncs version info with httpapi package
// - Starts Docker Stats Collector
// =====================================================

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
	"github.com/foxly-it/rootguard-webapp/backend/internal/httpapi"
)

// =====================================================
// Build Metadata (Injected at build time)
//
// Example:
//
// go build -ldflags "-X main.version=v0.1.0 -X main.commit=abc123"
// =====================================================

var version = "dev"
var commit = "unknown"

// =====================================================
// init()
// Sync build metadata into httpapi package
// =====================================================

func init() {
	httpapi.Version = version
	httpapi.Commit = commit
}

// =====================================================
// main()
// =====================================================

func main() {
	port := getEnv("PORT", "8080")
	coreToken := os.Getenv("ROOTGUARD_API_TOKEN")
	adminPassword := os.Getenv("ROOTGUARD_ADMIN_PASSWORD")
	if coreToken == "" {
		log.Fatal("ROOTGUARD_API_TOKEN must be set")
	}
	if adminPassword == "" {
		log.Fatal("ROOTGUARD_ADMIN_PASSWORD must be set")
	}

	core := coreclient.New(
		getEnv("ROOTGUARD_CORE_URL", "http://rootguard-core:8081"),
		coreToken,
	)
	sessionAuth := httpapi.NewSessionAuth(
		getEnv("ROOTGUARD_ADMIN_USER", "admin"),
		adminPassword,
		os.Getenv("ROOTGUARD_RECOVERY_TOKEN"),
		12*time.Hour,
		getEnv("ROOTGUARD_SESSION_FILE", "/var/lib/rootguard-sessions/sessions.json"),
	)
	router := httpapi.RequireSameOriginWrites(sessionAuth.Handler(httpapi.NewRouter(core)))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// -------------------------------------------------
	// Start HTTP server
	// -------------------------------------------------

	go func() {

		log.Printf("RootGuard WebApp starting (version=%s, commit=%s)", version, commit)
		log.Printf("Listening on :%s", port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}

	}()

	// -------------------------------------------------
	// Graceful Shutdown Handling
	// -------------------------------------------------

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

// =====================================================
// getEnv()
// Helper to read environment variables with fallback
// =====================================================

func getEnv(key, fallback string) string {

	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return fallback
}
