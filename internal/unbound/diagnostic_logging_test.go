package unbound

import (
	"context"
	"reflect"
	"testing"
)

func TestTemporaryDiagnosticLoggingStartsAndRestoresSafeLevel(t *testing.T) {
	manager := NewManager(t.TempDir(), "/etc/unbound/unbound.d", "rootguard-unbound")
	var commands [][]string
	manager.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		commands = append(commands, append([]string{name}, args...))
		return nil, nil
	}

	status, err := manager.StartDiagnosticLogging(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Active || status.Level != 2 || status.ExpiresAt == nil {
		t.Fatalf("unexpected active status: %+v", status)
	}
	status, err = manager.StopDiagnosticLogging(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Active || status.Level != 1 || status.ExpiresAt != nil {
		t.Fatalf("unexpected stopped status: %+v", status)
	}

	want := [][]string{
		{"docker", "exec", "rootguard-unbound", "unbound-control", "verbosity", "2"},
		{"docker", "exec", "rootguard-unbound", "unbound-control", "verbosity", "1"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("unexpected diagnostic logging commands: %#v", commands)
	}
}
