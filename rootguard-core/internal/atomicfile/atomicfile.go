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

	if err := stageFile(temp, data, mode); err != nil {
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

// File is one entry for WriteFiles: a target path, its new content, and
// the mode to create it with.
type File struct {
	Path string
	Data []byte
	Mode os.FileMode
}

// WriteFiles atomically replaces the contents of multiple, independent
// files as a single unit. Found in review: a caller with related state
// split across more than one file (e.g. updater.Manager's
// status.json/images.json/updates.yaml, all derived from the same
// in-memory state) used to call WriteFile once per file - each
// individually atomic, but a failure partway through (e.g. the second
// file's write failing for any of the usual reasons: disk full,
// permissions, an I/O error) left the first file's already-committed new
// content and the remaining files' untouched old content in two
// different "generations" of the same logical state, silently
// inconsistent with each other on the very next read - exactly the
// status.json/images.json split-brain scenario described.
//
// Every file is staged first - written to its own fresh temp file in its
// target's directory and fsynced - before any of them is renamed into
// place, and none are renamed unless every single stage succeeded: a
// staging failure leaves every target file completely untouched, the
// same guarantee a single WriteFile call already gives for one file.
//
// Renaming several files can never be one atomic operation on POSIX -
// each rename(2) is its own syscall - so this cannot close the window
// entirely. What it does is narrow it from "an arbitrarily slow write
// and fsync of a later file, which can fail for many mundane reasons"
// down to "the brief moment between two rename syscalls, both of which
// only run after every write in the batch has already durably
// succeeded" - the best available guarantee for multiple independent
// files without a write-ahead log or combining them into one file, which
// existing on-disk formats and external readers (docker compose's own
// -f-loaded updates.yaml, an operator's status.json) make impractical
// here.
func WriteFiles(files []File) error {
	type staged struct {
		tempPath string
		path     string
		dir      string
	}
	commits := make([]staged, 0, len(files))
	cleanup := func() {
		for _, s := range commits {
			os.Remove(s.tempPath)
		}
	}
	for _, file := range files {
		dir := filepath.Dir(file.Path)
		temp, err := os.CreateTemp(dir, "."+filepath.Base(file.Path)+".*.tmp")
		if err != nil {
			cleanup()
			return err
		}
		tempPath := temp.Name()
		if err := stageFile(temp, file.Data, file.Mode); err != nil {
			os.Remove(tempPath)
			cleanup()
			return err
		}
		commits = append(commits, staged{tempPath: tempPath, path: file.Path, dir: dir})
	}
	dirsSynced := make(map[string]bool, len(commits))
	for _, s := range commits {
		if err := os.Rename(s.tempPath, s.path); err != nil {
			// Every remaining temp file (this one's rename failed, so it's
			// still there; any not yet reached in this loop) is cleaned up
			// below - only files already renamed in a prior loop iteration
			// are left in their new state, which is the same
			// already-committed, can't-safely-undo situation a single
			// WriteFile's own rename failure leaves its one file in.
			for _, remaining := range commits {
				os.Remove(remaining.tempPath)
			}
			return err
		}
		dirsSynced[s.dir] = true
	}
	for dir := range dirsSynced {
		if err := syncDir(dir); err != nil {
			return err
		}
	}
	return nil
}

// JSONFile marshals value as indented JSON (mode 0600, matching
// WriteJSON) into a File entry for WriteFiles.
func JSONFile(path string, value any) (File, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return File{}, err
	}
	return File{Path: path, Data: data, Mode: 0600}, nil
}

func stageFile(temp *os.File, data []byte, mode os.FileMode) error {
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
	return temp.Close()
}
