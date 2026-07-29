package adguard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestBootstrapInstallsAndConfiguresUnbound(t *testing.T) {
	var mu sync.Mutex
	configured := false
	configureCalls := 0
	dnsConfigCalls := 0
	var credentials Credentials

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch r.URL.Path {
		case "/control/install/get_addresses":
			_ = json.NewEncoder(w).Encode(map[string]any{"interfaces": map[string]any{}})
		case "/control/install/check_config":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"web": map[string]any{"status": "", "can_autofix": false},
				"dns": map[string]any{"status": "", "can_autofix": false},
			})
		case "/control/install/configure":
			if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
				t.Fatal(err)
			}
			configured = true
			configureCalls++
		case "/control/status":
			user, password, ok := r.BasicAuth()
			if !configured || !ok || user != credentials.Username || password != credentials.Password {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"dns_addresses": []string{"0.0.0.0"}})
		case "/control/test_upstream_dns":
			_ = json.NewEncoder(w).Encode(map[string]string{"rootguard-unbound:5335": "OK"})
		case "/control/dns_config":
			dnsConfigCalls++
			w.WriteHeader(http.StatusOK)
		case "/control/dns_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"upstream_dns": []string{"rootguard-unbound:5335"},
				"fallback_dns": []string{},
			})
		case "/control/stats":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"num_dns_queries":       125,
				"num_blocked_filtering": 25,
				"avg_processing_time":   0.012,
			})
		default:
			http.NotFound(w, r)
		}
	})

	manager := newTestManager(t, handler)
	status, err := manager.Bootstrap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || !status.Healthy || !status.UpstreamReady {
		t.Fatalf("unexpected status: %+v", status)
	}
	if !status.StatsAvailable || status.Queries != 125 || status.Blocked != 25 {
		t.Fatalf("unexpected statistics: %+v", status)
	}

	status, err = manager.Bootstrap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.UpstreamReady {
		t.Fatalf("unexpected second status: %+v", status)
	}
	if configureCalls != 1 {
		t.Fatalf("expected one installer call, got %d", configureCalls)
	}
	if dnsConfigCalls != 2 {
		t.Fatalf("expected upstream reconciliation on each bootstrap, got %d", dnsConfigCalls)
	}
}

func TestBootstrapRejectsBrokenUpstream(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/control/status":
			_ = json.NewEncoder(w).Encode(map[string]any{})
		case "/control/test_upstream_dns":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"rootguard-unbound:5335": "connection refused",
			})
		case "/control/dns_config":
			t.Fatal("dns_config must not be called after failed upstream test")
		default:
			http.NotFound(w, r)
		}
	})

	dir := t.TempDir()
	if err := writeCredentials(dir+"/credentials.json", Credentials{Username: "rootguard", Password: "secret"}); err != nil {
		t.Fatal(err)
	}
	manager := newTestManagerWithDir(handler, dir)
	if _, err := manager.Bootstrap(context.Background()); err == nil {
		t.Fatal("expected upstream validation failure")
	}
}

func TestInstallerReadinessRetriesTemporaryFailures(t *testing.T) {
	attempts := 0
	manager := NewManager("http://adguard-installer", "http://adguard", t.TempDir(), "rootguard-unbound:5335")
	manager.retryDelay = time.Millisecond
	manager.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		attempts++
		if attempts < 3 {
			http.Error(recorder, "starting", http.StatusServiceUnavailable)
		} else {
			_ = json.NewEncoder(recorder).Encode(map[string]any{"interfaces": map[string]any{}})
		}
		return recorder.Result(), nil
	})

	if err := manager.waitUntilInstallerReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("expected three readiness attempts, got %d", attempts)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func newTestManager(t *testing.T, handler http.Handler) *Manager {
	t.Helper()
	return newTestManagerWithDir(handler, t.TempDir())
}

func newTestManagerWithDir(handler http.Handler, dir string) *Manager {
	manager := NewManager("http://adguard-installer", "http://adguard", dir, "rootguard-unbound:5335")
	manager.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	})
	return manager
}
