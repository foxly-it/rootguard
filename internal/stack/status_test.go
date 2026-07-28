package stack

import "testing"

func TestDecodeContainerInspectReturnsOperatorMetadata(t *testing.T) {
	payload := []byte(`[{
		"State":{"Running":true,"Status":"running","StartedAt":"2026-07-28T08:15:00Z","Health":{"Status":"healthy"}},
		"Config":{"Image":"ghcr.io/foxly-it/rootguard-unbound:0.1.0-alpha.2"},
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
}
