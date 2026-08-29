// Package atomicfile is the write-temp-then-rename pattern this module's
// managers each hand-rolled their own slightly different copy of - found
// in review while addressing a security audit's code-compression
// suggestions. The copies had genuinely drifted: some cleaned up their
// temp file on a failed rename, some didn't (a stray ".tmp" left behind
// on error, harmless but untidy); one used json.Marshal, another
// json.MarshalIndent. One shared implementation instead.
package atomicfile

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path's contents: write to a fresh, unique
// temp file in the same directory first, then rename into place, so a
// reader (or a concurrent process) never observes a partially-written
// file, and a crash mid-write leaves whatever was at path before intact
// rather than corrupt.
//
// Found in a follow-up review: the original version wrote to the fixed
// name path+".tmp" via a plain os.WriteFile. Three real gaps in that:
//   - Not concurrency-safe - two overlapping WriteFile calls to the same
//     path shared that one temp name, so their writes could interleave
//     before either got to rename.
//   - os.WriteFile opens an existing file and truncates it in place if
//     one is already there - if path+".tmp" happened to be (or a prior
//     caller could arrange it to be) a symlink, the write would follow it
//     and land on whatever it pointed at, not a fresh file.
//   - The same "opens an existing file" behavior also meant a stale
//     leftover path+".tmp" from a previous failed run kept whatever mode
//     it already had; the requested mode was only ever applied when the
//     file didn't already exist.
//
// os.CreateTemp(dir, ...) fixes all three at once: it always creates a
// brand-new, uniquely-named file (O_EXCL under the hood), so there's
// never an existing name - symlink or otherwise - for it to collide with,
// and the mode is set explicitly below regardless of what any earlier
// leftover happened to have. The temp file is created in the same
// directory as path (not a shared system temp dir) so the final rename
// stays on one filesystem, which is what makes it atomic at all.
//
// Both the file and its parent directory are fsynced before returning -
// the file so its content is actually durable before the rename that
// makes it visible, the directory because a rename is itself a directory
// entry change that needs its own fsync to survive a crash (POSIX doesn't
// guarantee a renamed-to name survives a power loss otherwise, even
// though the file content does).
func WriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath) // no-op once the rename below has succeeded

	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}

// WriteJSON marshals value (indented, for a file a human might actually
// open) and writes it via WriteFile at mode 0600 - every current caller
// stores either credentials or operational state, never anything meant
// to be group- or world-readable.
func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, data, 0600)
}
