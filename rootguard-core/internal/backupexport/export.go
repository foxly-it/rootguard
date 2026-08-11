package backupexport

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
)

const SchemaVersion = 1

var (
	ErrBusy              = errors.New("a backup export is already running")
	ErrInvalidPassphrase = errors.New("backup passphrase must contain at least 12 characters")
)

type CommandRunner func(context.Context, ...string) ([]byte, error)

type Source struct {
	ArchivePath string
	Path        string
}

type ContainerSource struct {
	ArchivePath string
	Container   string
	Path        string
}

type Options struct {
	DataDir          string
	LocalSources     []Source
	ContainerSources []ContainerSource
	Run              CommandRunner
}

type ManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion int            `json:"schema_version"`
	CreatedAt     time.Time      `json:"created_at"`
	Format        string         `json:"format"`
	Files         []ManifestFile `json:"files"`
}

type Exporter struct {
	mu               sync.Mutex
	dataDir          string
	localSources     []Source
	containerSources []ContainerSource
	run              CommandRunner
}

func New(options Options) *Exporter {
	if options.Run == nil {
		options.Run = runDocker
	}
	return &Exporter{dataDir: options.DataDir, localSources: options.LocalSources, containerSources: options.ContainerSources, run: options.Run}
}

func (e *Exporter) Export(ctx context.Context, passphrase string, destination io.Writer) (returnErr error) {
	if err := ValidatePassphrase(passphrase); err != nil {
		return err
	}
	if !e.mu.TryLock() {
		return ErrBusy
	}
	defer e.mu.Unlock()
	if err := os.MkdirAll(e.dataDir, 0700); err != nil {
		return fmt.Errorf("create protected export directory: %w", err)
	}
	stage, err := os.MkdirTemp(e.dataDir, ".stage-")
	if err != nil {
		return fmt.Errorf("create export staging directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(stage); err != nil {
			cleanupErr := fmt.Errorf("remove plaintext export staging directory: %w", err)
			if returnErr == nil {
				returnErr = cleanupErr
			} else {
				returnErr = errors.Join(returnErr, cleanupErr)
			}
		}
	}()
	if err := os.Chmod(stage, 0700); err != nil {
		return fmt.Errorf("protect export staging directory: %w", err)
	}
	for _, source := range e.localSources {
		if err := validateArchivePath(source.ArchivePath); err != nil {
			return err
		}
		if err := copyTree(source.Path, filepath.Join(stage, filepath.FromSlash(source.ArchivePath))); err != nil {
			return fmt.Errorf("stage %s: %w", source.ArchivePath, err)
		}
	}
	for _, source := range e.containerSources {
		if err := validateArchivePath(source.ArchivePath); err != nil {
			return err
		}
		target := filepath.Join(stage, filepath.FromSlash(source.ArchivePath))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if output, err := e.run(ctx, "cp", source.Container+":"+source.Path, target); err != nil {
			return fmt.Errorf("copy %s from %s: %w: %s", source.Path, source.Container, err, strings.TrimSpace(string(output)))
		}
	}
	files, err := inventory(stage)
	if err != nil {
		return err
	}
	manifest := Manifest{SchemaVersion: SchemaVersion, CreatedAt: time.Now().UTC(), Format: "tar+gzip+age-scrypt", Files: files}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), data, 0600); err != nil {
		return err
	}
	return encryptArchive(stage, passphrase, destination)
}

func ValidatePassphrase(passphrase string) error {
	if len([]rune(passphrase)) < 12 || len(passphrase) > 4096 {
		return ErrInvalidPassphrase
	}
	return nil
}

func inventory(root string) ([]ManifestFile, error) {
	files := []ManifestFile{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse symlink in backup source %q", path)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("refuse non-regular backup source %q", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, ManifestFile{Path: filepath.ToSlash(relative), Size: info.Size(), SHA256: hex.EncodeToString(hash.Sum(nil))})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, err
}

func encryptArchive(root, passphrase string, destination io.Writer) error {
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return err
	}
	encrypted, err := age.Encrypt(destination, recipient)
	if err != nil {
		return err
	}
	compressed := gzip.NewWriter(encrypted)
	archive := tar.NewWriter(compressed)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		header.Uid, header.Gid = 0, 0
		header.Uname, header.Gname = "", ""
		if err := archive.WriteHeader(header); err != nil || entry.IsDir() {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(archive, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if closeErr := archive.Close(); err == nil {
		err = closeErr
	}
	if closeErr := compressed.Close(); err == nil {
		err = closeErr
	}
	if closeErr := encrypted.Close(); err == nil {
		err = closeErr
	}
	return err
}

func copyTree(source, target string) error {
	info, err := os.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source is not a safe directory")
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse symlink %q", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("refuse non-regular file %q", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputErr, outputErr := input.Close(), output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		return outputErr
	})
}

func validateArchivePath(path string) error {
	clean := filepath.ToSlash(filepath.Clean(path))
	if path == "" || clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || clean != path {
		return fmt.Errorf("unsafe backup archive path %q", path)
	}
	return nil
}

func runDocker(ctx context.Context, arguments ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", arguments...).CombinedOutput()
}
