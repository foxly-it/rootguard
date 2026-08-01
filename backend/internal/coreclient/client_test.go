package coreclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUnboundSettingsPreserveResourceProfile(t *testing.T) {
	var received UnboundSettings
	client := New("http://rootguard-core.test", "test-token")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response := UnboundSettings{ResourceProfile: "medium", PrefetchKey: true, AggressiveNSEC: true, EDNSBufferSize: 1232, LogVerbosity: 1, ServeExpiredTTL: 86400, ServeExpiredClientTimeout: 1800}
		switch r.Method {
		case http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatal(err)
			}
			response = received
		}
		data, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
			Header:     make(http.Header),
		}, nil
	})

	active, err := client.UnboundSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if active.ResourceProfile != "medium" {
		t.Fatalf("expected medium resource profile, got %q", active.ResourceProfile)
	}
	if !active.PrefetchKey || !active.AggressiveNSEC || active.EDNSBufferSize != 1232 || active.LogVerbosity != 1 || active.ServeExpiredTTL != 86400 || active.ServeExpiredClientTimeout != 1800 {
		t.Fatalf("serve-expired controls were lost while decoding: %#v", active)
	}

	active.ResourceProfile = "large"
	active.ServeExpiredTTL = 172800
	active.ServeExpiredClientTimeout = 1200
	active.PrefetchKey = false
	active.AggressiveNSEC = false
	active.EDNSBufferSize = 1400
	active.LogVerbosity = 0
	updated, err := client.UpdateUnboundSettings(context.Background(), active)
	if err != nil {
		t.Fatal(err)
	}
	if received.ResourceProfile != "large" || updated.ResourceProfile != "large" {
		t.Fatalf("resource profile was lost in proxy roundtrip: received=%q updated=%q", received.ResourceProfile, updated.ResourceProfile)
	}
	if received.PrefetchKey || received.AggressiveNSEC ||
		updated.PrefetchKey || updated.AggressiveNSEC ||
		received.EDNSBufferSize != 1400 || updated.EDNSBufferSize != 1400 ||
		received.LogVerbosity != 0 || updated.LogVerbosity != 0 ||
		received.ServeExpiredTTL != 172800 || received.ServeExpiredClientTimeout != 1200 ||
		updated.ServeExpiredTTL != 172800 || updated.ServeExpiredClientTimeout != 1200 {
		t.Fatalf("serve-expired controls were lost in proxy roundtrip: received=%#v updated=%#v", received, updated)
	}
}

func TestServicesPreserveReleaseProvenance(t *testing.T) {
	client := New("http://rootguard-core.test", "test-token")
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/services" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		body := `[{"name":"core","displayName":"RootGuard Core","status":"running","health":"healthy","image":"rootguard-core:dev","version":"dev","revision":"abc123","created":"2026-08-01T00:00:00Z","source":"https://github.com/foxly-it/rootguard-core","immutable":false,"metadata":"complete","attestation":"not_applicable","attestedAt":"2026-08-01T12:00:00Z"}]`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})

	services, err := client.Services(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Attestation != "not_applicable" || services[0].Metadata != "complete" ||
		services[0].Version != "dev" || services[0].Revision != "abc123" || services[0].Source == "" || services[0].Immutable {
		t.Fatalf("release provenance was lost while decoding: %#v", services)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
