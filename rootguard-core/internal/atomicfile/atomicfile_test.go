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

func TestWriteFilesWritesEveryFileAtomically(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.txt")
	pathB := filepath.Join(dir, "b.json")
	err := WriteFiles([]File{
		{Path: pathA, Data: []byte("hello"), Mode: 0640},
		{Path: pathB, Data: []byte(`{"n":1}`), Mode: 0600},
	})
	if err != nil {
		t.Fatal(err)
	}
	dataA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataA) != "hello" {
		t.Fatalf("got %q, want %q", dataA, "hello")
	}
	infoA, err := os.Stat(pathA)
	if err != nil {
		t.Fatal(err)
	}
	if infoA.Mode().Perm() != 0640 {
		t.Fatalf("got mode %v, want 0640", infoA.Mode().Perm())
	}
	dataB, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataB) != `{"n":1}` {
		t.Fatalf("got %q, want %q", dataB, `{"n":1}`)
	}
	assertNoLeftoverTempFiles(t, dir)
}

// TestWriteFilesLeavesEveryFileUntouchedWhenAnyStagingFails is the
// regression test for the status.json/images.json split-brain scenario
// this function exists to close: unlike calling WriteFile once per file,
// a staging failure on a *later* file in the batch must not leave an
// *earlier* file's new content committed while the later one keeps its
// old content - either every file in the batch moves to its new
// generation, or (on any failure before renaming starts) none of them
// do. Points the second file at a path whose parent directory doesn't
// exist, so its os.CreateTemp step fails - after the first file's own
// temp file was already successfully staged.
func TestWriteFilesLeavesEveryFileUntouchedWhenAnyStagingFails(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(pathA, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	pathB := filepath.Join(dir, "missing-subdir", "b.txt")

	err := WriteFiles([]File{
		{Path: pathA, Data: []byte("new"), Mode: 0600},
		{Path: pathB, Data: []byte("new"), Mode: 0600},
	})
	if err == nil {
		t.Fatal("expected staging the second file to fail")
	}
	dataA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataA) != "original" {
		t.Fatalf("first file's content changed despite the batch failing: got %q, want %q", dataA, "original")
	}
	if _, err := os.Stat(pathB); !os.IsNotExist(err) {
		t.Fatalf("second file should not exist, got err=%v", err)
	}
	assertNoLeftoverTempFiles(t, dir)
}

// TestWriteFilesCleansUpRemainingTempFilesOnRenameFailure covers the one
// residual inconsistency window WriteFiles' own doc comment describes -
// a rename failing partway through the commit phase, after every file
// already staged successfully. An already-renamed file (pathA here)
// necessarily stays in its new state (nothing left to safely undo, the
// same as a single WriteFile's own rename-failure behavior for its one
// file) - what this test actually verifies is that every *unrenamed*
// file's temp file is still cleaned up rather than leaked, and the
// unrenamed target itself is left alone.
func TestWriteFilesCleansUpRemainingTempFilesOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.txt")
	pathB := filepath.Join(dir, "b-is-a-directory")
	if err := os.Mkdir(pathB, 0700); err != nil {
		t.Fatal(err)
	}

	err := WriteFiles([]File{
		{Path: pathA, Data: []byte("committed"), Mode: 0600},
		{Path: pathB, Data: []byte("new"), Mode: 0600},
	})
	if err == nil {
		t.Fatal("expected the second file's rename to fail against an existing directory")
	}
	dataA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	if string(dataA) != "committed" {
		t.Fatalf("got %q, want %q", dataA, "committed")
	}
	info, err := os.Stat(pathB)
	if err != nil || !info.IsDir() {
		t.Fatalf("second target should still be the original directory, got info=%v err=%v", info, err)
	}
	assertNoLeftoverTempFiles(t, dir)
}

func TestJSONFileMarshalsForWriteFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	file, err := JSONFile(path, map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteFiles([]File{file}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\n  \"a\": 1\n}" {
		t.Fatalf("unexpected JSON output: %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("got mode %v, want 0600", info.Mode().Perm())
	}
}
