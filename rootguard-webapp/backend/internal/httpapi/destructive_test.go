package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestStatusRecorderUnwrapsForResponseController is the regression test
// for a round-3 correctness-review finding: statusRecorder implemented
// neither SetReadDeadline itself nor Unwrap(), so
// http.NewResponseController - used by HandleBackupRestore
// (api/updates.go) to extend the read deadline for a large backup/restore
// upload past the server's blanket 10s ReadTimeout - silently failed with
// http.ErrNotSupported every time it ran behind guardRestoreUpload/
// guardDestructive, which always wrap the ResponseWriter in a
// statusRecorder before calling through. Every restore upload was
// therefore still bound by the global 10s timeout regardless of the
// extension. Needs a real network connection, not httptest.NewRecorder -
// the recorder itself doesn't implement SetReadDeadline either, so it
// can't tell "unwrap is missing" apart from "nothing here ever supports
// this".
func TestStatusRecorderUnwrapsForResponseController(t *testing.T) {
	var direct, viaRecorder error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		direct = http.NewResponseController(w).SetReadDeadline(time.Now().Add(time.Minute))

		wrapped := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		viaRecorder = http.NewResponseController(wrapped).SetReadDeadline(time.Now().Add(time.Minute))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if direct != nil {
		t.Fatalf("sanity check failed: expected SetReadDeadline to succeed against the real connection directly, got %v", direct)
	}
	if viaRecorder != nil {
		t.Fatalf("expected SetReadDeadline to reach the real connection through *statusRecorder via Unwrap(), got %v", viaRecorder)
	}
}
