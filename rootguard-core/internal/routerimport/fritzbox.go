// Package routerimport discovers local hosts from a router, as an untrusted
// draft to offer against Unbound's LocalZone/LocalHost model - never applied
// directly. FritzBoxClient is the first adapter; other vendors should follow
// the same DiscoveredHost/DiscoveryResult shape.
package routerimport

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	fritzBoxDefaultPort     = 49000
	fritzBoxHostsService    = "urn:dslforum-org:service:Hosts:1"
	fritzBoxHostsControlURL = "/upnp/control/hosts"
	fritzBoxRequestTimeout  = 8 * time.Second
	fritzBoxDiscoveryBudget = 30 * time.Second
	maxDiscoveredHosts      = 256
	maxSOAPResponseBytes    = 64 << 10

	sourceFritzBoxTR064 = "fritzbox-tr064"
)

var ErrRouterDiscovery = errors.New("router discovery failed")

type Credentials struct {
	Username string
	Password string
}

type DiscoveredHost struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
	IPv6     string `json:"ipv6,omitempty"`
	MAC      string `json:"mac,omitempty"`
	Active   bool   `json:"active"`
	Source   string `json:"source"`
}

type DiscoveryResult struct {
	Hosts     []DiscoveredHost `json:"hosts"`
	Truncated bool             `json:"truncated"`
	Scanned   int              `json:"scanned,omitempty"`
	Failed    int              `json:"failed,omitempty"`
}

// FritzBoxClient speaks the standard (non-AVM-proprietary) TR-064 Hosts:1
// service: GetHostNumberOfEntries followed by a bounded GetGenericHostEntry
// loop. Verified live against a real FRITZ!Box 6690 Cable, firmware
// 267.08.25.
type FritzBoxClient struct {
	baseURL string
	http    *http.Client
}

func NewFritzBoxClient(address string) (*FritzBoxClient, error) {
	host, err := normalizeRouterAddress(address)
	if err != nil {
		return nil, err
	}
	// net.JoinHostPort brackets an IPv6 literal itself
	// ("http://[fd00::1]:49000", not the invalid/ambiguous
	// "http://fd00::1:49000" a bare Sprintf produced before) - strip any
	// brackets the caller already included first, or it would double up.
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	return &FritzBoxClient{
		baseURL: "http://" + net.JoinHostPort(host, strconv.Itoa(fritzBoxDefaultPort)),
		http: &http.Client{
			Timeout:       fritzBoxRequestTimeout,
			CheckRedirect: rejectRedirects,
			Transport:     &http.Transport{DialContext: dialPrivateOnly},
		},
	}, nil
}

// rejectRedirects found in review: address only ever gets a light
// syntax check (normalizeRouterAddress), never a network-reachability
// one, so a router responding with a redirect - or an attacker-supplied
// address doing the same - could otherwise steer this client's request
// (including, on a same-host redirect, its Digest Authorization header)
// wherever it likes. Nothing about "import hosts from my router" needs a
// redirect ever followed.
func rejectRedirects(*http.Request, []*http.Request) error {
	return fmt.Errorf("%w: router responded with a redirect, refusing to follow it", ErrRouterDiscovery)
}

// dialPrivateOnly is the other half of the same review finding: even
// without redirects, address itself was never restricted to plausible
// router locations, so this client would happily speak TR-064 to any
// public IP an admin (or anything that can drive this endpoint through
// them) supplied. DNS rebinding is closed by checking the address the
// dialer actually resolved to, not the pre-resolution hostname - a
// hostname that looked private during an earlier check could otherwise
// repoint to a public one by the time the connection actually happens.
//
// rejectNonPrivateControl (below) is where that check happens, via
// net.Dialer.Control: found in a follow-up review that the original
// version of this check dialed first and inspected the resulting
// conn.RemoteAddr() afterwards - real, but a genuine TCP connect(2) to
// the disallowed address still happened before the connection was torn
// down, letting a caller distinguish an open port from a closed one on a
// host this client should never have touched at all. Control runs after
// DNS resolution but before connect(2), so a disallowed address is never
// actually contacted.
func dialPrivateOnly(ctx context.Context, network, address string) (net.Conn, error) {
	return rawDial(ctx, network, address)
}

var rawDial = (&net.Dialer{Timeout: fritzBoxRequestTimeout, Control: rejectNonPrivateControl}).DialContext

// rejectNonPrivateControl is a net.Dialer.Control hook. address is the
// concrete ip:port the dialer resolved and is about to connect(2) to -
// never the original, pre-resolution hostname - so this still closes DNS
// rebinding the same way the old post-dial check did. Never touches rawConn,
// so tests can call it directly with a nil syscall.RawConn instead of
// needing a real (or faked) network connection.
func rejectNonPrivateControl(_ string, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: could not determine the resolved address", ErrRouterDiscovery)
	}
	// netip.ParseAddr, not net.ParseIP: found in review that a link-local
	// IPv6 address dialed with a required zone identifier (e.g.
	// "fe80::1%en0" - link-local addresses are ambiguous without one,
	// since the same address can exist on multiple interfaces) resolves
	// to exactly that shape here, and net.ParseIP has never supported the
	// "%zone" suffix at all - it returns nil for any address containing
	// one, unconditionally refusing every zone-qualified link-local
	// address regardless of whether it was actually private. netip.Addr
	// parses the zone correctly and exposes the identical
	// IsPrivate/IsLinkLocalUnicast/IsLoopback methods net.IP has.
	ip, err := netip.ParseAddr(host)
	if err != nil || !isRouterReachable(ip) {
		return fmt.Errorf("%w: refusing to contact %s - not a private network address", ErrRouterDiscovery, host)
	}
	return nil
}

// isRouterReachable accepts private (RFC 1918 / ULA), link-local, and
// loopback addresses - the ranges a home or small-office router
// plausibly sits on - and rejects everything globally routable.
// Loopback is included mainly for testability (this package's own tests
// exercise the client against httptest servers, which bind there); it's
// also not meaningfully riskier than the private ranges above it for
// this client's actual blast radius.
func isRouterReachable(ip netip.Addr) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLoopback()
}

func normalizeRouterAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", fmt.Errorf("%w: router address is required", ErrRouterDiscovery)
	}
	if len(address) > 253 {
		return "", fmt.Errorf("%w: router address is too long", ErrRouterDiscovery)
	}
	if strings.ContainsAny(address, "/\\ \t\r\n") {
		return "", fmt.Errorf("%w: router address must be a bare hostname or IP address", ErrRouterDiscovery)
	}
	return address, nil
}

// DiscoverHosts fetches the router's known hosts. Credentials are used only
// for this call - the client never persists them, and callers must not
// either (they belong in generated Unbound config or browser storage even
// less than in Core's own state).
func (c *FritzBoxClient) DiscoverHosts(ctx context.Context, creds Credentials) (DiscoveryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, fritzBoxDiscoveryBudget)
	defer cancel()

	count, err := c.hostNumberOfEntries(ctx, creds)
	if err != nil {
		return DiscoveryResult{}, err
	}
	if count < 0 {
		count = 0
	}
	truncated := count > maxDiscoveredHosts
	if truncated {
		count = maxDiscoveredHosts
	}

	hosts := make([]DiscoveredHost, 0, count)
	for index := range count {
		select {
		case <-ctx.Done():
			return DiscoveryResult{}, fmt.Errorf("%w: timed out after %d of %d entries: %v", ErrRouterDiscovery, index, count, ctx.Err())
		default:
		}
		host, err := c.genericHostEntry(ctx, creds, index)
		if err != nil {
			return DiscoveryResult{}, fmt.Errorf("%w: entry %d: %v", ErrRouterDiscovery, index, err)
		}
		if host.IPv4 == "" && host.Hostname == "" {
			continue
		}
		hosts = append(hosts, host)
	}
	return DiscoveryResult{Hosts: hosts, Truncated: truncated}, nil
}

func (c *FritzBoxClient) hostNumberOfEntries(ctx context.Context, creds Credentials) (int, error) {
	body, err := c.call(ctx, creds, "GetHostNumberOfEntries", "")
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Count int `xml:"Body>GetHostNumberOfEntriesResponse>NewHostNumberOfEntries"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("%w: could not parse host count: %v", ErrRouterDiscovery, err)
	}
	return parsed.Count, nil
}

func (c *FritzBoxClient) genericHostEntry(ctx context.Context, creds Credentials, index int) (DiscoveredHost, error) {
	body, err := c.call(ctx, creds, "GetGenericHostEntry", fmt.Sprintf("<NewIndex>%d</NewIndex>", index))
	if err != nil {
		return DiscoveredHost{}, err
	}
	var parsed struct {
		IPAddress  string `xml:"Body>GetGenericHostEntryResponse>NewIPAddress"`
		MACAddress string `xml:"Body>GetGenericHostEntryResponse>NewMACAddress"`
		Active     string `xml:"Body>GetGenericHostEntryResponse>NewActive"`
		HostName   string `xml:"Body>GetGenericHostEntryResponse>NewHostName"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return DiscoveredHost{}, fmt.Errorf("%w: could not parse host entry: %v", ErrRouterDiscovery, err)
	}
	return DiscoveredHost{
		Hostname: parsed.HostName,
		IPv4:     parsed.IPAddress,
		MAC:      parsed.MACAddress,
		Active:   parsed.Active == "1",
		Source:   sourceFritzBoxTR064,
	}, nil
}

// call issues one SOAP action against the Hosts service, transparently
// answering an HTTP Digest challenge if the router sends one - some
// deployments require it, others answer 200 directly (observed live).
func (c *FritzBoxClient) call(ctx context.Context, creds Credentials, action, argsXML string) ([]byte, error) {
	requestBody := buildSOAPRequestBody(action, argsXML)
	soapAction := fritzBoxHostsService + "#" + action

	status, body, wwwAuthenticate, err := c.postSOAP(ctx, requestBody, soapAction, "")
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized {
		if creds.Username == "" {
			return nil, fmt.Errorf("%w: this router requires FRITZ!Box credentials for host discovery", ErrRouterDiscovery)
		}
		challenge, ok := parseDigestChallenge(wwwAuthenticate)
		if !ok {
			return nil, fmt.Errorf("%w: router requires authentication but sent no usable challenge", ErrRouterDiscovery)
		}
		authorization, err := challenge.authorizationHeader(creds.Username, creds.Password, http.MethodPost, fritzBoxHostsControlURL)
		if err != nil {
			return nil, fmt.Errorf("%w: building authentication response: %v", ErrRouterDiscovery, err)
		}
		status, body, _, err = c.postSOAP(ctx, requestBody, soapAction, authorization)
		if err != nil {
			return nil, err
		}
		if status == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: router rejected the provided credentials", ErrRouterDiscovery)
		}
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected router response status %d", ErrRouterDiscovery, status)
	}

	var fault struct {
		Fault *soapFault `xml:"Body>Fault"`
	}
	if err := xml.Unmarshal(body, &fault); err == nil && fault.Fault != nil {
		return nil, fmt.Errorf("%w: %s", ErrRouterDiscovery, fault.Fault.describe())
	}
	return body, nil
}

func (c *FritzBoxClient) postSOAP(ctx context.Context, body []byte, soapAction, authorization string) (status int, responseBody []byte, wwwAuthenticate string, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+fritzBoxHostsControlURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, "", fmt.Errorf("%w: building request: %v", ErrRouterDiscovery, err)
	}
	request.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	request.Header.Set("SOAPAction", soapAction)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return 0, nil, "", fmt.Errorf("%w: contacting router: %v", ErrRouterDiscovery, err)
	}
	defer response.Body.Close()
	responseBody, err = io.ReadAll(io.LimitReader(response.Body, maxSOAPResponseBytes))
	if err != nil {
		return 0, nil, "", fmt.Errorf("%w: reading router response: %v", ErrRouterDiscovery, err)
	}
	return response.StatusCode, responseBody, response.Header.Get("WWW-Authenticate"), nil
}

func buildSOAPRequestBody(action, argsXML string) []byte {
	return fmt.Appendf(nil,
		`<?xml version="1.0" encoding="utf-8"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:%s xmlns:u=%q>%s</u:%s></s:Body></s:Envelope>`,
		action, fritzBoxHostsService, argsXML, action)
}

type soapFault struct {
	FaultString      string `xml:"faultstring"`
	ErrorCode        string `xml:"detail>UPnPError>errorCode"`
	ErrorDescription string `xml:"detail>UPnPError>errorDescription"`
}

func (f soapFault) describe() string {
	switch {
	case f.ErrorDescription != "":
		return fmt.Sprintf("%s (UPnP error %s)", f.ErrorDescription, strconv.Itoa(mustAtoiOrZero(f.ErrorCode)))
	case f.FaultString != "":
		return f.FaultString
	default:
		return "unknown router fault"
	}
}

func mustAtoiOrZero(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}
