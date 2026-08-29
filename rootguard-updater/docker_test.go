package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteAtomicIgnoresStaleLegacyTempFile is the regression test for the
// same bug rootguard-core's internal/atomicfile fixed: os.OpenFile only
// applies the requested permission bits when it actually creates a file -
// a stale leftover at the old fixed name path+".tmp" (from an earlier
// failed run, or a planted symlink) would silently donate its own mode to
// every future write. os.CreateTemp's always-fresh, randomly named file
// makes that collision impossible; this proves a stale file sitting at
// the old fixed name specifically no longer has any effect at all.
func TestWriteAtomicIgnoresStaleLegacyTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	legacyTemp := path + ".tmp"
	if err := os.WriteFile(legacyTemp, []byte("stale garbage"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := writeAtomic(path, []byte("fresh")); err != nil {
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

func TestWriteAtomicWritesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := writeAtomic(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %q, want %q", data, "hello")
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no leftover temp files, found: %v", matches)
	}
}
