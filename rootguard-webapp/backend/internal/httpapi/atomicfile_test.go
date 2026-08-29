package httpapi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicFileWritesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := writeAtomicFile(path, []byte("hello"), 0640); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %q, want %q", data, "hello")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Fatalf("got mode %v, want 0640", info.Mode().Perm())
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no leftover temp files, found: %v", matches)
	}
}

// TestWriteAtomicFileIgnoresStaleLegacyTempFile is the regression test for
// the bug this shared helper closes across all three of this package's own
// call sites (credentials, sessions, audit log) at once: os.OpenFile only
// applies the requested permission bits when it actually creates a file -
// a stale leftover at the old fixed name path+".tmp" would silently donate
// its own mode to every future write. os.CreateTemp's always-fresh,
// randomly named file makes that collision impossible.
func TestWriteAtomicFileIgnoresStaleLegacyTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	legacyTemp := path + ".tmp"
	if err := os.WriteFile(legacyTemp, []byte("stale garbage"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := writeAtomicFile(path, []byte("fresh"), 0600); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fresh" {
		t.Fatalf("got %q, want %q", data, "fresh")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("got mode %v, want 0600 - the stale leftover's permissions leaked through", info.Mode().Perm())
	}
	staleData, err := os.ReadFile(legacyTemp)
	if err != nil {
		t.Fatal(err)
	}
	if string(staleData) != "stale garbage" {
		t.Fatalf("stale legacy temp file was modified: %q", staleData)
	}
}
