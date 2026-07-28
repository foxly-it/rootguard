package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

var version = "dev"

func main() {
	port := envOrDefault("PORT", "8080")
	failHealth := os.Getenv("FAIL_HEALTH") == "true"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if failHealth && (r.URL.Path == "/health" || r.URL.Path == "/api/health") {
			http.Error(w, "unhealthy candidate", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": version,
		})
	})
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
