package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// dialDockerProxy is swapped out in tests so checkDockerProxyReachable's
// own logic can be verified without a real TCP dial - kept separate from
// attestation.go's dialProxy since the two check independent proxies.
var dialDockerProxy = func(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, 3*time.Second)
}

// checkDockerProxyReachable is rootguard-core/internal/stack.CheckDockerProxyReachable's
// standalone copy for this module (a different Go module entirely, same
// reason attestation.go carries its own copy of the attestation-proxy
// check instead of importing Core's). See that function's doc comment
// for the full rationale, including why an unset
// ROOTGUARD_DOCKER_PROXY_URL is deliberately not an error here, unlike
// checkAttestationProxyReachable's own unset case.
func checkDockerProxyReachable() error {
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
