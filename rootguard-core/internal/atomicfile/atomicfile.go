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
)

// WriteFile atomically replaces path's contents: write to path+".tmp"
// first, then rename into place, so a reader (or a concurrent process)
// never observes a partially-written file, and a crash mid-write leaves
// whatever was at path before intact rather than corrupt. The temp file
// is removed if the rename fails; a successful rename removes it as a
// side effect of the rename itself, so there's nothing to clean up on
// that path.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
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
