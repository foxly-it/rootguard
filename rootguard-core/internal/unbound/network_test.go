package unbound

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNetworkCapabilitiesProbeBothFamilies(t *testing.T) {
	manager := NewManager(t.TempDir(), "/etc/unbound/unbound.d", "rootguard-unbound")
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		if strings.Contains(command, " -4 ") {
			return []byte("a.root-servers.net.\n"), nil
		}
		return []byte("network unreachable"), errors.New("exit 9")
	}
	result := manager.NetworkCapabilities(context.Background())
	if !result.IPv4Available || result.IPv6Available {
		t.Fatalf("unexpected capabilities: %+v", result)
	}
	if !strings.Contains(result.IPv6Detail, "unreachable") {
		t.Fatalf("missing bounded probe detail: %+v", result)
	}
}

func TestApplyRejectsUnavailableIPv6Mode(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.NetworkMode = networkModeDual
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("network unreachable"), errors.New("exit 9")
	}
	if err := manager.Apply(context.Background(), settings); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected unavailable IPv6 rejection, got %v", err)
	}
}
