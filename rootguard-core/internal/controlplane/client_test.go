package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The updater's status response includes a "history" entry once an update
// has run. Status previously had no History field, so json.Decode silently
// dropped it - Status() would report success with no way to see the
// outcome it just reported. Guards against that regressing.
func TestStatusPreservesHistory(t *testing.T) {
	const body = `{
		"state": "idle",
		"message": "Core und WebApp wurden aktualisiert und erfolgreich geprüft.",
		"services": [],
		"history": [
			{
				"outcome": "success",
				"from_ids": {"core": "sha256:old"},
				"to_ids": {"core": "sha256:new"},
				"message": "done",
				"cleanup": {"removed_images": ["sha256:old"]},
				"created_at": "2026-01-01T00:00:00Z"
			}
		],
		"updated_at": "2026-01-01T00:00:01Z"
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	status, err := client.Status(t.Context())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(status.History) != 1 {
		t.Fatalf("History = %v, want 1 entry", status.History)
	}
	if status.History[0].Outcome != "success" {
		t.Errorf("History[0].Outcome = %q, want success", status.History[0].Outcome)
	}
	if status.History[0].ToIDs["core"] != "sha256:new" {
		t.Errorf("History[0].ToIDs[core] = %q, want sha256:new", status.History[0].ToIDs["core"])
	}

	var roundTrip Status
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(roundTrip.History) != 1 {
		t.Fatalf("round-tripped History = %v, want 1 entry", roundTrip.History)
	}
}

// TestCheckSendsResolvedTargetImages guards against Check() silently
// ignoring registered resolvers: the updater's own control-plane network
// is deliberately internet-isolated, so Core must resolve live release
// images itself and forward them in the request body, not leave the
// updater to fall back to its static pins every time.
func TestCheckSendsResolvedTargetImages(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"checking","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	client.WithTargetResolver("core", func(context.Context) (string, error) {
		return "ghcr.io/foxly-it/rootguard-core:1.0.14", nil
	})
	client.WithTargetResolver("webapp", func(context.Context) (string, error) {
		return "ghcr.io/foxly-it/rootguard-webapp:1.0.14", nil
	})

	if _, err := client.Check(t.Context()); err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	var payload struct {
		TargetImages map[string]string `json:"target_images"`
	}
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("decode request body: %v (body: %s)", err, receivedBody)
	}
	if payload.TargetImages["core"] != "ghcr.io/foxly-it/rootguard-core:1.0.14" ||
		payload.TargetImages["webapp"] != "ghcr.io/foxly-it/rootguard-webapp:1.0.14" {
		t.Fatalf("expected both resolved images in the request, got %#v", payload.TargetImages)
	}
}

// TestUpdateSendsNoTargetsWhenOneResolverFails is the regression test for the
// upgrade journey failure of 2026-11-13: Core's lookup worked, WebApp's
// didn't, and passing on just Core's override moved Core to 1.0.14 while
// WebApp stayed on 1.0.13 - reported as a successful update. One failed
// resolver must leave both components on the updater's own pins.
func TestUpdateSendsNoTargetsWhenOneResolverFails(t *testing.T) {
	var receivedLength int64 = -1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLength = r.ContentLength
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"updating","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	client.WithTargetResolver("core", func(context.Context) (string, error) {
		return "ghcr.io/foxly-it/rootguard-core:1.0.14", nil
	})
	client.WithTargetResolver("webapp", func(context.Context) (string, error) {
		return "", errors.New("GitHub API rate limit reached")
	})

	if _, err := client.Update(t.Context()); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if receivedLength > 0 {
		t.Fatalf("expected no target overrides after a failed resolver, got Content-Length %d", receivedLength)
	}
}

// TestCheckOmitsBodyWithoutResolvers guards against always sending a
// (possibly empty) JSON body: with no resolvers registered, Check()'s
// request must look exactly like it did before this feature existed.
func TestCheckOmitsBodyWithoutResolvers(t *testing.T) {
	var receivedLength int64 = -1
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLength = r.ContentLength
		receivedContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"checking","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	if _, err := client.Check(t.Context()); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if receivedLength > 0 {
		t.Fatalf("expected no request body without resolvers, got Content-Length %d", receivedLength)
	}
	if receivedContentType != "" {
		t.Fatalf("expected no Content-Type header without a body, got %q", receivedContentType)
	}
}
