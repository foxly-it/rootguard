package stack

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// dialDockerProxy is swapped out in tests so CheckDockerProxyReachable's
// own logic can be verified without a real TCP dial - same pattern as
// attestation.go's dialProxy, kept as its own variable rather than shared
// since the two check independent proxies.
var dialDockerProxy = func(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, 3*time.Second)
}

// CheckDockerProxyReachable is CheckAttestationProxyReachable's
// counterpart for rootguard-docker-proxy, with one deliberate difference:
// an unset ROOTGUARD_DOCKER_PROXY_URL is not an error here. Attestation's
// check only ever gates one narrow operation (activating a policy-covered
// image), so treating "unset" as fatal there simply blocks that one
// operation on an installation whose compose topology predates the
// attestation proxy. Docker connectivity, by contrast, is needed for
// nearly everything Core does - and self-update can never deliver a
// compose-topology change to an existing installation (only a fresh
// install or a manual compose.release.yaml refresh can, see
// docs/release-process.md), so an installation whose compose.release.yaml
// predates rootguard-docker-proxy will keep receiving new Core binaries
// with no ROOTGUARD_DOCKER_PROXY_URL set at all. Treating that as fatal
// at startup would brick every such installation the moment it received
// this change, not just gate one feature - so "unset" here means "this
// installation still mounts the real socket directly, same as before",
// and callers that care about the fresh-topology guarantee check
// os.Getenv themselves before deciding whether to call this at all (see
// cmd/rootguard/main.go).
//
// Once a URL *is* configured, it's a hard dependency: this function
// verifies it with a real HTTP GET against the proxy's own /healthz
// (rootguard-docker-proxy/proxy.go), not just a TCP dial-and-close, for
// the same reason CheckAttestationProxyReachable does - a bare TCP
// connect succeeds against anything that merely accepts connections
// without speaking the proxy's protocol at all.
func CheckDockerProxyReachable() error {
	proxyURL := os.Getenv("ROOTGUARD_DOCKER_PROXY_URL")
	if proxyURL == "" {
		return nil
	}
	if !strings.Contains(proxyURL, "://") {
		proxyURL = "http://" + proxyURL
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialDockerProxy(network, addr)
			},
		},
		Timeout: 5 * time.Second,
	}
	response, err := client.Get(strings.TrimSuffix(proxyURL, "/") + "/healthz")
	if err != nil {
		return fmt.Errorf("docker proxy configured (%s) but unreachable: %w - this installation's compose topology may be missing the rootguard-docker-proxy service; a fresh install or a manual compose.release.yaml refresh is required, see docs/release-process.md", proxyURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("docker proxy configured (%s) but unhealthy: /healthz returned %s - this installation's compose topology may be missing the rootguard-docker-proxy service; a fresh install or a manual compose.release.yaml refresh is required, see docs/release-process.md", proxyURL, response.Status)
	}
	return nil
}
