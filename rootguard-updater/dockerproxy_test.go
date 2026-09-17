package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCheckDockerProxyReachableUnset is the one deliberate behavioral
// difference from checkAttestationProxyReachable's own unset case - see
// dockerproxy.go's doc comment for why unset must not be an error here.
func TestCheckDockerProxyReachableUnset(t *testing.T) {
	t.Setenv("ROOTGUARD_DOCKER_PROXY_URL", "")
	if err := checkDockerProxyReachable(); err != nil {
		t.Fatalf("expected no error when ROOTGUARD_DOCKER_PROXY_URL is unset, got: %v", err)
	}
}

func TestCheckDockerProxyReachableConfiguredButUnreachable(t *testing.T) {
	t.Setenv("ROOTGUARD_DOCKER_PROXY_URL", "http://docker-proxy:2375")
	original := dialDockerProxy
	dialDockerProxy = func(network, addr string) (net.Conn, error) {
		return nil, errors.New("connection refused")
	}
	defer func() { dialDockerProxy = original }()

	err := checkDockerProxyReachable()
	if err == nil {
		t.Fatal("expected an error when the configured proxy is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") || !strings.Contains(err.Error(), "docker-proxy:2375") {
		t.Fatalf("expected the unreachable-specific message naming the configured URL, got: %v", err)
	}
}

func TestCheckDockerProxyReachableConfiguredAndUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("ROOTGUARD_DOCKER_PROXY_URL", server.URL)
	if err := checkDockerProxyReachable(); err != nil {
		t.Fatalf("expected no error against a real, healthy /healthz endpoint: %v", err)
	}
}

func TestCheckDockerProxyReachableConfiguredWithoutScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bareHostPort := strings.TrimPrefix(server.URL, "http://")
	t.Setenv("ROOTGUARD_DOCKER_PROXY_URL", bareHostPort)
	if err := checkDockerProxyReachable(); err != nil {
		t.Fatalf("expected a schemeless host:port value to default to http://, got: %v", err)
	}
}

func TestCheckDockerProxyReachableConfiguredButUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	t.Setenv("ROOTGUARD_DOCKER_PROXY_URL", server.URL)
	err := checkDockerProxyReachable()
	if err == nil {
		t.Fatal("expected an error against a non-200 /healthz response")
	}
	if !strings.Contains(err.Error(), "unhealthy") {
		t.Fatalf("expected the unhealthy-specific message, got: %v", err)
	}
}
