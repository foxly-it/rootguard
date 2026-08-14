package controlplane

import (
	"encoding/json"
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
