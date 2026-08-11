package backuprestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/foxly-it/rootguard-core/internal/installer"
)

var (
	ErrInvalidBackup = errors.New("invalid RootGuard backup")
	ErrRestoreFailed = errors.New("RootGuard backup restore failed")
)

type CommandRunner func(context.Context, ...string) ([]byte, error)

type Options struct {
	DataDir        string
	UnboundDir     string
	AdGuardDir     string
	AdGuardAuthDir string
	Installer      *installer.Manager
	Run            CommandRunner
}

type Manager struct {
	dataDir, unboundDir, adGuardDir, adGuardAuthDir string
	installer                                       *installer.Manager
	run                                             CommandRunner
}

type RestoreRequest struct {
	Passphrase string
	Config     installer.Config
	Archive    io.Reader
}

type PreviewResult struct {
	Preview
	Preflight installer.Preflight `json:"preflight"`
}

func New(options Options) *Manager {
	if options.Run == nil {
		options.Run = runDocker
	}
	return &Manager{dataDir: options.DataDir, unboundDir: options.UnboundDir, adGuardDir: options.AdGuardDir,
		adGuardAuthDir: options.AdGuardAuthDir, installer: options.Installer, run: options.Run}
}

func (m *Manager) Preview(ctx context.Context, passphrase string, archive io.Reader, target *installer.Config) (PreviewResult, error) {
	stage, preview, err := Extract(m.dataDir, passphrase, archive)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer os.RemoveAll(stage)
	config := preview.Config
	if target != nil {
		config = *target
	}
	return PreviewResult{Preview: preview, Preflight: m.installer.RestorePreflight(ctx, config)}, nil
}

func (m *Manager) Restore(ctx context.Context, request RestoreRequest) (installer.Status, error) {
	stage, preview, err := Extract(m.dataDir, request.Passphrase, request.Archive)
	if err != nil {
		return installer.Status{}, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer os.RemoveAll(stage)
	config := request.Config
	if config.DNSBindAddress == "" {
		config = preview.Config
	}
	local := []struct{ source, target string }{
		{filepath.Join(stage, "rootguard", "unbound"), m.unboundDir},
		{filepath.Join(stage, "rootguard", "adguard"), m.adGuardDir},
		{filepath.Join(stage, "rootguard", "adguard-auth"), m.adGuardAuthDir},
	}
	rollback, err := os.MkdirTemp(m.dataDir, ".rollback-")
	if err != nil {
		return installer.Status{}, err
	}
	defer os.RemoveAll(rollback)
	for index, item := range local {
		backup := filepath.Join(rollback, fmt.Sprintf("%d", index))
		if err := os.MkdirAll(backup, 0700); err != nil {
			return installer.Status{}, err
		}
		if _, err := os.Stat(item.target); err == nil {
			if err := copyDirectory(item.target, backup); err != nil {
				return installer.Status{}, fmt.Errorf("stage restore rollback: %w", err)
			}
		}
	}
	status, restoreErr := m.installer.Restore(ctx, config, func(ctx context.Context) error {
		for _, item := range local {
			if err := replaceDirectory(item.source, item.target); err != nil {
				return err
			}
		}
		for _, item := range []struct{ source, container, target string }{
			{filepath.Join(stage, "services", "adguard", "config"), "rootguard-adguard", "/opt/adguardhome/conf"},
			{filepath.Join(stage, "services", "adguard", "work"), "rootguard-adguard", "/opt/adguardhome/work"},
			{filepath.Join(stage, "services", "unbound", "state"), "rootguard-unbound", "/var/lib/unbound"},
		} {
			if _, err := os.Stat(item.source); os.IsNotExist(err) {
				continue
			}
			output, err := m.run(ctx, "cp", item.source+string(os.PathSeparator)+".", item.container+":"+item.target)
			if err != nil {
				return fmt.Errorf("restore %s: %w: %s", item.target, err, strings.TrimSpace(string(output)))
			}
		}
		if err := m.normalizeUnboundOwnership(ctx); err != nil {
			return err
		}
		return nil
	})
	if restoreErr != nil {
		for index, item := range local {
			if err := replaceDirectory(filepath.Join(rollback, fmt.Sprintf("%d", index)), item.target); err != nil {
				return status, fmt.Errorf("%w; roll back local restore data: %v", restoreErr, err)
			}
		}
	}
	if restoreErr != nil && !errors.Is(restoreErr, installer.ErrNotClean) {
		return status, fmt.Errorf("%w: %v", ErrRestoreFailed, restoreErr)
	}
	return status, restoreErr
}

func (m *Manager) normalizeUnboundOwnership(ctx context.Context) error {
	output, err := m.run(ctx, "inspect", "--format", "{{.Config.Image}}", "rootguard-unbound")
	if err != nil {
		return fmt.Errorf("inspect restored Unbound image: %w", err)
	}
	image := strings.TrimSpace(string(output))
	if image == "" {
		return fmt.Errorf("inspect restored Unbound image: empty image")
	}
	for _, volume := range []struct{ name, path string }{
		{"rootguard-unbound-config", "/etc/unbound/unbound.d"},
		{"rootguard-unbound-state", "/var/lib/unbound"},
	} {
		output, err := m.run(ctx, "run", "--rm", "--network", "none", "--user", "0:0", "--read-only", "--cap-drop", "ALL", "--cap-add", "CHOWN", "--security-opt", "no-new-privileges:true", "--volume", volume.name+":"+volume.path, "--entrypoint", "/usr/bin/chown", image, "--recursive", "100:101", volume.path)
		if err != nil {
			return fmt.Errorf("normalize %s ownership: %w: %s", volume.name, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func replaceDirectory(source, target string) error {
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(target, 0700); err != nil {
		return err
	}
	if err := clearDirectory(target); err != nil {
		return err
	}
	return copyDirectory(source, target)
}

func clearDirectory(target string) error {
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse existing symlink in restore target %q", entry.Name())
		}
		if err := os.RemoveAll(filepath.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyDirectory(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refuse symlink while copying %q", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		return errorsJoin(copyErr, input.Close(), output.Close())
	})
}

func errorsJoin(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func runDocker(ctx context.Context, arguments ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", arguments...).CombinedOutput()
}
