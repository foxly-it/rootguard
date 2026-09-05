package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	stateIdle     = "idle"
	stateChecking = "checking"
	stateUpdating = "updating"
	stateFailed   = "failed"
)

type serviceSpec struct {
	Name        string
	DisplayName string
	Container   string
	TargetImage string
	HealthURL   string
}

type serviceStatus struct {
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

type cleanupResult struct {
	RemovedImages  []string `json:"removed_images,omitempty"`
	RemovedVolumes []string `json:"removed_volumes,omitempty"`
	Skipped        []string `json:"skipped,omitempty"`
}

type historyEntry struct {
	Outcome   string            `json:"outcome"`
	FromIDs   map[string]string `json:"from_ids,omitempty"`
	ToIDs     map[string]string `json:"to_ids,omitempty"`
	Message   string            `json:"message"`
	Cleanup   cleanupResult     `json:"cleanup"`
	CreatedAt time.Time         `json:"created_at"`
}

type status struct {
	State     string          `json:"state"`
	Message   string          `json:"message"`
	Services  []serviceStatus `json:"services"`
	History   []historyEntry  `json:"history,omitempty"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type runner func(context.Context, ...string) ([]byte, error)

type manager struct {
	mu                  sync.RWMutex
	status              status
	specs               []serviceSpec
	dataDir             string
	composeFile         string
	projectName         string
	run                 runner
	httpClient          *http.Client
	skipPull            bool
	verifyAttempts      int
	attestationVerifier attestationVerifierFunc
}

var errBusy = errors.New("an updater operation is already running")

func newManager(dataDir, composeFile, projectName string, specs []serviceSpec, run runner) *manager {
	m := &manager{
		dataDir: dataDir, composeFile: composeFile, projectName: projectName,
		specs: specs, run: run, httpClient: &http.Client{Timeout: 8 * time.Second},
		verifyAttempts:      45,
		attestationVerifier: verifyAttestation,
		status:              status{State: stateIdle, Message: "Noch keine Control-Plane-Prüfung durchgeführt.", UpdatedAt: time.Now().UTC()},
	}
	for _, spec := range specs {
		m.status.Services = append(m.status.Services, serviceStatus{
			Name: spec.Name, DisplayName: spec.DisplayName, TargetImage: spec.TargetImage,
		})
	}
	m.load()
	m.reconcile()
	if m.status.State == stateChecking || m.status.State == stateUpdating {
		m.status.State = stateFailed
		m.status.Message = "Der vorherige Control-Plane-Vorgang wurde unterbrochen."
		m.status.UpdatedAt = time.Now().UTC()
		_ = m.persist()
	}
	return m
}

func (m *manager) Status() status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStatus(m.status)
}

// StartCheck begins a check. targetImages optionally overrides a
// service's static TargetImage pin for this run only (e.g. a live
// release resolved by rootguard-core, which - unlike this network-
// isolated control-plane updater - has outbound internet access); a
// service with no entry keeps using its configured static pin.
func (m *manager) StartCheck(targetImages map[string]string) (status, error) {
	if err := validateTargetOverrides(m.specs, targetImages); err != nil {
		return status{}, err
	}
	return m.start(stateChecking, "Core- und WebApp-Images werden geprüft.", func() { m.check(targetImages) })
}

func (m *manager) StartUpdate(targetImages map[string]string) (status, error) {
	if err := validateTargetOverrides(m.specs, targetImages); err != nil {
		return status{}, err
	}
	return m.start(stateUpdating, "Das atomare Control-Plane-Update wird vorbereitet.", func() { m.update(targetImages) })
}

func (m *manager) start(state, message string, fn func()) (status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busyLocked() {
		return status{}, errBusy
	}
	m.status.State = state
	m.status.Message = message
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	go fn()
	return cloneStatus(m.status), nil
}

func (m *manager) check(targetImages map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	results := make([]serviceStatus, 0, len(m.specs))
	for _, spec := range m.specs {
		m.progress("Prüfe " + spec.DisplayName + ".")
		targetImage := targetImageFor(spec, targetImages)
		currentImage, currentID, err := m.inspectContainer(ctx, spec.Container)
		if err == nil && !m.skipPull {
			pullOutput, pullErr := m.run(ctx, "pull", targetImage)
			if pullErr != nil {
				err = pullErr
			} else if qualified, ok := digestFromPullOutput(targetImage, pullOutput); ok {
				targetImage = qualified
			} else {
				targetImage = digestQualify(ctx, m.run, targetImage)
			}
		}
		result := serviceStatus{Name: spec.Name, DisplayName: spec.DisplayName, TargetImage: targetImage, CheckedAt: time.Now().UTC()}
		result.CurrentImage, result.CurrentID = currentImage, currentID
		if err == nil {
			result.CandidateID, err = m.inspectImage(ctx, targetImage)
		}
		if err != nil {
			result.Error = err.Error()
		} else {
			result.UpdateAvailable = result.CurrentID != result.CandidateID
		}
		results = append(results, result)
	}
	m.mu.Lock()
	m.status.State = stateIdle
	m.status.Message = "Control-Plane-Prüfung abgeschlossen."
	m.status.Services = results
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *manager) update(targetImages map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	oldImages := map[string]string{}
	candidateImages := map[string]string{}
	candidateIDs := map[string]string{}
	for _, spec := range m.specs {
		_, oldID, err := m.inspectContainer(ctx, spec.Container)
		if err != nil {
			m.fail(err)
			return
		}
		oldImages[spec.Name] = oldID
		targetImage := targetImageFor(spec, targetImages)
		m.progress("Lade " + spec.DisplayName + ".")
		if !m.skipPull {
			pullOutput, err := m.run(ctx, "pull", targetImage)
			if err != nil {
				m.fail(err)
				return
			}
			if qualified, ok := digestFromPullOutput(targetImage, pullOutput); ok {
				targetImage = qualified
			} else {
				targetImage = digestQualify(ctx, m.run, targetImage)
			}
		}
		candidateID, err := m.inspectImage(ctx, targetImage)
		if err != nil {
			m.fail(err)
			return
		}
		// Refuses to apply what would actually be a downgrade - found live:
		// a resolution bug elsewhere (see digestFromPullOutput and
		// pickLatestReleaseImage) could target an older release than the
		// one currently running, and nothing here checked that before
		// swapping. Both images carry org.opencontainers.image.version
		// (every Dockerfile sets it from its own build), so this doesn't
		// depend on either reference still carrying a tag by this point -
		// oldID/candidateID work whether they're bare digests or not.
		// Silently skips the check (comparable=false) for anything that
		// isn't a RootGuard release version - a local :dev build, for
		// instance - rather than blocking a build lacking that label.
		if candidateVersion, err := m.imageVersion(ctx, candidateID); err == nil {
			if oldVersion, err := m.imageVersion(ctx, oldID); err == nil {
				if older, comparable := isOlderReleaseVersion(candidateVersion, oldVersion); comparable && older {
					m.fail(fmt.Errorf("%s: refusing to install %s - it looks older than the currently running %s (the resolved update target may be stale)", spec.DisplayName, candidateVersion, oldVersion))
					return
				}
			}
		}
		m.progress("Prüfe die Release-Attestierung von " + spec.DisplayName + ".")
		if err := m.attestationVerifier(ctx, spec.Name, targetImage); err != nil {
			m.fail(fmt.Errorf("attestation: %w", err))
			return
		}
		candidateImages[spec.Name] = targetImage
		candidateIDs[spec.Name] = candidateID
	}

	changed := false
	for name, candidateID := range candidateIDs {
		changed = changed || candidateID != oldImages[name]
	}
	if !changed {
		m.recordHistory(historyEntry{
			Outcome: "no_change", FromIDs: oldImages, ToIDs: candidateIDs,
			Message: "Core und WebApp verwenden bereits die aktuellen Images.", CreatedAt: time.Now().UTC(),
		})
		m.finish(candidateImages, candidateIDs, "Core und WebApp verwenden bereits die aktuellen Images.")
		return
	}

	m.progress("Ersetze Core und WebApp als gemeinsame Control Plane.")
	if err := m.writeOverride(candidateImages); err != nil {
		m.fail(err)
		return
	}
	updateErr := m.composeUp(ctx)
	if updateErr == nil {
		updateErr = m.verifyWithRetry(ctx, candidateIDs)
	}
	if updateErr == nil {
		m.recordHistory(historyEntry{
			Outcome: "success", FromIDs: oldImages, ToIDs: candidateIDs,
			Message: "Core und WebApp wurden aktualisiert und erfolgreich geprüft.", CreatedAt: time.Now().UTC(),
		})
		cleanup := m.cleanupAfterSuccess(ctx)
		m.attachCleanup(cleanup)
		m.finish(candidateImages, candidateIDs, "Core und WebApp wurden aktualisiert und erfolgreich geprüft.")
		return
	}

	m.progress("Prüfung fehlgeschlagen – beide Control-Plane-Images werden zurückgesetzt.")
	if rollbackErr := m.rollback(ctx, oldImages); rollbackErr != nil {
		m.recordHistory(historyEntry{
			Outcome: "failed", FromIDs: oldImages, ToIDs: candidateIDs,
			Message: fmt.Sprintf("Update und Rollback fehlgeschlagen: %v", rollbackErr), CreatedAt: time.Now().UTC(),
		})
		m.fail(fmt.Errorf("control-plane update failed: %v; rollback failed: %w", updateErr, rollbackErr))
		return
	}
	m.recordHistory(historyEntry{
		Outcome: "rolled_back", FromIDs: oldImages, ToIDs: candidateIDs,
		Message: "Das fehlerhafte Control-Plane-Update wurde sicher zurückgesetzt.", CreatedAt: time.Now().UTC(),
	})
	m.fail(fmt.Errorf("control-plane update failed and was rolled back safely: %w", updateErr))
}

func (m *manager) recordHistory(entry historyEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.History = append([]historyEntry{entry}, m.status.History...)
	if len(m.status.History) > 50 {
		m.status.History = m.status.History[:50]
	}
	_ = m.persistLocked()
}

func (m *manager) attachCleanup(cleanup cleanupResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.status.History) > 0 {
		m.status.History[0].Cleanup = cleanup
	}
	_ = m.persistLocked()
}

func (m *manager) cleanupAfterSuccess(ctx context.Context) cleanupResult {
	result := cleanupResult{}
	alreadyRemoved := map[string]bool{}
	m.mu.RLock()
	for _, entry := range m.status.History {
		for _, id := range entry.Cleanup.RemovedImages {
			alreadyRemoved[id] = true
		}
	}
	m.mu.RUnlock()
	for _, spec := range m.specs {
		seen := map[string]bool{}
		var ids []string
		m.mu.RLock()
		for index := len(m.status.History) - 1; index >= 0; index-- {
			entry := m.status.History[index]
			if entry.Outcome != "success" {
				continue
			}
			for _, id := range []string{entry.FromIDs[spec.Name], entry.ToIDs[spec.Name]} {
				if id != "" && !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
		m.mu.RUnlock()
		if len(ids) <= 2 {
			continue
		}
		for _, id := range ids[:len(ids)-2] {
			if alreadyRemoved[id] {
				continue
			}
			containers, err := m.run(ctx, "ps", "-a", "--filter", "ancestor="+id, "--format", "{{.ID}}")
			if err != nil || strings.TrimSpace(string(containers)) != "" {
				result.Skipped = append(result.Skipped, id)
				continue
			}
			if _, err := m.run(ctx, "image", "rm", id); err != nil {
				result.Skipped = append(result.Skipped, id)
			} else {
				result.RemovedImages = append(result.RemovedImages, id)
				alreadyRemoved[id] = true
			}
		}
	}
	volumes, err := m.run(ctx, "volume", "ls", "--quiet", "--filter", "label=io.rootguard.cleanup=true")
	if err != nil {
		result.Skipped = append(result.Skipped, "volume-scan")
		return result
	}
	for _, volume := range strings.Fields(string(volumes)) {
		containers, checkErr := m.run(ctx, "ps", "-a", "--filter", "volume="+volume, "--format", "{{.ID}}")
		if checkErr != nil || strings.TrimSpace(string(containers)) != "" {
			result.Skipped = append(result.Skipped, "volume:"+volume)
			continue
		}
		if _, removeErr := m.run(ctx, "volume", "rm", volume); removeErr != nil {
			result.Skipped = append(result.Skipped, "volume:"+volume)
		} else {
			result.RemovedVolumes = append(result.RemovedVolumes, volume)
		}
	}
	return result
}

func (m *manager) rollback(ctx context.Context, images map[string]string) error {
	if err := m.writeOverride(images); err != nil {
		return err
	}
	if err := m.composeUp(ctx); err != nil {
		return err
	}
	return m.verifyWithRetry(ctx, images)
}

func (m *manager) composeUp(ctx context.Context) error {
	override := filepath.Join(m.dataDir, "control-plane-images.yaml")
	_, err := m.run(ctx, "compose", "--project-name", m.projectName, "-f", m.composeFile, "-f", override,
		"up", "-d", "--no-deps", "core", "webapp")
	return err
}

func (m *manager) verifyWithRetry(ctx context.Context, expected map[string]string) error {
	var lastErr error
	for attempt := 0; attempt < m.verifyAttempts; attempt++ {
		lastErr = m.verify(expected)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return lastErr
}

func (m *manager) verify(expected map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for _, spec := range m.specs {
		_, currentID, err := m.inspectContainer(ctx, spec.Container)
		if err != nil {
			return err
		}
		if currentID != expected[spec.Name] {
			return fmt.Errorf("%s is running unexpected image %s", spec.DisplayName, currentID)
		}
		response, err := m.httpClient.Get(spec.HealthURL)
		if err != nil {
			return fmt.Errorf("%s health request: %w", spec.DisplayName, err)
		}
		_ = response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("%s health returned %s", spec.DisplayName, response.Status)
		}
	}
	return nil
}

func (m *manager) inspectContainer(ctx context.Context, container string) (string, string, error) {
	output, err := m.run(ctx, "inspect", "--format", "{{.Config.Image}}|{{.Image}}", container)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", fmt.Errorf("invalid image metadata for %s", container)
	}
	return parts[0], parts[1], nil
}

func (m *manager) inspectImage(ctx context.Context, image string) (string, error) {
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

// imageVersion reads an image's org.opencontainers.image.version label -
// every RootGuard Dockerfile sets it from its own build (see
// release-alpha.yml's build-args), so this works regardless of whether the
// image reference on hand is a tag, a digest, or a local image ID. Returns
// an empty string (not an error) for an image with no such label - a local
// :dev build, for instance - rather than failing the caller over a
// cosmetic gap.
func (m *manager) imageVersion(ctx context.Context, image string) (string, error) {
	output, err := m.run(ctx, "image", "inspect", "--format", `{{index .Config.Labels "org.opencontainers.image.version"}}`, image)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (m *manager) writeOverride(images map[string]string) error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}
	content := "services:\n"
	for _, spec := range m.specs {
		image := images[spec.Name]
		if image == "" {
			return fmt.Errorf("missing selected image for %s", spec.Name)
		}
		content += "  " + spec.Name + ":\n    image: " + fmt.Sprintf("%q", image) + "\n"
	}
	return writeAtomic(filepath.Join(m.dataDir, "control-plane-images.yaml"), []byte(content))
}

func (m *manager) progress(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.Message = message
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
}

func (m *manager) finish(images, ids map[string]string, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = stateIdle
	m.status.Message = message
	m.status.UpdatedAt = time.Now().UTC()
	for index := range m.status.Services {
		service := &m.status.Services[index]
		service.CurrentImage = images[service.Name]
		service.CurrentID = ids[service.Name]
		service.CandidateID = ids[service.Name]
		service.UpdateAvailable = false
		service.CheckedAt = time.Now().UTC()
		service.Error = ""
	}
	_ = m.persistLocked()
}

func (m *manager) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = stateFailed
	m.status.Message = err.Error()
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
}

func (m *manager) busyLocked() bool {
	return m.status.State == stateChecking || m.status.State == stateUpdating
}

func (m *manager) persist() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.persistLocked()
}

func (m *manager) persistLocked() error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(m.dataDir, "status.json"), data)
}

func (m *manager) load() {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "status.json"))
	if err == nil {
		_ = json.Unmarshal(data, &m.status)
	}
}

func (m *manager) reconcile() {
	previous := map[string]serviceStatus{}
	for _, service := range m.status.Services {
		previous[service.Name] = service
	}
	m.status.Services = nil
	for _, spec := range m.specs {
		service := previous[spec.Name]
		service.Name, service.DisplayName, service.TargetImage = spec.Name, spec.DisplayName, spec.TargetImage
		m.status.Services = append(m.status.Services, service)
	}
}

func cloneStatus(value status) status {
	result := value
	result.Services = append([]serviceStatus(nil), value.Services...)
	result.History = append([]historyEntry(nil), value.History...)
	return result
}
