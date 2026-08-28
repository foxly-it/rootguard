package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileWritesContentAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.txt")
	if err := WriteFile(path, []byte("hello"), 0640); err != nil {
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
	// No leftover temp file on the success path.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover temp file, stat returned: %v", err)
	}
}

func TestWriteJSONMarshalsAndWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	value := map[string]int{"a": 1}
	if err := WriteJSON(path, value); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\n  \"a\": 1\n}" {
		t.Fatalf("unexpected JSON output: %q", data)
	}
}

// TestWriteFileCleansUpTempFileOnFailedRename is the regression test for
// what consolidating this pattern actually fixed: some of the copies this
// package replaced didn't remove their ".tmp" file when the rename step
// failed, leaving it behind indefinitely. Points path at a location
// os.Rename can never succeed against (a directory), so the write step
// succeeds but the rename doesn't.
func TestWriteFileCleansUpTempFileOnFailedRename(t *testing.T) {
	dir := t.TempDir()
	// path itself is a directory - os.Rename(temp, path) fails on every OS
	// when path already exists as a directory.
	path := filepath.Join(dir, "target")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(path, []byte("data"), 0600); err == nil {
		t.Fatal("expected the rename to fail against an existing directory")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected the temp file to be cleaned up after a failed rename, stat returned: %v", err)
	}
}
