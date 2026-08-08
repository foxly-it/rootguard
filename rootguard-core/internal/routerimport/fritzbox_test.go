package routerimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fixtureHosts mirrors real GetGenericHostEntry response shapes captured
// live from a FRITZ!Box 6690 Cable (firmware 267.08.25).
var fixtureHosts = []struct {
	ip, mac, active, name string
}{
	{"192.168.178.3", "32:3A:FD:E9:D6:87", "1", "Access-Point"},
	{"192.168.178.42", "AA:BB:CC:DD:EE:FF", "0", "printer"},
}

func fritzBoxFixtureServer(t *testing.T, requireAuth bool) (*httptest.Server, string) {
	t.Helper()
	const realm = "F!Box SL"
	const nonce = "abc123nonce"
	authenticated := false

	mux := http.NewServeMux()
	mux.HandleFunc(fritzBoxHostsControlURL, func(w http.ResponseWriter, r *http.Request) {
		if requireAuth && !authenticated && r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm=%q, nonce=%q, qop="auth"`, realm, nonce))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if requireAuth && !authenticated {
			if !strings.Contains(r.Header.Get("Authorization"), `username="tester"`) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		body := readBody(t, r)
		switch {
		case strings.Contains(body, "GetHostNumberOfEntries"):
			fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetHostNumberOfEntriesResponse xmlns:u="urn:dslforum-org:service:Hosts:1"><NewHostNumberOfEntries>%d</NewHostNumberOfEntries></u:GetHostNumberOfEntriesResponse></s:Body></s:Envelope>`, len(fixtureHosts))
		case strings.Contains(body, "GetGenericHostEntry"):
			index := extractIndex(t, body)
			if index < 0 || index >= len(fixtureHosts) {
				w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
				fmt.Fprint(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError xmlns="urn:dslforum-org:control-1-0"><errorCode>713</errorCode><errorDescription>SpecifiedArrayIndexInvalid</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
				return
			}
			entry := fixtureHosts[index]
			fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetGenericHostEntryResponse xmlns:u="urn:dslforum-org:service:Hosts:1"><NewIPAddress>%s</NewIPAddress><NewAddressSource>Static</NewAddressSource><NewLeaseTimeRemaining>0</NewLeaseTimeRemaining><NewMACAddress>%s</NewMACAddress><NewInterfaceType>Ethernet</NewInterfaceType><NewActive>%s</NewActive><NewHostName>%s</NewHostName></u:GetGenericHostEntryResponse></s:Body></s:Envelope>`, entry.ip, entry.mac, entry.active, entry.name)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, server.URL
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func extractIndex(t *testing.T, body string) int {
	t.Helper()
	start := strings.Index(body, "<NewIndex>")
	if start < 0 {
		return -1
	}
	start += len("<NewIndex>")
	end := strings.Index(body[start:], "</NewIndex>")
	if end < 0 {
		return -1
	}
	var index int
	if _, err := fmt.Sscanf(body[start:start+end], "%d", &index); err != nil {
		return -1
	}
	return index
}

// newTestClient points a client directly at the httptest server's own
// dynamically-assigned host:port, bypassing NewFritzBoxClient's fixed
// production port 49000.
func newTestClient(_ *testing.T, baseURL string) *FritzBoxClient {
	return &FritzBoxClient{baseURL: baseURL, http: &http.Client{Timeout: fritzBoxRequestTimeout}}
}

func TestDiscoverHostsWithoutAuthChallenge(t *testing.T) {
	_, address := fritzBoxFixtureServer(t, false)
	client := newTestClient(t, address)

	result, err := client.DiscoverHosts(context.Background(), Credentials{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d: %+v", len(result.Hosts), result.Hosts)
	}
	if result.Truncated {
		t.Fatal("did not expect truncation")
	}
	if result.Hosts[0].Hostname != "Access-Point" || result.Hosts[0].IPv4 != "192.168.178.3" || !result.Hosts[0].Active {
		t.Fatalf("unexpected first host: %+v", result.Hosts[0])
	}
	if result.Hosts[1].Active {
		t.Fatalf("expected second host to be inactive: %+v", result.Hosts[1])
	}
	for _, host := range result.Hosts {
		if host.Source != sourceFritzBoxTR064 {
			t.Fatalf("unexpected source: %q", host.Source)
		}
	}
}

func TestDiscoverHostsAnswersDigestChallenge(t *testing.T) {
	_, address := fritzBoxFixtureServer(t, true)
	client := newTestClient(t, address)

	result, err := client.DiscoverHosts(context.Background(), Credentials{Username: "tester", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(result.Hosts))
	}
}

func TestDiscoverHostsRequiresCredentialsWhenChallenged(t *testing.T) {
	_, address := fritzBoxFixtureServer(t, true)
	client := newTestClient(t, address)

	_, err := client.DiscoverHosts(context.Background(), Credentials{})
	if !errors.Is(err, ErrRouterDiscovery) {
		t.Fatalf("expected ErrRouterDiscovery, got %v", err)
	}
	if !strings.Contains(err.Error(), "requires FRITZ!Box credentials") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestDiscoverHostsRejectsWrongCredentials(t *testing.T) {
	_, address := fritzBoxFixtureServer(t, true)
	client := newTestClient(t, address)

	_, err := client.DiscoverHosts(context.Background(), Credentials{Username: "wrong", Password: "wrong"})
	if !errors.Is(err, ErrRouterDiscovery) {
		t.Fatalf("expected ErrRouterDiscovery, got %v", err)
	}
	if !strings.Contains(err.Error(), "rejected the provided credentials") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestNewFritzBoxClientRejectsUnsafeAddresses(t *testing.T) {
	tests := []string{"", "  ", "192.168.178.1/upnp/control/hosts", "192.168.178.1 extra", strings.Repeat("a", 254)}
	for _, address := range tests {
		if _, err := NewFritzBoxClient(address); !errors.Is(err, ErrRouterDiscovery) {
			t.Errorf("address %q: expected ErrRouterDiscovery, got %v", address, err)
		}
	}
}

func TestDigestChallengeAuthorizationHeaderIsWellFormed(t *testing.T) {
	challenge, ok := parseDigestChallenge(`Digest realm="F!Box SL", nonce="abc123", qop="auth"`)
	if !ok {
		t.Fatal("expected challenge to parse")
	}
	header, err := challenge.authorizationHeader("tester", "secret", http.MethodPost, "/upnp/control/hosts")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`username="tester"`, `realm="F!Box SL"`, `nonce="abc123"`, `qop=auth`, `nc=00000001`, `uri="/upnp/control/hosts"`} {
		if !strings.Contains(header, expected) {
			t.Errorf("authorization header missing %q: %s", expected, header)
		}
	}
}
