// Package dockercli holds the one production way rootguard-core's internal
// packages shell out to the docker CLI - found in review: backupexport,
// backuprestore, installer, and updater each carried a byte-identical
// CommandRunner type plus their own runDocker() (two of the four wrapping
// the error with the command and output, two returning CombinedOutput's
// result bare), duplicated purely because each package needs its own
// injectable default for tests, not because the implementation itself ever
// differed on purpose.
//
// Run is the wrapping variant: installer's classifyDeploymentError and
// runComposeUp/probeHostPortBusy's port-bind-conflict detection both match
// against err.Error(), so the command and its output need to already be in
// there, not just in the separately-returned output bytes. Callers that
// build their own, more specific message on top (backupexport,
// backuprestore) get the same output text twice - see their own call
// sites' comments for why that's fine there.
package dockercli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CommandRunner abstracts a single docker CLI invocation so callers can
// inject a fake one in tests instead of shelling out for real.
type CommandRunner func(context.Context, ...string) ([]byte, error)

// Run shells out to the docker CLI and wraps a failure with the exact
// arguments and combined output, so err.Error() alone already carries
// enough detail for both a human reading a surfaced message and code that
// classifies failures by matching against it.
func Run(ctx context.Context, arguments ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, "docker", arguments...).CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("docker %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
