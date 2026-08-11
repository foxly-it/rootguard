package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CleanupResource struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	EstimatedBytes int64  `json:"estimated_bytes"`
}

type CleanupPreview struct {
	Resources      []CleanupResource `json:"resources"`
	Skipped        []string          `json:"skipped,omitempty"`
	EstimatedBytes int64             `json:"estimated_bytes"`
}

func (m *Manager) PreviewCleanup(ctx context.Context) (CleanupPreview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.busyLocked() {
		return CleanupPreview{}, ErrBusy
	}
	return m.cleanupPreviewLocked(ctx, "")
}

func (m *Manager) RunManualCleanup(ctx context.Context) (CleanupResult, error) {
	m.mu.Lock()
	if m.busyLocked() {
		m.mu.Unlock()
		return CleanupResult{}, ErrBusy
	}
	m.status.State = StateUpdating
	m.status.ActiveService = ""
	m.status.Message = "Manuelle Docker-Bereinigung wird sicher ausgeführt."
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()

	result, err := m.executeCleanup(ctx, "")
	if err != nil {
		result.Skipped = append(result.Skipped, "cleanup-preview:"+err.Error())
	}
	m.mu.Lock()
	m.status.State = StateIdle
	m.status.Message = "Manuelle Docker-Bereinigung abgeschlossen."
	outcome, message := "cleanup", "Manuelle Docker-Bereinigung abgeschlossen."
	if err != nil {
		m.status.State = StateFailed
		m.status.Message = "Manuelle Docker-Bereinigung fehlgeschlagen: " + err.Error()
		outcome, message = "failed", m.status.Message
	}
	m.status.History = append([]HistoryEntry{{
		Service: "cleanup", Outcome: outcome, Message: message, Cleanup: result, CreatedAt: time.Now().UTC(),
	}}, m.status.History...)
	if len(m.status.History) > 50 {
		m.status.History = m.status.History[:50]
	}
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()
	return result, err
}

func (m *Manager) cleanupAfterSuccess(ctx context.Context, service string) CleanupResult {
	result, err := m.executeCleanup(ctx, service)
	if err != nil {
		result.Skipped = append(result.Skipped, "cleanup-preview:"+err.Error())
	}
	return result
}

func (m *Manager) executeCleanup(ctx context.Context, service string) (CleanupResult, error) {
	m.mu.RLock()
	preview, err := m.cleanupPreviewLocked(ctx, service)
	m.mu.RUnlock()
	if err != nil {
		return CleanupResult{}, err
	}
	result := CleanupResult{Skipped: append([]string(nil), preview.Skipped...)}
	for _, resource := range preview.Resources {
		var arguments []string
		switch resource.Kind {
		case "image":
			arguments = []string{"image", "rm", resource.ID}
		case "volume":
			arguments = []string{"volume", "rm", resource.ID}
		default:
			continue
		}
		if _, removeErr := m.run(ctx, arguments...); removeErr != nil {
			result.Skipped = append(result.Skipped, resource.Kind+":"+resource.ID)
		} else if resource.Kind == "image" {
			result.RemovedImages = append(result.RemovedImages, resource.ID)
		} else {
			result.RemovedVolumes = append(result.RemovedVolumes, resource.ID)
		}
	}
	return result, nil
}

func (m *Manager) cleanupPreviewLocked(ctx context.Context, service string) (CleanupPreview, error) {
	preview := CleanupPreview{Resources: []CleanupResource{}}
	removed := map[string]bool{}
	byService := map[string][]string{}
	seen := map[string]map[string]bool{}
	for _, entry := range m.status.History {
		for _, id := range entry.Cleanup.RemovedImages {
			removed[id] = true
		}
	}
	for index := len(m.status.History) - 1; index >= 0; index-- {
		entry := m.status.History[index]
		if entry.Outcome != "success" || entry.Service == "" {
			continue
		}
		if seen[entry.Service] == nil {
			seen[entry.Service] = map[string]bool{}
		}
		for _, id := range []string{entry.FromID, entry.ToID} {
			if id != "" && !seen[entry.Service][id] {
				seen[entry.Service][id] = true
				byService[entry.Service] = append(byService[entry.Service], id)
			}
		}
	}
	candidates := []string{}
	candidateSeen := map[string]bool{}
	for candidateService, ids := range byService {
		if service != "" && candidateService != service {
			continue
		}
		if len(ids) <= 2 {
			continue
		}
		for _, id := range ids[:len(ids)-2] {
			if removed[id] {
				continue
			}
			if !m.unused(ctx, "ancestor="+id, "image:"+id, &preview) {
				continue
			}
			if !candidateSeen[id] {
				candidateSeen[id] = true
				candidates = append(candidates, id)
			}
		}
	}
	sort.Strings(candidates)
	if len(candidates) > 0 {
		sizes, err := m.dockerUsageSizes(ctx, "Images")
		if err != nil {
			return CleanupPreview{}, err
		}
		for _, id := range candidates {
			size, ok := sizes[id]
			if !ok {
				preview.Skipped = append(preview.Skipped, "image-size:"+id)
				continue
			}
			preview.add(CleanupResource{Kind: "image", ID: id, EstimatedBytes: size})
		}
	}

	output, err := m.run(ctx, "volume", "ls", "--quiet", "--filter", "label=io.rootguard.cleanup=true")
	if err != nil {
		return CleanupPreview{}, fmt.Errorf("scan cleanup volumes: %w", err)
	}
	volumes := strings.Fields(string(output))
	sort.Strings(volumes)
	if len(volumes) == 0 {
		return preview, nil
	}
	sizes, err := m.dockerUsageSizes(ctx, "Volumes")
	if err != nil {
		return CleanupPreview{}, err
	}
	for _, volume := range volumes {
		if !m.unused(ctx, "volume="+volume, "volume:"+volume, &preview) {
			continue
		}
		size, ok := sizes[volume]
		if !ok {
			preview.Skipped = append(preview.Skipped, "volume-size:"+volume)
			continue
		}
		preview.add(CleanupResource{Kind: "volume", ID: volume, EstimatedBytes: size})
	}
	return preview, nil
}

func (m *Manager) unused(ctx context.Context, filter, label string, preview *CleanupPreview) bool {
	output, err := m.run(ctx, "ps", "-a", "--filter", filter, "--format", "{{.ID}}")
	if err != nil || strings.TrimSpace(string(output)) != "" {
		preview.Skipped = append(preview.Skipped, label)
		return false
	}
	return true
}

func (m *Manager) dockerUsageSizes(ctx context.Context, collection string) (map[string]int64, error) {
	output, err := m.run(ctx, "system", "df", "-v", "--format", "{{json ."+collection+"}}")
	if err != nil {
		return nil, fmt.Errorf("estimate cleanup %s: %w", strings.ToLower(collection), err)
	}
	var resources []struct {
		ID         string `json:"ID"`
		Name       string `json:"Name"`
		Size       string `json:"Size"`
		UniqueSize string `json:"UniqueSize"`
	}
	if err := json.Unmarshal(output, &resources); err != nil {
		return nil, fmt.Errorf("decode cleanup %s sizes: %w", strings.ToLower(collection), err)
	}
	sizes := make(map[string]int64, len(resources))
	for _, resource := range resources {
		key, value := resource.Name, resource.Size
		if collection == "Images" {
			key, value = resource.ID, resource.UniqueSize
		}
		if size, ok := parseDockerSize(value); ok {
			sizes[key] = size
		}
	}
	return sizes, nil
}

func parseDockerSize(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	units := []struct {
		suffix string
		factor float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit.suffix) {
			continue
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, unit.suffix)), 64)
		return int64(number * unit.factor), err == nil && number >= 0
	}
	return 0, false
}

func (preview *CleanupPreview) add(resource CleanupResource) {
	preview.Resources = append(preview.Resources, resource)
	preview.EstimatedBytes += resource.EstimatedBytes
}
