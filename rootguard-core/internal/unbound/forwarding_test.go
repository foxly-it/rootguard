package unbound

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCheckForwardTargetsPreservesZoneAndServerOrder(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "docker" {
			return nil, errors.New("unexpected command: " + name)
		}
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "@192.0.2.53 corp.example. SOA"):
			return []byte("status: NOERROR\ncorp.example. 300 IN SOA ns.corp.example. hostmaster.corp.example. 1 60 60 60 60"), nil
		case strings.Contains(joined, "@2001:db8::53 corp.example. SOA"):
			return []byte("status: NXDOMAIN\nexample. 300 IN SOA ns.example. hostmaster.example. 1 60 60 60 60"), nil
		case strings.Contains(joined, "@192.0.2.54 lab.example. SOA"):
			return []byte("status: REFUSED"), nil
		default:
			return nil, errors.New("unexpected probe")
		}
	}
	zones := []ForwardZone{
		{Name: "corp.example.", Servers: []string{"192.0.2.53", "2001:db8::53"}},
		{Name: "lab.example.", Servers: []string{"192.0.2.54"}},
	}
	checks, err := manager.CheckForwardTargets(context.Background(), zones)
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 3 {
		t.Fatalf("expected three checks, got %+v", checks)
	}
	for index, expected := range []struct {
		zone      string
		address   string
		reachable bool
	}{
		{zone: "corp.example.", address: "192.0.2.53", reachable: true},
		{zone: "corp.example.", address: "2001:db8::53"},
		{zone: "lab.example.", address: "192.0.2.54"},
	} {
		if checks[index].Zone != expected.zone || checks[index].Address != expected.address || checks[index].Reachable != expected.reachable {
			t.Fatalf("unexpected check at index %d: %+v", index, checks[index])
		}
	}
}

func TestCheckForwardTargetsRequiresZoneSOA(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("status: NOERROR"), nil
	}
	checks, err := manager.CheckForwardTargets(context.Background(), []ForwardZone{{
		Name:    "corp.example.",
		Servers: []string{"192.0.2.53"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Reachable || !strings.Contains(checks[0].Detail, "did not return an SOA") {
		t.Fatalf("unexpected incomplete zone check: %+v", checks)
	}
}

func TestCheckForwardTargetsReportsStableStatusError(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(";; Got answer:\n;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 1234\n;; flags: qr aa"), nil
	}
	checks, err := manager.CheckForwardTargets(context.Background(), []ForwardZone{{
		Name:    "empty.example.",
		Servers: []string{"192.0.2.53"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	const expected = "DNS server did not return an SOA record for the forwarding zone (status: NXDOMAIN)"
	if len(checks) != 1 || checks[0].Reachable || checks[0].Detail != expected {
		t.Fatalf("unexpected rejected zone check: %+v", checks)
	}
}

func TestCheckForwardTargetsRedactsIncompleteResponse(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("unexpected resolver response that must not reach the API"), nil
	}
	checks, err := manager.CheckForwardTargets(context.Background(), []ForwardZone{{
		Name:    "empty.example.",
		Servers: []string{"192.0.2.53"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Reachable ||
		!strings.Contains(checks[0].Detail, "did not return an SOA") ||
		strings.Contains(checks[0].Detail, "must not reach") {
		t.Fatalf("unexpected incomplete response check: %+v", checks)
	}
}

func TestCheckForwardTargetsReportsUnreachableServer(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("communications error: timed out"), errors.New("exit status 9")
	}
	checks, err := manager.CheckForwardTargets(context.Background(), []ForwardZone{{
		Name:    "corp.example.",
		Servers: []string{"192.0.2.53"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0].Reachable || !strings.Contains(checks[0].Detail, "timed out") {
		t.Fatalf("unexpected failed check: %+v", checks)
	}
}

func TestCheckForwardTargetsRejectsInvalidRequest(t *testing.T) {
	manager := newTestManager(t)
	if _, err := manager.CheckForwardTargets(context.Background(), []ForwardZone{{
		Name:    ".",
		Servers: []string{"127.0.0.1"},
	}}); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected invalid settings, got %v", err)
	}
}
