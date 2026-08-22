package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	StateIdle     = "idle"
	StateChecking = "checking"
	StateUpdating = "updating"
	StateFailed   = "failed"
)

var (
	ErrBusy           = errors.New("an update operation is already running")
	ErrUnknownService = errors.New("unknown update service")
)

type CommandRunner func(context.Context, ...string) ([]byte, error)
type VerifyFunc func(context.Context, string) error

type ServiceSpec struct {
	Name                string
	DisplayName         string
	Container           string
	TargetImage         string
	ResolveTarget       func(ctx context.Context) (string, error)
	BackupPaths         []string
	OwnershipMigrations []VolumeOwnershipMigration
}

type VolumeOwnershipMigration struct {
	Volume string
	Path   string
	UID    int
	GID    int
}

type previousVolumeOwnership struct {
	migration VolumeOwnershipMigration
	owner     string
	changed   bool
}

type ServiceStatus struct {
	Name            string    `json:"name"`
	DisplayName     string    `json:"display_name"`
	CurrentImage    string    `json:"current_image,omitempty"`
	TargetImage     string    `json:"target_image"`
	CurrentID       string    `json:"current_id,omitempty"`
	CandidateID     string    `json:"candidate_id,omitempty"`
	UpdateAvailable bool      `json:"update_available"`
	CheckedAt       time.Time `json:"checked_at,omitempty"`
	Error           string    `json:"error,omitempty"`
}

type CleanupResult struct {
	RemovedImages  []string `json:"removed_images,omitempty"`
	RemovedVolumes []string `json:"removed_volumes,omitempty"`
	Skipped        []string `json:"skipped,omitempty"`
}

type HistoryEntry struct {
	Service   string        `json:"service"`
	Outcome   string        `json:"outcome"`
	FromID    string        `json:"from_id,omitempty"`
	ToID      string        `json:"to_id,omitempty"`
	Message   string        `json:"message"`
	Cleanup   CleanupResult `json:"cleanup"`
	CreatedAt time.Time     `json:"created_at"`
}

type Status struct {
	State         string          `json:"state"`
	ActiveService string          `json:"active_service,omitempty"`
	Message       string          `json:"message"`
	Services      []ServiceStatus `json:"services"`
	History       []HistoryEntry  `json:"history,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// DefaultComposeProject is the Docker Compose project name every production
// caller uses. Options.ComposeProject exists so tests can point composeUp at
// an isolated project instead of colliding with a real running deployment.
const DefaultComposeProject = "rootguard-dns"

type Options struct {
	DataDir        string
	ComposeDir     string
	ComposeProject string
	Run            CommandRunner
	Verify         VerifyFunc
	Services       []ServiceSpec
	VerifyAttempts int
	RetryDelay     time.Duration
}

type Manager struct {
	mu              sync.RWMutex
	status          Status
	dataDir         string
	composeDir      string
	composeProject  string
	run             CommandRunner
	verify          VerifyFunc
	specs           map[string]ServiceSpec
	selected        map[string]string
	backupRetention int
	backupError     string
	verifyAttempts  int
	retryDelay      time.Duration
}

func NewManager(options Options) *Manager {
	if options.Run == nil {
		options.Run = runDocker
	}
	if options.ComposeProject == "" {
		options.ComposeProject = DefaultComposeProject
	}
	if options.Verify == nil {
		options.Verify = func(context.Context, string) error { return nil }
	}
	if options.VerifyAttempts <= 0 {
		options.VerifyAttempts = 30
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = time.Second
	}
	manager := &Manager{
		dataDir:         options.DataDir,
		composeDir:      options.ComposeDir,
		composeProject:  options.ComposeProject,
		run:             options.Run,
		verify:          options.Verify,
		specs:           make(map[string]ServiceSpec, len(options.Services)),
		selected:        map[string]string{},
		backupRetention: DefaultBackupRetention,
		verifyAttempts:  options.VerifyAttempts,
		retryDelay:      options.RetryDelay,
		status: Status{
			State:     StateIdle,
			Message:   "Noch keine Update-Prüfung durchgeführt.",
			Services:  []ServiceStatus{},
			UpdatedAt: time.Now().UTC(),
		},
	}
	for _, spec := range options.Services {
		manager.specs[spec.Name] = spec
		manager.status.Services = append(manager.status.Services, ServiceStatus{
			Name: spec.Name, DisplayName: spec.DisplayName, TargetImage: spec.TargetImage,
		})
	}
	manager.load()
	manager.loadBackupSettings()
	manager.reconcileServices(options.Services)
	if manager.status.State == StateChecking || manager.status.State == StateUpdating {
		manager.status.State = StateFailed
		manager.status.Message = "Der vorherige Update-Vorgang wurde durch einen Neustart unterbrochen."
		manager.status.UpdatedAt = time.Now().UTC()
		_ = manager.persist()
	}
	return manager
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStatus(m.status)
}

func (m *Manager) RunExclusive(message string, operation func() error) error {
	m.mu.Lock()
	if m.busyLocked() {
		m.mu.Unlock()
		return ErrBusy
	}
	m.status.State = StateUpdating
	m.status.ActiveService = ""
	m.status.Message = message
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()

	err := operation()
	m.mu.Lock()
	m.status.State = StateIdle
	m.status.Message = "Geschützte Operation abgeschlossen."
	if err != nil {
		m.status.State = StateFailed
		m.status.Message = "Geschützte Operation fehlgeschlagen: " + err.Error()
	}
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()
	return err
}

func (m *Manager) StartCheck() (Status, error) {
	m.mu.Lock()
	if m.busyLocked() {
		m.mu.Unlock()
		return Status{}, ErrBusy
	}
	m.status.State = StateChecking
	m.status.ActiveService = ""
	m.status.Message = "Images werden geladen und mit den laufenden Containern verglichen."
	m.status.UpdatedAt = time.Now().UTC()
	m.clearServiceErrorsLocked()
	_ = m.persistLocked()
	status := cloneStatus(m.status)
	m.mu.Unlock()
	go m.check()
	return status, nil
}

func (m *Manager) StartUpdate(service string) (Status, error) {
	if _, ok := m.specs[service]; !ok {
		return Status{}, fmt.Errorf("%w: %s", ErrUnknownService, service)
	}
	m.mu.Lock()
	if m.busyLocked() {
		m.mu.Unlock()
		return Status{}, ErrBusy
	}
	m.status.State = StateUpdating
	m.status.ActiveService = service
	m.status.Message = "Sicherung und Update werden vorbereitet."
	m.status.UpdatedAt = time.Now().UTC()
	m.setServiceErrorLocked(service, "")
	_ = m.persistLocked()
	status := cloneStatus(m.status)
	m.mu.Unlock()
	go m.update(service)
	return status, nil
}

func (m *Manager) check() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	for _, service := range m.serviceNames() {
		spec := m.specs[service]
		m.setProgress(service, "Prüfe "+spec.DisplayName+".")
		targetImage := resolveTargetImage(ctx, spec)
		currentImage, currentID, err := m.inspectContainer(ctx, spec)
		if err != nil {
			m.setServiceResult(service, ServiceStatus{TargetImage: targetImage, Error: err.Error(), CheckedAt: time.Now().UTC()})
			continue
		}
		if _, err := m.run(ctx, "pull", targetImage); err != nil {
			m.setServiceResult(service, ServiceStatus{
				CurrentImage: currentImage, CurrentID: currentID, TargetImage: targetImage, Error: err.Error(), CheckedAt: time.Now().UTC(),
			})
			continue
		}
		targetImage = digestQualify(ctx, m.run, targetImage)
		candidateID, err := m.inspectImage(ctx, targetImage)
		result := ServiceStatus{
			CurrentImage: currentImage, CurrentID: currentID, TargetImage: targetImage, CandidateID: candidateID,
			UpdateAvailable: err == nil && currentID != candidateID, CheckedAt: time.Now().UTC(),
		}
		if err != nil {
			result.Error = err.Error()
		}
		m.setServiceResult(service, result)
	}

	m.mu.Lock()
	m.status.State = StateIdle
	m.status.ActiveService = ""
	m.status.Message = "Update-Prüfung abgeschlossen."
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *Manager) update(service string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	spec := m.specs[service]

	currentImage, oldID, err := m.inspectContainer(ctx, spec)
	if err != nil {
		m.fail(service, err)
		return
	}
	targetImage := resolveTargetImage(ctx, spec)
	m.setProgress(service, "Erstelle eine Sicherung der persistenten Dienstdaten.")
	backupDir, err := m.backup(ctx, spec)
	if err != nil {
		m.fail(service, fmt.Errorf("backup %s: %w", service, err))
		return
	}
	defer m.enforceBackupRetention()
	m.setProgress(service, "Lade das freigegebene Ziel-Image.")
	if _, err := m.run(ctx, "pull", targetImage); err != nil {
		m.fail(service, fmt.Errorf("pull target image: %w", err))
		return
	}
	targetImage = digestQualify(ctx, m.run, targetImage)
	candidateID, err := m.inspectImage(ctx, targetImage)
	if err != nil {
		m.fail(service, err)
		return
	}
	if candidateID == oldID {
		m.recordHistory(HistoryEntry{
			Service: service, Outcome: "no_change", FromID: oldID, ToID: candidateID,
			Message: "Der Dienst verwendet bereits das aktuelle Image.", CreatedAt: time.Now().UTC(),
		})
		m.finish(service, currentImage, oldID, candidateID, false, "Der Dienst verwendet bereits das aktuelle Image.")
		return
	}

	m.setProgress(service, "Migriere persistente Volume-Berechtigungen für das Ziel-Image.")
	previousOwnership, err := m.migrateVolumeOwnership(ctx, spec, oldID, candidateID)
	if err != nil {
		m.fail(service, fmt.Errorf("migrate persistent volume ownership: %w", err))
		return
	}

	m.setProgress(service, "Ersetze den Container kontrolliert.")
	if err := m.selectImage(service, targetImage); err != nil {
		if restoreErr := m.restoreVolumeOwnership(ctx, previousOwnership, oldID); restoreErr != nil {
			err = fmt.Errorf("%v; restore volume ownership: %w", err, restoreErr)
		}
		m.fail(service, err)
		return
	}
	err = m.composeUp(ctx, service)
	if err == nil {
		err = m.verifyWithRetry(ctx, service)
	}
	if err != nil {
		updateErr := err
		m.setProgress(service, "Gesundheitsprüfung fehlgeschlagen – vorheriges Image wird wiederhergestellt.")
		rollbackErr := m.rollback(ctx, spec, oldID, backupDir, previousOwnership)
		if rollbackErr != nil {
			m.recordHistory(HistoryEntry{
				Service: service, Outcome: "failed", FromID: oldID, ToID: candidateID,
				Message: fmt.Sprintf("Update und Rollback fehlgeschlagen: %v", rollbackErr), CreatedAt: time.Now().UTC(),
			})
			m.fail(service, fmt.Errorf("update failed: %v; rollback failed: %w", updateErr, rollbackErr))
			return
		}
		m.recordHistory(HistoryEntry{
			Service: service, Outcome: "rolled_back", FromID: oldID, ToID: candidateID,
			Message: "Das fehlerhafte Update wurde sicher zurückgesetzt.", CreatedAt: time.Now().UTC(),
		})
		m.fail(service, fmt.Errorf("update failed and was rolled back safely: %w", updateErr))
		return
	}
	entry := HistoryEntry{
		Service: service, Outcome: "success", FromID: oldID, ToID: candidateID,
		Message: spec.DisplayName + " wurde aktualisiert und erfolgreich geprüft.", CreatedAt: time.Now().UTC(),
	}
	m.recordHistory(entry)
	cleanup := m.cleanupAfterSuccess(ctx, service)
	m.attachCleanup(cleanup)
	m.finish(service, targetImage, candidateID, candidateID, false, spec.DisplayName+" wurde aktualisiert und erfolgreich geprüft.")
}

func (m *Manager) recordHistory(entry HistoryEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.History = append([]HistoryEntry{entry}, m.status.History...)
	if len(m.status.History) > 50 {
		m.status.History = m.status.History[:50]
	}
	_ = m.persistLocked()
}

func (m *Manager) attachCleanup(cleanup CleanupResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.status.History) > 0 {
		m.status.History[0].Cleanup = cleanup
	}
	_ = m.persistLocked()
}

func (m *Manager) rollback(
	ctx context.Context,
	spec ServiceSpec,
	oldID, backupDir string,
	previousOwnership []previousVolumeOwnership,
) error {
	if err := m.restoreVolumeOwnership(ctx, previousOwnership, oldID); err != nil {
		return fmt.Errorf("restore volume ownership: %w", err)
	}
	if err := m.selectImage(spec.Name, oldID); err != nil {
		return err
	}
	if err := m.composeUp(ctx, spec.Name); err != nil {
		return err
	}
	// The container is back on the known-good previous image at this point even
	// if the check below fails, rather than left on the candidate image that
	// already failed its own health check.
	if err := verifyBackupManifest(backupDir, spec); err != nil {
		return fmt.Errorf("verify backup integrity: %w", err)
	}
	for _, source := range spec.BackupPaths {
		name := filepath.Base(source)
		if _, err := m.run(ctx, "cp", filepath.Join(backupDir, name)+"/.", spec.Container+":"+source); err != nil {
			return fmt.Errorf("restore %s: %w", source, err)
		}
	}
	if _, err := m.run(ctx, "restart", spec.Container); err != nil {
		return err
	}
	return m.verifyWithRetry(ctx, spec.Name)
}

func (m *Manager) migrateVolumeOwnership(
	ctx context.Context,
	spec ServiceSpec,
	currentImage, candidateImage string,
) ([]previousVolumeOwnership, error) {
	previous := make([]previousVolumeOwnership, 0, len(spec.OwnershipMigrations))
	for _, migration := range spec.OwnershipMigrations {
		if migration.Volume == "" || migration.Path == "" || migration.UID < 0 || migration.GID < 0 {
			_ = m.restoreVolumeOwnership(ctx, previous, currentImage)
			return nil, fmt.Errorf("invalid ownership migration for %s", spec.Name)
		}
		owner, err := m.inspectVolumeOwnership(ctx, currentImage, migration)
		if err != nil {
			_ = m.restoreVolumeOwnership(ctx, previous, currentImage)
			return nil, err
		}
		target := strconv.Itoa(migration.UID) + ":" + strconv.Itoa(migration.GID)
		record := previousVolumeOwnership{migration: migration, owner: owner, changed: owner != target}
		previous = append(previous, record)
		if !record.changed {
			continue
		}
		if err := m.changeVolumeOwnership(ctx, candidateImage, migration, target); err != nil {
			_ = m.restoreVolumeOwnership(ctx, previous, currentImage)
			return nil, err
		}
	}
	return previous, nil
}

func (m *Manager) restoreVolumeOwnership(
	ctx context.Context,
	previous []previousVolumeOwnership,
	image string,
) error {
	for index := len(previous) - 1; index >= 0; index-- {
		record := previous[index]
		if !record.changed {
			continue
		}
		if err := m.changeVolumeOwnership(ctx, image, record.migration, record.owner); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) inspectVolumeOwnership(
	ctx context.Context,
	image string,
	migration VolumeOwnershipMigration,
) (string, error) {
	output, err := m.run(ctx,
		"run", "--rm",
		"--network", "none",
		"--user", "0:0",
		"--read-only",
		"--cap-drop", "ALL",
		"--volume", migration.Volume+":"+migration.Path+":ro",
		"--entrypoint", "/usr/bin/stat",
		image,
		"--format=%u:%g",
		migration.Path,
	)
	if err != nil {
		return "", fmt.Errorf("inspect %s ownership: %w", migration.Volume, err)
	}
	owner := strings.TrimSpace(string(output))
	parts := strings.Split(owner, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ownership %q for %s", owner, migration.Volume)
	}
	if _, err := strconv.ParseUint(parts[0], 10, 32); err != nil {
		return "", fmt.Errorf("invalid owner UID %q for %s", parts[0], migration.Volume)
	}
	if _, err := strconv.ParseUint(parts[1], 10, 32); err != nil {
		return "", fmt.Errorf("invalid owner GID %q for %s", parts[1], migration.Volume)
	}
	return owner, nil
}

func (m *Manager) changeVolumeOwnership(
	ctx context.Context,
	image string,
	migration VolumeOwnershipMigration,
	owner string,
) error {
	if _, err := m.run(ctx,
		"run", "--rm",
		"--network", "none",
		"--user", "0:0",
		"--read-only",
		"--cap-drop", "ALL",
		"--cap-add", "CHOWN",
		"--security-opt", "no-new-privileges:true",
		"--volume", migration.Volume+":"+migration.Path,
		"--entrypoint", "/usr/bin/chown",
		image,
		"--recursive",
		owner,
		migration.Path,
	); err != nil {
		return fmt.Errorf("change %s ownership to %s: %w", migration.Volume, owner, err)
	}
	return nil
}

func (m *Manager) backup(ctx context.Context, spec ServiceSpec) (string, error) {
	directory := filepath.Join(m.dataDir, "backups", time.Now().UTC().Format("20060102T150405.000000000Z"), spec.Name)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	for _, source := range spec.BackupPaths {
		if _, err := m.run(ctx, "cp", spec.Container+":"+source, directory); err != nil {
			return "", fmt.Errorf("copy %s: %w", source, err)
		}
	}
	if err := writeBackupManifest(directory, spec); err != nil {
		return "", err
	}
	return directory, nil
}

func (m *Manager) inspectContainer(ctx context.Context, spec ServiceSpec) (string, string, error) {
	output, err := m.run(ctx, "inspect", "--format", "{{.Config.Image}}|{{.Image}}", spec.Container)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("invalid image metadata for %s", spec.Container)
	}
	return parts[0], parts[1], nil
}

func (m *Manager) inspectImage(ctx context.Context, image string) (string, error) {
	output, err := m.run(ctx, "image", "inspect", "--format", "{{.Id}}", image)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(output))
	if id == "" {
		return "", fmt.Errorf("empty image ID for %s", image)
	}
	return id, nil
}

func (m *Manager) composeUp(ctx context.Context, service string) error {
	base := filepath.Join(m.composeDir, "compose.yaml")
	override := filepath.Join(m.dataDir, "updates.yaml")
	_, err := m.run(ctx, "compose", "--project-name", m.composeProject, "-f", base, "-f", override, "up", "-d", "--no-deps", service)
	return err
}

func (m *Manager) selectImage(service, image string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selected[service] = image
	return m.persistLocked()
}

func (m *Manager) verifyWithRetry(ctx context.Context, service string) error {
	var lastErr error
	for attempt := 0; attempt < m.verifyAttempts; attempt++ {
		verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		lastErr = m.verify(verifyCtx, service)
		cancel()
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.retryDelay):
		}
	}
	return lastErr
}

func (m *Manager) setProgress(service, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.ActiveService = service
	m.status.Message = message
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
}

func (m *Manager) setServiceResult(service string, result ServiceStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.status.Services {
		if m.status.Services[index].Name != service {
			continue
		}
		result.Name = m.status.Services[index].Name
		result.DisplayName = m.status.Services[index].DisplayName
		if result.TargetImage == "" {
			result.TargetImage = m.status.Services[index].TargetImage
		}
		m.status.Services[index] = result
		break
	}
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
}

func (m *Manager) finish(service, image, currentID, candidateID string, available bool, message string) {
	m.setServiceResult(service, ServiceStatus{
		CurrentImage: image, CurrentID: currentID, CandidateID: candidateID,
		UpdateAvailable: available, CheckedAt: time.Now().UTC(),
	})
	m.mu.Lock()
	m.status.State = StateIdle
	m.status.ActiveService = ""
	m.status.Message = message
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *Manager) fail(service string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = StateFailed
	m.status.ActiveService = ""
	m.status.Message = err.Error()
	m.status.UpdatedAt = time.Now().UTC()
	m.setServiceErrorLocked(service, err.Error())
	_ = m.persistLocked()
}

func (m *Manager) setServiceErrorLocked(service, message string) {
	for index := range m.status.Services {
		if m.status.Services[index].Name == service {
			m.status.Services[index].Error = message
			break
		}
	}
}

func (m *Manager) clearServiceErrorsLocked() {
	for index := range m.status.Services {
		m.status.Services[index].Error = ""
	}
}

func (m *Manager) busyLocked() bool {
	return m.status.State == StateChecking || m.status.State == StateUpdating
}

func (m *Manager) serviceNames() []string {
	names := make([]string, 0, len(m.status.Services))
	for _, service := range m.status.Services {
		names = append(names, service.Name)
	}
	return names
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "status.json"))
	if err == nil {
		var status Status
		if json.Unmarshal(data, &status) == nil && status.State != "" {
			m.status = status
		}
	}
	data, err = os.ReadFile(filepath.Join(m.dataDir, "images.json"))
	if err == nil {
		_ = json.Unmarshal(data, &m.selected)
	}
}

func (m *Manager) reconcileServices(specs []ServiceSpec) {
	previous := make(map[string]ServiceStatus, len(m.status.Services))
	for _, service := range m.status.Services {
		previous[service.Name] = service
	}
	m.status.Services = make([]ServiceStatus, 0, len(specs))
	for _, spec := range specs {
		service := previous[spec.Name]
		service.Name = spec.Name
		service.DisplayName = spec.DisplayName
		service.TargetImage = spec.TargetImage
		m.status.Services = append(m.status.Services, service)
	}
}

func (m *Manager) persist() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.persistLocked()
}

func (m *Manager) persistLocked() error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(m.dataDir, "status.json"), m.status); err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(m.dataDir, "images.json"), m.selected); err != nil {
		return err
	}
	return m.writeOverrideLocked()
}

func (m *Manager) writeOverrideLocked() error {
	var content strings.Builder
	content.WriteString("services:\n")
	for _, service := range m.serviceNames() {
		image := m.selected[service]
		if image == "" {
			image = m.specs[service].TargetImage
		}
		content.WriteString("  " + service + ":\n    image: " + strconv.Quote(image) + "\n")
	}
	return writeAtomic(filepath.Join(m.dataDir, "updates.yaml"), []byte(content.String()))
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

func writeAtomic(path string, data []byte) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func cloneStatus(status Status) Status {
	clone := status
	clone.Services = append([]ServiceStatus(nil), status.Services...)
	clone.History = append([]HistoryEntry(nil), status.History...)
	return clone
}

func runDocker(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("docker %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
