package httpapi

import (
	"os"
	"path/filepath"
)

// writeAtomicFile mirrors rootguard-core's internal/atomicfile.WriteFile -
// found in review: this package's own three persistence call sites
// (credentials, sessions, audit log) each hand-rolled the same fixed
// temp-name pattern a follow-up security review found and fixed in
// rootguard-core (path+".tmp" via a plain os.WriteFile - not
// concurrency-safe, follows an existing file/symlink at that name rather
// than refusing it, and silently inherits a stale leftover's permissions
// instead of applying the requested mode). Separate Go modules can't
// share an internal/ package directly (no shared module currently exists
// for this, and a prior round already judged standing one up not worth
// it for ~40 lines of stable logic) - so this is the same fix, ported
// here once and shared by all three call sites in this package instead
// of being triplicated (and left vulnerable) within it too.
func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
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
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}
