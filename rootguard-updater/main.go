package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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

// targetImageFor returns targetImages[spec.Name] when set, falling back to
// spec's own static TargetImage pin otherwise.
func targetImageFor(spec serviceSpec, targetImages map[string]string) string {
	if image, ok := targetImages[spec.Name]; ok && image != "" {
		return image
	}
	return spec.TargetImage
}

// digestQualify turns a bare "repo:tag" override (as resolved by Core's
// live release discovery) into an immutable "repo@sha256:..." one, using
// the digest the image was just pulled at. Cosign attestation verification
// requires an explicit @sha256: reference and reports "not_applicable"
// without one; an already-qualified (static pin) target passes through
// unchanged, and a lookup failure falls back to the original reference
// rather than failing the check/update over a cosmetic gap.
func digestQualify(ctx context.Context, run runner, image string) string {
	if strings.Contains(image, "@sha256:") {
		return image
	}
	repo, _, ok := strings.Cut(image, ":")
	if !ok {
		return image
	}
	output, err := run(ctx, "image", "inspect", "--format", "{{range .RepoDigests}}{{.}}|{{end}}", image)
	if err != nil {
		return image
	}
	for _, digestRef := range strings.Split(strings.TrimSpace(string(output)), "|") {
		if strings.HasPrefix(digestRef, repo+"@") {
			return digestRef
		}
	}
	return image
}

// digestFromPullOutput extracts the digest `docker pull` itself reports for
// the image it just pulled ("Digest: sha256:...", printed once pulling
// finishes) - authoritative for "what was just pulled" in a way
// digestQualify's RepoDigests lookup above isn't: RepoDigests belongs to
// the local image object as a whole, so if a repository ever has more than
// one digest recorded against a matching local image (observed for real in
// CI - a repository whose tag had recently moved to a new release still
// carried the previous release's digest in its RepoDigests list), the
// first-match loop there can silently return the *stale* one, making an
// available update look like "already current." Preferred over
// digestQualify when it can parse pull's own output; digestQualify stays
// as the fallback for an already-qualified static pin (whose "pull" prints
// the same digest back anyway) or an unexpected output format.
func digestFromPullOutput(image string, output []byte) (string, bool) {
	repo, _, ok := strings.Cut(image, ":")
	if !ok {
		return "", false
	}
	for _, line := range strings.Split(string(output), "\n") {
		digest, ok := strings.CutPrefix(strings.TrimSpace(line), "Digest: ")
		if ok && strings.HasPrefix(digest, "sha256:") {
			return repo + "@" + digest, true
		}
	}
	return "", false
}

// decodeTargetOverrides reads an optional {"target_images": {...}} JSON
// body; a missing/empty body is not an error and yields no overrides.
func decodeTargetOverrides(body io.Reader) (map[string]string, error) {
	var payload struct {
		TargetImages map[string]string `json:"target_images"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	return payload.TargetImages, nil
}

type runner func(context.Context, ...string) ([]byte, error)

type manager struct {
	mu             sync.RWMutex
	status         status
	specs          []serviceSpec
	dataDir        string
	composeFile    string
	projectName    string
	run            runner
	httpClient     *http.Client
	skipPull       bool
	verifyAttempts int
}

func main() {
	token := os.Getenv("ROOTGUARD_UPDATER_TOKEN")
	if token == "" {
		log.Fatal("ROOTGUARD_UPDATER_TOKEN must be set")
	}
	requireSecretStrength("ROOTGUARD_UPDATER_TOKEN", token, minTokenLength)
	if err := prepareSessionVolume(envOrDefault("ROOTGUARD_SESSION_DIR", "/var/lib/rootguard-sessions")); err != nil {
		log.Fatalf("prepare WebApp session volume: %v", err)
	}
	manager := newManager(
		envOrDefault("ROOTGUARD_UPDATER_DATA_DIR", "/var/lib/rootguard/control-plane-updater"),
		envOrDefault("ROOTGUARD_COMPOSE_FILE", "/opt/rootguard/compose.yaml"),
		envOrDefault("ROOTGUARD_COMPOSE_PROJECT", "rootguard"),
		[]serviceSpec{
			{
				Name: "core", DisplayName: "RootGuard Core", Container: "rootguard-core",
				TargetImage: envOrDefault("ROOTGUARD_CORE_UPDATE_IMAGE", "ghcr.io/foxly-it/rootguard-core:latest"),
				HealthURL:   "http://core:8081/api/health",
			},
			{
				Name: "webapp", DisplayName: "RootGuard WebApp", Container: "rootguard-webapp",
				TargetImage: envOrDefault("ROOTGUARD_WEBAPP_UPDATE_IMAGE", "ghcr.io/foxly-it/rootguard-webapp:latest"),
				HealthURL:   "http://webapp:8080/health",
			},
		},
		runDocker,
	)
	manager.skipPull = strings.EqualFold(os.Getenv("ROOTGUARD_UPDATER_SKIP_PULL"), "true")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/control-plane/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, manager.Status())
	})
	mux.HandleFunc("POST /api/control-plane/check", func(w http.ResponseWriter, r *http.Request) {
		overrides, err := decodeTargetOverrides(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		next, err := manager.StartCheck(overrides)
		if errors.Is(err, errBusy) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, next)
	})
	mux.HandleFunc("POST /api/control-plane/update", func(w http.ResponseWriter, r *http.Request) {
		overrides, err := decodeTargetOverrides(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		next, err := manager.StartUpdate(overrides)
		if errors.Is(err, errBusy) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, next)
	})

	server := &http.Server{
		Addr:              ":8082",
		Handler:           requireBearer(token, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Print("RootGuard control-plane updater listening on :8082")
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func prepareSessionVolume(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	if err := os.Chown(path, 65532, 65532); err != nil {
		return err
	}
	return os.Chmod(path, 0700)
}

var errBusy = errors.New("an updater operation is already running")

func newManager(dataDir, composeFile, projectName string, specs []serviceSpec, run runner) *manager {
	m := &manager{
		dataDir: dataDir, composeFile: composeFile, projectName: projectName,
		specs: specs, run: run, httpClient: &http.Client{Timeout: 8 * time.Second},
		verifyAttempts: 45,
		status:         status{State: stateIdle, Message: "Noch keine Control-Plane-Prüfung durchgeführt.", UpdatedAt: time.Now().UTC()},
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
	return m.start(stateChecking, "Core- und WebApp-Images werden geprüft.", func() { m.check(targetImages) })
}

func (m *manager) StartUpdate(targetImages map[string]string) (status, error) {
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

func requireBearer(token string, next http.Handler) http.Handler {
	expected := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		actual := sha256.Sum256([]byte(provided))
		if provided == "" || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeAtomic(path string, data []byte) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func runDocker(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("docker %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

const (
	minTokenLength = 32
	// placeholderPrefix matches every secret value in .env.release.example
	// ("replace-with-a-long-random-token", ...) - compose.release.yaml
	// derives ROOTGUARD_UPDATER_TOKEN from that same ROOTGUARD_API_TOKEN
	// value, so an unedited placeholder ends up here too.
	placeholderPrefix = "replace-with-"
)

// requireSecretStrength exits the process if value is too short or is
// still an unedited .env.release.example placeholder.
func requireSecretStrength(name, value string, minLength int) {
	if strings.HasPrefix(strings.ToLower(value), placeholderPrefix) {
		log.Fatalf("%s is still set to its .env.release.example placeholder value - set a real secret", name)
	}
	if len(value) < minLength {
		log.Fatalf("%s must be at least %d characters", name, minLength)
	}
}
