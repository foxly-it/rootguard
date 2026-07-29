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
		response := UnboundSettings{ResourceProfile: "medium"}
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

	active.ResourceProfile = "large"
	updated, err := client.UpdateUnboundSettings(context.Background(), active)
	if err != nil {
		t.Fatal(err)
	}
	if received.ResourceProfile != "large" || updated.ResourceProfile != "large" {
		t.Fatalf("resource profile was lost in proxy roundtrip: received=%q updated=%q", received.ResourceProfile, updated.ResourceProfile)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
