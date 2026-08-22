package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

type ContainerInfo struct {
	Exists       bool     `json:"exists"`
	Running      bool     `json:"running"`
	Status       string   `json:"status"`
	Health       string   `json:"health"`
	Image        string   `json:"image,omitempty"`
	ImageID      string   `json:"image_id,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	RestartCount int      `json:"restart_count"`
	Ports        []string `json:"ports,omitempty"`
	Version      string   `json:"version,omitempty"`
	Revision     string   `json:"revision,omitempty"`
	Created      string   `json:"created,omitempty"`
	Source       string   `json:"source,omitempty"`
	Immutable    bool     `json:"immutable"`
	Metadata     string   `json:"metadata"`
	Attestation  string   `json:"attestation"`
	AttestedAt   string   `json:"attested_at,omitempty"`
}

func CheckStackAttestations(ctx context.Context, status *StackStatus) {
	type target struct {
		name string
		info *ContainerInfo
	}
	// AdGuard has no RootGuard signing policy (third-party image) and stays
	// permanently not_applicable - core, webapp, updater, and unbound are
	// all published by the same release-alpha.yml matrix build and do have
	// real policies in attestationPolicies, so they must actually be
	// checked rather than assumed not_applicable.
	targets := []target{
		{"core", &status.Core},
		{"webapp", &status.WebApp},
		{"updater", &status.Updater},
		{"unbound", &status.Unbound},
	}
	var wait sync.WaitGroup
	for _, item := range targets {
		wait.Add(1)
		go func(item target) {
			defer wait.Done()
			item.info.Attestation, item.info.AttestedAt = verifyReleaseAttestation(ctx, item.name, item.info.Image)
		}(item)
	}
	wait.Wait()
	status.AdGuard.Attestation = "not_applicable"
}

type StackStatus struct {
	Core    ContainerInfo `json:"core"`
	WebApp  ContainerInfo `json:"webapp"`
	Updater ContainerInfo `json:"updater"`
	AdGuard ContainerInfo `json:"adguard"`
	Unbound ContainerInfo `json:"unbound"`
}

func CheckStackStatus() StackStatus {

	return StackStatus{
		Core:    inspectContainer("rootguard-core"),
		WebApp:  inspectContainer("rootguard-webapp"),
		Updater: inspectContainer("rootguard-updater"),
		AdGuard: inspectContainer("rootguard-adguard"),
		Unbound: inspectContainer("rootguard-unbound"),
	}
}

func inspectContainer(name string) ContainerInfo {

	cmd := exec.Command("docker", "inspect", name)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return ContainerInfo{
			Exists:  false,
			Running: false,
			Status:  "missing",
			Health:  "unknown",
		}
	}

	info, err := decodeContainerInspect(out.Bytes())
	if err != nil {
		return ContainerInfo{Status: "unknown", Health: "unknown"}
	}
	return info
}

func decodeContainerInspect(payload []byte) (ContainerInfo, error) {
	var data []struct {
		State struct {
			Running   bool   `json:"Running"`
			Status    string `json:"Status"`
			StartedAt string `json:"StartedAt"`
			Health    *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
		Config struct {
			Image  string            `json:"Image"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		Image           string `json:"Image"`
		RestartCount    int    `json:"RestartCount"`
		NetworkSettings struct {
			Ports map[string]json.RawMessage `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return ContainerInfo{}, err
	}

	if len(data) == 0 {
		return ContainerInfo{Status: "missing", Health: "unknown"}, nil
	}

	var ports []string
	for port, bindings := range data[0].NetworkSettings.Ports {
		if len(bindings) > 0 && string(bindings) != "null" && string(bindings) != "[]" {
			ports = append(ports, port)
		}
	}
	sort.Strings(ports)
	health := "not_configured"
	if data[0].State.Health != nil && data[0].State.Health.Status != "" {
		health = data[0].State.Health.Status
	}

	labels := data[0].Config.Labels
	version := labels["org.opencontainers.image.version"]
	revision := labels["org.opencontainers.image.revision"]
	created := labels["org.opencontainers.image.created"]
	source := labels["org.opencontainers.image.source"]
	metadata := "unavailable"
	available := 0
	for _, value := range []string{version, revision, created, source} {
		if value != "" {
			available++
		}
	}
	if available == 4 {
		metadata = "complete"
	} else if available > 0 {
		metadata = "partial"
	}

	return ContainerInfo{
		Exists: true, Running: data[0].State.Running, Status: data[0].State.Status,
		Health: health, Image: data[0].Config.Image, ImageID: data[0].Image,
		StartedAt: data[0].State.StartedAt, RestartCount: data[0].RestartCount,
		Ports: ports, Version: version, Revision: revision, Created: created,
		Source: source, Immutable: strings.Contains(data[0].Config.Image, "@sha256:"),
		Metadata: metadata,
	}, nil
}
