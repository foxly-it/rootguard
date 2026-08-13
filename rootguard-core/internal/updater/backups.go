package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultBackupRetention = 5
	MinBackupRetention     = 2
	MaxBackupRetention     = 50
	backupTimestampLayout  = "20060102T150405.000000000Z"
	manifestFileName       = "manifest.json"
)

var ErrInvalidBackupRetention = errors.New("invalid backup retention")

type BackupSettings struct {
	RetentionPerService int `json:"retention_per_service"`
}

type BackupServiceUsage struct {
	Service  string     `json:"service"`
	Count    int        `json:"count"`
	Bytes    int64      `json:"bytes"`
	OldestAt *time.Time `json:"oldest_at,omitempty"`
	NewestAt *time.Time `json:"newest_at,omitempty"`
}

type BackupStatus struct {
	Settings       BackupSettings       `json:"settings"`
	Count          int                  `json:"count"`
	ManagedBytes   int64                `json:"managed_bytes"`
	UnmanagedBytes int64                `json:"unmanaged_bytes"`
	Services       []BackupServiceUsage `json:"services"`
	LastError      string               `json:"last_error,omitempty"`
}

type managedBackup struct {
	path    string
	service string
	created time.Time
	bytes   int64
}

func (m *Manager) BackupStatus() (BackupStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.backupStatusLocked()
}

func (m *Manager) SetBackupRetention(retention int) (BackupStatus, error) {
	if retention < MinBackupRetention || retention > MaxBackupRetention {
		return BackupStatus{}, fmt.Errorf("%w: must be between %d and %d", ErrInvalidBackupRetention, MinBackupRetention, MaxBackupRetention)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busyLocked() {
		return BackupStatus{}, ErrBusy
	}
	m.backupRetention = retention
	if err := m.persistBackupSettingsLocked(); err != nil {
		return BackupStatus{}, err
	}
	m.backupError = ""
	if err := m.pruneBackupsLocked(); err != nil {
		m.backupError = err.Error()
	}

	return m.backupStatusLocked()
}

func (m *Manager) enforceBackupRetention() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backupError = ""
	if err := m.pruneBackupsLocked(); err != nil {
		m.backupError = err.Error()
	}
}

func (m *Manager) backupStatusLocked() (BackupStatus, error) {
	backups, unmanaged, err := m.scanBackupsLocked()
	status := BackupStatus{
		Settings:       BackupSettings{RetentionPerService: m.backupRetention},
		UnmanagedBytes: unmanaged,
		Services:       make([]BackupServiceUsage, 0, len(m.specs)),
		LastError:      m.backupError,
	}
	if err != nil {
		return status, err
	}
	usage := make(map[string]*BackupServiceUsage, len(m.specs))
	for _, name := range m.serviceNames() {
		item := &BackupServiceUsage{Service: name}
		usage[name] = item
		status.Services = append(status.Services, *item)
	}
	for _, backup := range backups {
		item := usage[backup.service]
		item.Count++
		item.Bytes += backup.bytes
		created := backup.created
		if item.OldestAt == nil || created.Before(*item.OldestAt) {
			item.OldestAt = &created
		}
		if item.NewestAt == nil || created.After(*item.NewestAt) {
			item.NewestAt = &created
		}
		status.Count++
		status.ManagedBytes += backup.bytes
	}
	for index := range status.Services {
		status.Services[index] = *usage[status.Services[index].Service]
	}

	return status, nil
}

func (m *Manager) pruneBackupsLocked() error {
	backups, _, err := m.scanBackupsLocked()
	if err != nil {
		return err
	}
	byService := make(map[string][]managedBackup, len(m.specs))
	for _, backup := range backups {
		byService[backup.service] = append(byService[backup.service], backup)
	}
	for _, serviceBackups := range byService {
		sort.Slice(serviceBackups, func(i, j int) bool { return serviceBackups[i].created.After(serviceBackups[j].created) })
		if len(serviceBackups) <= m.backupRetention {
			continue
		}
		for _, backup := range serviceBackups[m.backupRetention:] {
			if err := removeManagedBackup(m.backupRoot(), backup.path); err != nil {
				return err
			}
			_ = os.Remove(filepath.Dir(backup.path))
		}
	}

	return nil
}

func (m *Manager) scanBackupsLocked() ([]managedBackup, int64, error) {
	root := m.backupRoot()
	timestamps, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []managedBackup{}, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read backup directory: %w", err)
	}
	backups := make([]managedBackup, 0)
	var unmanaged int64
	for _, timestampEntry := range timestamps {
		timestampPath := filepath.Join(root, timestampEntry.Name())
		created, parseErr := time.Parse(backupTimestampLayout, timestampEntry.Name())
		if parseErr != nil || !timestampEntry.IsDir() || timestampEntry.Type()&os.ModeSymlink != 0 {
			unmanaged += treeSize(timestampPath)
			continue
		}
		serviceEntries, readErr := os.ReadDir(timestampPath)
		if readErr != nil {
			return nil, unmanaged, fmt.Errorf("read backup set %q: %w", timestampEntry.Name(), readErr)
		}
		for _, serviceEntry := range serviceEntries {
			servicePath := filepath.Join(timestampPath, serviceEntry.Name())
			if !serviceEntry.IsDir() || serviceEntry.Type()&os.ModeSymlink != 0 || !m.validBackupManifest(servicePath, serviceEntry.Name()) {
				unmanaged += treeSize(servicePath)
				continue
			}
			backups = append(backups, managedBackup{path: servicePath, service: serviceEntry.Name(), created: created, bytes: treeSize(servicePath)})
		}
	}

	return backups, unmanaged, nil
}

func (m *Manager) validBackupManifest(directory, service string) bool {
	spec, ok := m.specs[service]
	if !ok {
		return false
	}
	path := filepath.Join(directory, manifestFileName)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var manifest backupManifest
	return json.Unmarshal(data, &manifest) == nil && manifest.Service == service && manifest.Container == spec.Container
}

// backupManifest records the files copied into a pre-update snapshot so a
// later restore can detect a partial or corrupted docker cp before trusting
// it, instead of silently restoring bad data.
type backupManifest struct {
	Service   string       `json:"service"`
	Container string       `json:"container"`
	Image     string       `json:"image"`
	Files     []backupFile `json:"files"`
}

type backupFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// writeBackupManifest checksums every file already copied into directory and
// writes manifest.json describing them. It must run after all sources are
// copied and before the directory is relied upon for rollback.
func writeBackupManifest(directory string, spec ServiceSpec) error {
	files, err := checksumTree(directory)
	if err != nil {
		return fmt.Errorf("checksum backup: %w", err)
	}
	manifest := backupManifest{Service: spec.Name, Container: spec.Container, Image: spec.TargetImage, Files: files}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, manifestFileName), data, 0600)
}

// verifyBackupManifest recomputes checksums for directory and compares them
// against its manifest.json, so a rollback refuses a partial or corrupted
// snapshot instead of restoring it into a running container.
func verifyBackupManifest(directory string, spec ServiceSpec) error {
	data, err := os.ReadFile(filepath.Join(directory, manifestFileName))
	if err != nil {
		return fmt.Errorf("read backup manifest: %w", err)
	}
	var manifest backupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse backup manifest: %w", err)
	}
	if manifest.Service != spec.Name || manifest.Container != spec.Container {
		return fmt.Errorf("backup manifest does not match %s", spec.Name)
	}
	current, err := checksumTree(directory)
	if err != nil {
		return fmt.Errorf("checksum backup: %w", err)
	}
	if len(current) != len(manifest.Files) {
		return fmt.Errorf("backup file count changed: expected %d, found %d", len(manifest.Files), len(current))
	}
	expected := make(map[string]backupFile, len(manifest.Files))
	for _, file := range manifest.Files {
		expected[file.Path] = file
	}
	for _, file := range current {
		want, ok := expected[file.Path]
		if !ok {
			return fmt.Errorf("unexpected backup file %q", file.Path)
		}
		if want.SHA256 != file.SHA256 || want.Size != file.Size {
			return fmt.Errorf("backup file %q failed integrity check", file.Path)
		}
	}
	return nil
}

// checksumTree hashes every regular file under root except manifest.json
// itself, refusing symlinks the same way the portable backup export does.
func checksumTree(root string) ([]backupFile, error) {
	manifestPath := filepath.Join(root, manifestFileName)
	files := []backupFile{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == manifestPath || entry.IsDir() {
			return nil
		}
		file, err := checksumFile(root, path, entry)
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, err
}

func checksumFile(root, path string, entry fs.DirEntry) (backupFile, error) {
	if entry.Type()&os.ModeSymlink != 0 {
		return backupFile{}, fmt.Errorf("refuse symlink in backup %q", path)
	}
	info, err := entry.Info()
	if err != nil || !info.Mode().IsRegular() {
		return backupFile{}, fmt.Errorf("refuse non-regular backup entry %q", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return backupFile{}, err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return backupFile{}, copyErr
	}
	if closeErr != nil {
		return backupFile{}, closeErr
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return backupFile{}, err
	}
	return backupFile{Path: filepath.ToSlash(relative), Size: info.Size(), SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (m *Manager) backupRoot() string {
	return filepath.Join(m.dataDir, "backups")
}

func (m *Manager) persistBackupSettingsLocked() error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(m.dataDir, "backup-settings.json"), BackupSettings{RetentionPerService: m.backupRetention})
}

func (m *Manager) loadBackupSettings() {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "backup-settings.json"))
	var settings BackupSettings
	if err == nil && json.Unmarshal(data, &settings) == nil && settings.RetentionPerService >= MinBackupRetention && settings.RetentionPerService <= MaxBackupRetention {
		m.backupRetention = settings.RetentionPerService
	}
}

func treeSize(root string) int64 {
	var size int64
	_ = filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func removeManagedBackup(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || strings.Count(relative, string(filepath.Separator)) != 1 {
		return fmt.Errorf("refuse unsafe backup path %q", path)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse non-directory backup path %q", path)
	}
	return os.RemoveAll(path)
}
