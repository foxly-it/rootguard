package system

import (
	"os/exec"
	"strings"
)

func IsServiceActive(name string) bool {

	cmd := exec.Command("systemctl", "is-active", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	status := strings.TrimSpace(string(output))
	return status == "active"
}

func RestartService(name string) error {
	cmd := exec.Command("systemctl", "restart", name)
	return cmd.Run()
}

func StopService(name string) error {
	cmd := exec.Command("systemctl", "stop", name)
	return cmd.Run()
}

func StartService(name string) error {
	cmd := exec.Command("systemctl", "start", name)
	return cmd.Run()
}
