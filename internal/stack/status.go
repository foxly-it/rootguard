package stack

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"sort"
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
}

type StackStatus struct {
	AdGuard ContainerInfo `json:"adguard"`
	Unbound ContainerInfo `json:"unbound"`
}

func CheckStackStatus() StackStatus {

	return StackStatus{
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
			Image string `json:"Image"`
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

	return ContainerInfo{
		Exists: true, Running: data[0].State.Running, Status: data[0].State.Status,
		Health: health, Image: data[0].Config.Image, ImageID: data[0].Image,
		StartedAt: data[0].State.StartedAt, RestartCount: data[0].RestartCount,
		Ports: ports,
	}, nil
}
