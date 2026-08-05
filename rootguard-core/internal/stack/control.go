package stack

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

var (
	ErrUnknownService = errors.New("unknown service")
	ErrUnknownAction  = errors.New("unknown action")
)

var serviceContainers = map[string]string{
	"adguard": "rootguard-adguard",
	"unbound": "rootguard-unbound",
}

var execCommandContext = exec.CommandContext

func ControlService(ctx context.Context, service, action string) error {
	container, ok := serviceContainers[service]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownService, service)
	}
	if action != "start" && action != "stop" && action != "restart" {
		return fmt.Errorf("%w: %s", ErrUnknownAction, action)
	}

	output, err := exec.CommandContext(ctx, "docker", action, container).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s %s: %w: %s", action, container, err, output)
	}
	return nil
}
