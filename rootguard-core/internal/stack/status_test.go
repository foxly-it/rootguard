package stack

import (
	"context"
	"testing"
)

// TestCheckStackAttestationsCoversCoreWebappUpdaterUnbound guards against
// the actual live-serving wiring drifting away from attestationPolicies
// again: core/webapp/updater/unbound all have real signing policies and a
// correctly signed image must verify for all four, not just core/webapp -
// #230 extended the policy map to cover updater/unbound/blockpage but
// CheckStackAttestations itself, which is what /api/services actually
// calls, never started checking them and hardcoded not_applicable
// instead. adguard has no RootGuard policy (third-party image) and must
// stay not_applicable regardless.
func TestCheckStackAttestationsCoversCoreWebappUpdaterUnbound(t *testing.T) {
	resetAttestationCache()
	original := attestationRun
	defer func() { attestationRun = original }()
	attestationRun = func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"verified":true}`), nil
	}

	status := &StackStatus{
		Core:    ContainerInfo{Image: "ghcr.io/foxly-it/rootguard-core:v1@sha256:abc"},
		WebApp:  ContainerInfo{Image: "ghcr.io/foxly-it/rootguard-webapp:v1@sha256:abc"},
		Updater: ContainerInfo{Image: "ghcr.io/foxly-it/rootguard-updater:v1@sha256:abc"},
		AdGuard: ContainerInfo{Image: "adguard/adguardhome:v0.107.79@sha256:abc"},
		Unbound: ContainerInfo{Image: "ghcr.io/foxly-it/rootguard-unbound:v1@sha256:abc"},
	}
	CheckStackAttestations(context.Background(), status)

	for name, info := range map[string]ContainerInfo{
		"core": status.Core, "webapp": status.WebApp,
		"updater": status.Updater, "unbound": status.Unbound,
	} {
		if info.Attestation != "verified" {
			t.Fatalf("%s: expected verified, got %q", name, info.Attestation)
		}
	}
	if status.AdGuard.Attestation != "not_applicable" {
		t.Fatalf("adguard: expected not_applicable, got %q", status.AdGuard.Attestation)
	}
}

func TestDecodeContainerInspectReturnsOperatorMetadata(t *testing.T) {
	payload := []byte(`[{
		"State":{"Running":true,"Status":"running","StartedAt":"2026-07-28T08:15:00Z","Health":{"Status":"healthy"}},
		"Config":{"Image":"ghcr.io/foxly-it/rootguard-unbound:0.1.0-alpha.2@sha256:manifest","Labels":{"org.opencontainers.image.version":"0.1.0-alpha.2","org.opencontainers.image.revision":"abc123","org.opencontainers.image.created":"2026-07-28T08:00:00Z","org.opencontainers.image.source":"https://github.com/foxly-it/rootguard-unbound"}},
		"Image":"sha256:abcdef1234567890",
		"RestartCount":2,
		"NetworkSettings":{"Ports":{"53/udp":[{"HostIp":"0.0.0.0","HostPort":"53"}],"53/tcp":[{"HostIp":"0.0.0.0","HostPort":"53"}],"5335/tcp":null}}
	}]`)
	info, err := decodeContainerInspect(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Exists || !info.Running || info.Health != "healthy" || info.RestartCount != 2 {
		t.Fatalf("unexpected runtime metadata: %+v", info)
	}
	if info.ImageID != "sha256:abcdef1234567890" || info.StartedAt != "2026-07-28T08:15:00Z" {
		t.Fatalf("unexpected image metadata: %+v", info)
	}
	if !info.Immutable || info.Metadata != "complete" || info.Version != "0.1.0-alpha.2" || info.Revision != "abc123" {
		t.Fatalf("unexpected release metadata: %+v", info)
	}
	if len(info.Ports) != 2 || info.Ports[0] != "53/tcp" || info.Ports[1] != "53/udp" {
		t.Fatalf("ports are not stable and sorted: %+v", info.Ports)
	}
}

func TestDecodeContainerInspectHandlesMissingContainer(t *testing.T) {
	info, err := decodeContainerInspect([]byte(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Exists || info.Status != "missing" || info.Health != "unknown" {
		t.Fatalf("unexpected missing state: %+v", info)
	}
}

func TestDecodeContainerInspectDistinguishesMissingHealthcheck(t *testing.T) {
	payload := []byte(`[{
		"State":{"Running":true,"Status":"running","StartedAt":"2026-07-28T08:15:00Z"},
		"Config":{"Image":"adguard/adguardhome:v0.107.78"},
		"Image":"sha256:abcdef1234567890",
		"RestartCount":0,
		"NetworkSettings":{"Ports":{}}
	}]`)
	info, err := decodeContainerInspect(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Running || info.Health != "not_configured" {
		t.Fatalf("expected a running container without a healthcheck, got %+v", info)
	}
	if info.Immutable || info.Metadata != "unavailable" {
		t.Fatalf("unexpected trust metadata: %+v", info)
	}
}

func TestDecodeContainerInspectMarksPartialMetadata(t *testing.T) {
	payload := []byte(`[{"State":{"Running":true,"Status":"running"},"Config":{"Image":"rootguard-core:dev","Labels":{"org.opencontainers.image.version":"dev"}},"Image":"sha256:local","NetworkSettings":{"Ports":{}}}]`)
	info, err := decodeContainerInspect(payload)
	if err != nil {
		t.Fatal(err)
	}
	if info.Metadata != "partial" || info.Immutable || info.Version != "dev" {
		t.Fatalf("unexpected partial metadata: %+v", info)
	}
}
