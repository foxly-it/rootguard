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
	assertNoLeftoverTempFiles(t, filepath.Dir(path))
}

// assertNoLeftoverTempFiles globs for any *.tmp file in dir rather than
// checking a fixed name - WriteFile's temp file now has a random suffix
// (see WriteFile's own doc comment on why), so there's no longer one
// predictable name to check for directly.
func assertNoLeftoverTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no leftover temp files in %s, found: %v", dir, matches)
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
	assertNoLeftoverTempFiles(t, dir)
}

// TestWriteFileIgnoresStaleLegacyTempFile is the regression test for the
// specific bug that motivated switching from the fixed name path+".tmp" to
// os.CreateTemp: os.OpenFile only applies the requested permission bits
// when it actually creates a new file - if a file already exists at the
// target name, its existing mode is left untouched. Under the old
// implementation, a stale path+".tmp" leftover from an earlier failed run
// (or one deliberately planted, e.g. as a symlink) would silently donate
// its own mode - and, for a symlink, its own target - to every future
// write, however it got there. os.CreateTemp's always-fresh, randomly
// named file makes that collision impossible; this test proves a stale
// file sitting at the *old* fixed name specifically no longer has any
// effect at all.
func TestWriteFileIgnoresStaleLegacyTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.txt")
	legacyTemp := path + ".tmp"
	if err := os.WriteFile(legacyTemp, []byte("stale garbage"), 0777); err != nil {
		t.Fatal(err)
	}

	if err := WriteFile(path, []byte("fresh"), 0600); err != nil {
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
	// The stale legacy-named file itself is untouched - not this
	// package's leftover to clean up, but critically also never read from
	// or written through.
	staleData, err := os.ReadFile(legacyTemp)
	if err != nil {
		t.Fatal(err)
	}
	if string(staleData) != "stale garbage" {
		t.Fatalf("stale legacy temp file was modified: %q", staleData)
	}
}
