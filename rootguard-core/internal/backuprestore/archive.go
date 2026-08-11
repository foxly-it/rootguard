package backuprestore

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/installer"
)

const (
	MaxEncryptedBytes = 1 << 30
	MaxExpandedBytes  = 4 << 30
	MaxFiles          = 100000
	MaxManifestBytes  = 16 << 20
)

var allowedRoots = []string{
	"rootguard/unbound/",
	"rootguard/adguard/",
	"rootguard/adguard-auth/",
	"rootguard/installation/",
	"services/adguard/config/",
	"services/adguard/work/",
	"services/unbound/state/",
}

type Preview struct {
	SchemaVersion int              `json:"schema_version"`
	CreatedAt     time.Time        `json:"created_at"`
	FileCount     int              `json:"file_count"`
	ExpandedBytes int64            `json:"expanded_bytes"`
	Config        installer.Config `json:"config"`
}

// Extract decrypts and validates an export before returning a private staging
// directory.  The caller owns the directory and must remove it on every exit.
func Extract(dataDir, passphrase string, encrypted io.Reader) (_ string, preview Preview, returnErr error) {
	if err := backupexport.ValidatePassphrase(passphrase); err != nil {
		return "", Preview{}, err
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", Preview{}, fmt.Errorf("create restore directory: %w", err)
	}
	stage, err := os.MkdirTemp(dataDir, ".restore-")
	if err != nil {
		return "", Preview{}, fmt.Errorf("create restore staging directory: %w", err)
	}
	if err := os.Chmod(stage, 0700); err != nil {
		_ = os.RemoveAll(stage)
		return "", Preview{}, err
	}
	defer func() {
		if returnErr != nil {
			returnErr = errors.Join(returnErr, os.RemoveAll(stage))
		}
	}()

	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return "", Preview{}, err
	}
	limited := &io.LimitedReader{R: encrypted, N: MaxEncryptedBytes + 1}
	decrypted, err := age.Decrypt(limited, identity)
	if err != nil {
		return "", Preview{}, fmt.Errorf("decrypt backup: %w", err)
	}
	compressed, err := gzip.NewReader(decrypted)
	if err != nil {
		return "", Preview{}, fmt.Errorf("open compressed backup: %w", err)
	}
	defer compressed.Close()

	archive := tar.NewReader(compressed)
	seen := map[string]bool{}
	var manifestData []byte
	var expanded int64
	for count := 0; ; count++ {
		if count > MaxFiles {
			return "", Preview{}, fmt.Errorf("backup contains more than %d entries", MaxFiles)
		}
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", Preview{}, fmt.Errorf("read backup archive: %w", err)
		}
		name, err := safeName(header.Name)
		if err != nil {
			return "", Preview{}, err
		}
		if seen[name] {
			return "", Preview{}, fmt.Errorf("duplicate backup path %q", name)
		}
		seen[name] = true
		if header.Typeflag == tar.TypeDir {
			if name != "manifest.json" && !allowedDirectory(name) {
				return "", Preview{}, fmt.Errorf("unexpected backup directory %q", name)
			}
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 {
			return "", Preview{}, fmt.Errorf("refuse non-regular backup entry %q", name)
		}
		expanded += header.Size
		if expanded > MaxExpandedBytes {
			return "", Preview{}, fmt.Errorf("expanded backup exceeds %d bytes", MaxExpandedBytes)
		}
		if name == "manifest.json" {
			if header.Size > MaxManifestBytes {
				return "", Preview{}, errors.New("backup manifest is too large")
			}
			manifestData, err = io.ReadAll(io.LimitReader(archive, MaxManifestBytes+1))
			if err != nil {
				return "", Preview{}, err
			}
			continue
		}
		if !allowedPath(name) {
			return "", Preview{}, fmt.Errorf("unexpected backup path %q", name)
		}
		target := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", Preview{}, err
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return "", Preview{}, err
		}
		_, copyErr := io.CopyN(file, archive, header.Size)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return "", Preview{}, errors.Join(copyErr, closeErr)
		}
	}
	if limited.N == 0 {
		return "", Preview{}, fmt.Errorf("encrypted backup exceeds %d bytes", MaxEncryptedBytes)
	}
	manifest, err := validateManifest(stage, manifestData)
	if err != nil {
		return "", Preview{}, err
	}
	config, err := restoredConfig(stage)
	if err != nil {
		return "", Preview{}, err
	}
	return stage, Preview{SchemaVersion: manifest.SchemaVersion, CreatedAt: manifest.CreatedAt, FileCount: len(manifest.Files), ExpandedBytes: expanded, Config: config}, nil
}

func safeName(name string) (string, error) {
	name = strings.TrimSuffix(name, "/")
	if name == "" || strings.ContainsRune(name, '\x00') || path.IsAbs(name) || path.Clean(name) != name || name == "." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("unsafe backup path %q", name)
	}
	return name, nil
}

func allowedPath(name string) bool {
	for _, root := range allowedRoots {
		if strings.HasPrefix(name, root) || strings.TrimSuffix(root, "/") == strings.TrimSuffix(name, "/") {
			return true
		}
	}
	return false
}

func allowedDirectory(name string) bool {
	name = strings.TrimSuffix(name, "/")
	for _, root := range allowedRoots {
		root = strings.TrimSuffix(root, "/")
		if name == root || strings.HasPrefix(root, name+"/") || strings.HasPrefix(name, root+"/") {
			return true
		}
	}
	return false
}

func validateManifest(stage string, data []byte) (backupexport.Manifest, error) {
	if len(data) == 0 {
		return backupexport.Manifest{}, errors.New("backup manifest is missing")
	}
	var manifest backupexport.Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("decode backup manifest: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return manifest, errors.New("backup manifest contains trailing data")
	}
	if manifest.SchemaVersion != backupexport.SchemaVersion || manifest.Format != "tar+gzip+age-scrypt" || manifest.CreatedAt.IsZero() {
		return manifest, errors.New("unsupported backup manifest")
	}
	expected := append([]backupexport.ManifestFile(nil), manifest.Files...)
	sort.Slice(expected, func(i, j int) bool { return expected[i].Path < expected[j].Path })
	actual := make([]backupexport.ManifestFile, 0, len(expected))
	err := filepath.WalkDir(stage, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(stage, filePath)
		if err != nil {
			return err
		}
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		hash := sha256.New()
		size, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		actual = append(actual, backupexport.ManifestFile{Path: filepath.ToSlash(relative), Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
		return nil
	})
	if err != nil {
		return manifest, err
	}
	sort.Slice(actual, func(i, j int) bool { return actual[i].Path < actual[j].Path })
	if len(actual) != len(expected) {
		return manifest, errors.New("backup manifest does not match archive inventory")
	}
	for index := range actual {
		if actual[index] != expected[index] || !allowedPath(expected[index].Path) {
			return manifest, fmt.Errorf("backup manifest mismatch at %q", actual[index].Path)
		}
	}
	for _, required := range []string{"rootguard/adguard/credentials.json", "services/adguard/config/AdGuardHome.yaml", "rootguard/installation/status.json"} {
		found := false
		for _, file := range actual {
			if file.Path == required {
				found = true
				break
			}
		}
		if !found {
			return manifest, fmt.Errorf("backup is missing required file %q", required)
		}
	}
	return manifest, nil
}

func restoredConfig(stage string) (installer.Config, error) {
	data, err := os.ReadFile(filepath.Join(stage, "rootguard", "installation", "status.json"))
	if err != nil {
		return installer.Config{}, errors.New("backup has no installation status")
	}
	var status installer.Status
	if json.Unmarshal(data, &status) != nil || status.State != installer.StateInstalled || status.Config == nil {
		return installer.Config{}, errors.New("backup was not created from an installed RootGuard instance")
	}
	return *status.Config, nil
}
