package docker

import (
	"os/exec"
)

type DockerStatus struct {
	Installed bool `json:"installed"`
	Running   bool `json:"running"`
	Compose   bool `json:"compose"`
}

func CheckDockerStatus() DockerStatus {

	status := DockerStatus{}

	// Prüfen ob docker binary existiert
	if _, err := exec.LookPath("docker"); err == nil {
		status.Installed = true
	}

	// Prüfen ob Docker Daemon läuft
	if status.Installed {
		cmd := exec.Command("docker", "info")
		if err := cmd.Run(); err == nil {
			status.Running = true
		}
	}

	// Prüfen ob docker compose verfügbar ist
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		status.Compose = true
	}

	return status
}
