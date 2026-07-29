package unbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrInvalidSettings = errors.New("invalid unbound settings")

const (
	maxForwardZones        = 32
	maxForwardServers      = 8
	maxForwardTargets      = 32
	maxPrivateDomains      = 32
	reverseModeNXDOMAIN    = "nxdomain"
	reverseModeTransparent = "transparent"
	networkModeIPv4        = "ipv4"
	networkModeDual        = "dual"
	networkModeIPv6        = "ipv6"
	resourceProfileSmall   = "small"
	resourceProfileMedium  = "medium"
	resourceProfileLarge   = "large"
)

var rootGuardDNSNetwork = netip.MustParsePrefix("172.29.53.0/24")

type ForwardZone struct {
	Name                  string   `json:"name"`
	Servers               []string `json:"servers"`
	ForwardFirst          bool     `json:"forward_first"`
	AllowUnsigned         bool     `json:"allow_unsigned"`
	AllowPrivateAddresses bool     `json:"allow_private_addresses"`
}

type ReverseZonePolicy struct {
	Network string `json:"network"`
	Mode    string `json:"mode"`
}

type Settings struct {
	QnameMinimisation         bool                `json:"qname_minimisation"`
	Prefetch                  bool                `json:"prefetch"`
	ServeExpired              bool                `json:"serve_expired"`
	ServeExpiredTTL           int                 `json:"serve_expired_ttl"`
	ServeExpiredClientTimeout int                 `json:"serve_expired_client_timeout"`
	CacheMinTTL               int                 `json:"cache_min_ttl"`
	CacheMaxTTL               int                 `json:"cache_max_ttl"`
	Threads                   int                 `json:"threads"`
	ResourceProfile           string              `json:"resource_profile"`
	NetworkMode               string              `json:"network_mode"`
	ForwardZones              []ForwardZone       `json:"forward_zones"`
	PrivateDomains            []string            `json:"private_domains"`
	ReverseZones              []ReverseZonePolicy `json:"reverse_zones"`
}

type ActiveConfiguration struct {
	BaseConfig    string    `json:"base_config"`
	ManagedConfig string    `json:"managed_config"`
	CustomConfig  string    `json:"custom_config"`
	CheckedAt     time.Time `json:"checked_at"`
}

func DefaultSettings() Settings {
	return Settings{
		QnameMinimisation:         true,
		Prefetch:                  true,
		ServeExpired:              true,
		ServeExpiredTTL:           86400,
		ServeExpiredClientTimeout: 1800,
		CacheMinTTL:               0,
		CacheMaxTTL:               86400,
		Threads:                   2,
		ResourceProfile:           resourceProfileMedium,
		NetworkMode:               networkModeIPv4,
		ForwardZones:              []ForwardZone{},
		PrivateDomains:            []string{},
		ReverseZones: []ReverseZonePolicy{
			{Network: "10.0.0.0/8", Mode: reverseModeNXDOMAIN},
			{Network: "172.16.0.0/12", Mode: reverseModeNXDOMAIN},
			{Network: "192.168.0.0/16", Mode: reverseModeNXDOMAIN},
		},
	}
}

func (s Settings) Validate() error {
	if s.CacheMinTTL < 0 || s.CacheMinTTL > 3600 {
		return fmt.Errorf("%w: cache_min_ttl must be between 0 and 3600", ErrInvalidSettings)
	}
	if s.ServeExpiredTTL < 3600 || s.ServeExpiredTTL > 604800 {
		return fmt.Errorf("%w: serve_expired_ttl must be between 3600 and 604800 seconds", ErrInvalidSettings)
	}
	if s.ServeExpiredClientTimeout < 0 || s.ServeExpiredClientTimeout > 5000 {
		return fmt.Errorf("%w: serve_expired_client_timeout must be between 0 and 5000 milliseconds", ErrInvalidSettings)
	}
	if s.CacheMaxTTL < 60 || s.CacheMaxTTL > 604800 {
		return fmt.Errorf("%w: cache_max_ttl must be between 60 and 604800", ErrInvalidSettings)
	}
	if s.CacheMinTTL > s.CacheMaxTTL {
		return fmt.Errorf("%w: cache_min_ttl must not exceed cache_max_ttl", ErrInvalidSettings)
	}
	if s.Threads < 1 || s.Threads > 32 {
		return fmt.Errorf("%w: threads must be between 1 and 32", ErrInvalidSettings)
	}
	if _, ok := resourceProfileCacheSizes[s.ResourceProfile]; !ok {
		return fmt.Errorf("%w: resource_profile must be small, medium, or large", ErrInvalidSettings)
	}
	if s.NetworkMode != networkModeIPv4 && s.NetworkMode != networkModeDual && s.NetworkMode != networkModeIPv6 {
		return fmt.Errorf("%w: network_mode must be ipv4, dual, or ipv6", ErrInvalidSettings)
	}
	if len(s.ForwardZones) > maxForwardZones {
		return fmt.Errorf("%w: forward_zones must contain at most %d zones", ErrInvalidSettings, maxForwardZones)
	}
	zoneNames := make(map[string]struct{}, len(s.ForwardZones))
	targetCount := 0
	for zoneIndex, zone := range s.ForwardZones {
		if err := validateCanonicalZoneName(zone.Name); err != nil {
			return fmt.Errorf("%w: forward_zones[%d].name: %v", ErrInvalidSettings, zoneIndex, err)
		}
		if _, exists := zoneNames[zone.Name]; exists {
			return fmt.Errorf("%w: forward_zones[%d].name duplicates %q", ErrInvalidSettings, zoneIndex, zone.Name)
		}
		zoneNames[zone.Name] = struct{}{}
		if len(zone.Servers) == 0 || len(zone.Servers) > maxForwardServers {
			return fmt.Errorf("%w: forward_zones[%d].servers must contain between 1 and %d addresses", ErrInvalidSettings, zoneIndex, maxForwardServers)
		}
		targetCount += len(zone.Servers)
		if targetCount > maxForwardTargets {
			return fmt.Errorf("%w: forward_zones must contain at most %d total server addresses", ErrInvalidSettings, maxForwardTargets)
		}
		servers := make(map[netip.Addr]struct{}, len(zone.Servers))
		for serverIndex, server := range zone.Servers {
			address, err := netip.ParseAddr(server)
			if err != nil || address.String() != server {
				return fmt.Errorf("%w: forward_zones[%d].servers[%d] must be a canonical IPv4 or IPv6 address", ErrInvalidSettings, zoneIndex, serverIndex)
			}
			routedAddress := address.Unmap()
			if routedAddress.IsUnspecified() || routedAddress.IsLoopback() || routedAddress.IsMulticast() || routedAddress.IsLinkLocalUnicast() || rootGuardDNSNetwork.Contains(routedAddress) {
				return fmt.Errorf("%w: forward_zones[%d].servers[%d] points to a local or reserved RootGuard resolver address", ErrInvalidSettings, zoneIndex, serverIndex)
			}
			if _, exists := servers[address]; exists {
				return fmt.Errorf("%w: forward_zones[%d].servers[%d] duplicates %q", ErrInvalidSettings, zoneIndex, serverIndex, server)
			}
			servers[address] = struct{}{}
		}
	}
	if len(s.PrivateDomains) > maxPrivateDomains {
		return fmt.Errorf("%w: private_domains must contain at most %d domains", ErrInvalidSettings, maxPrivateDomains)
	}
	privateNames := make(map[string]struct{}, len(s.PrivateDomains))
	for domainIndex, domain := range s.PrivateDomains {
		if err := validateCanonicalZoneName(domain); err != nil {
			return fmt.Errorf("%w: private_domains[%d]: %v", ErrInvalidSettings, domainIndex, err)
		}
		if _, exists := privateNames[domain]; exists {
			return fmt.Errorf("%w: private_domains[%d] duplicates %q", ErrInvalidSettings, domainIndex, domain)
		}
		privateNames[domain] = struct{}{}
		for _, zone := range s.ForwardZones {
			if zone.AllowPrivateAddresses && zone.Name == domain {
				return fmt.Errorf("%w: private domain %q duplicates the forwarding-zone rebinding exception", ErrInvalidSettings, domain)
			}
		}
	}
	if len(s.ReverseZones) > len(rfc1918ReverseZones) {
		return fmt.Errorf("%w: reverse_zones contains unsupported entries", ErrInvalidSettings)
	}
	reverseNetworks := make(map[string]struct{}, len(s.ReverseZones))
	for policyIndex, policy := range s.ReverseZones {
		if _, supported := rfc1918ReverseZones[policy.Network]; !supported {
			return fmt.Errorf("%w: reverse_zones[%d].network is not a supported RFC1918 range", ErrInvalidSettings, policyIndex)
		}
		if policy.Mode != reverseModeNXDOMAIN && policy.Mode != reverseModeTransparent {
			return fmt.Errorf("%w: reverse_zones[%d].mode must be nxdomain or transparent", ErrInvalidSettings, policyIndex)
		}
		if _, exists := reverseNetworks[policy.Network]; exists {
			return fmt.Errorf("%w: reverse_zones[%d] duplicates %q", ErrInvalidSettings, policyIndex, policy.Network)
		}
		reverseNetworks[policy.Network] = struct{}{}
	}
	return nil
}

var rfc1918ReverseZones = map[string][]string{
	"10.0.0.0/8":     {"10.in-addr.arpa."},
	"172.16.0.0/12":  reverse172Zones(),
	"192.168.0.0/16": {"168.192.in-addr.arpa."},
}

func reverse172Zones() []string {
	zones := make([]string, 0, 16)
	for secondOctet := 16; secondOctet <= 31; secondOctet++ {
		zones = append(zones, fmt.Sprintf("%d.172.in-addr.arpa.", secondOctet))
	}
	return zones
}

func validateCanonicalZoneName(name string) error {
	if name == "." {
		return errors.New("the root zone is managed by RootGuard recursion and cannot be forwarded")
	}
	if len(name) < 2 || len(name) > 254 || !strings.HasSuffix(name, ".") || name != strings.ToLower(name) || strings.TrimSpace(name) != name {
		return errors.New("must be a lowercase canonical FQDN ending in a dot")
	}
	labels := strings.Split(strings.TrimSuffix(name, "."), ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || !isASCIILetterOrDigit(label[0]) || !isASCIILetterOrDigit(label[len(label)-1]) {
			return errors.New("contains an invalid DNS label")
		}
		for index := 1; index < len(label)-1; index++ {
			if !isASCIILetterOrDigit(label[index]) && label[index] != '-' {
				return errors.New("contains an invalid DNS label")
			}
		}
	}
	return nil
}

func isASCIILetterOrDigit(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func settingsEqual(left, right Settings) bool {
	if left.QnameMinimisation != right.QnameMinimisation ||
		left.Prefetch != right.Prefetch ||
		left.ServeExpired != right.ServeExpired ||
		left.ServeExpiredTTL != right.ServeExpiredTTL ||
		left.ServeExpiredClientTimeout != right.ServeExpiredClientTimeout ||
		left.CacheMinTTL != right.CacheMinTTL ||
		left.CacheMaxTTL != right.CacheMaxTTL ||
		left.Threads != right.Threads ||
		left.ResourceProfile != right.ResourceProfile ||
		left.NetworkMode != right.NetworkMode ||
		len(left.ForwardZones) != len(right.ForwardZones) ||
		len(left.PrivateDomains) != len(right.PrivateDomains) ||
		len(left.ReverseZones) != len(right.ReverseZones) {
		return false
	}
	for index, leftZone := range left.ForwardZones {
		rightZone := right.ForwardZones[index]
		if leftZone.Name != rightZone.Name ||
			leftZone.ForwardFirst != rightZone.ForwardFirst ||
			leftZone.AllowUnsigned != rightZone.AllowUnsigned ||
			leftZone.AllowPrivateAddresses != rightZone.AllowPrivateAddresses ||
			len(leftZone.Servers) != len(rightZone.Servers) {
			return false
		}
		for serverIndex, leftServer := range leftZone.Servers {
			if leftServer != rightZone.Servers[serverIndex] {
				return false
			}
		}
	}
	for index, domain := range left.PrivateDomains {
		if domain != right.PrivateDomains[index] {
			return false
		}
	}
	for index, policy := range left.ReverseZones {
		if policy != right.ReverseZones[index] {
			return false
		}
	}
	return true
}

func (s Settings) Render() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	fmt.Fprintln(&out, "# Generated by RootGuard. Do not edit manually.")
	fmt.Fprintln(&out, "server:")
	fmt.Fprintln(&out, "    # Privacy: reveal only the currently required DNS label to each authoritative server.")
	fmt.Fprintf(&out, "    qname-minimisation: %s\n", yesNo(s.QnameMinimisation))
	fmt.Fprintln(&out, "    # Performance: refresh frequently used records shortly before their TTL expires.")
	fmt.Fprintf(&out, "    prefetch: %s\n", yesNo(s.Prefetch))
	fmt.Fprintln(&out, "    # Availability: keep serving cached records during temporary upstream failures.")
	fmt.Fprintf(&out, "    serve-expired: %s\n", yesNo(s.ServeExpired))
	fmt.Fprintln(&out, "    # Maximum age in seconds for stale records eligible as an availability fallback.")
	fmt.Fprintf(&out, "    serve-expired-ttl: %d\n", s.ServeExpiredTTL)
	fmt.Fprintln(&out, "    # Wait in milliseconds for fresh resolution before using an eligible stale answer.")
	fmt.Fprintf(&out, "    serve-expired-client-timeout: %d\n", s.ServeExpiredClientTimeout)
	fmt.Fprintln(&out, "    # Cache floor in seconds; higher values delay authoritative DNS changes.")
	fmt.Fprintf(&out, "    cache-min-ttl: %d\n", s.CacheMinTTL)
	fmt.Fprintln(&out, "    # Cache ceiling in seconds; records are never retained longer than this value.")
	fmt.Fprintf(&out, "    cache-max-ttl: %d\n", s.CacheMaxTTL)
	fmt.Fprintln(&out, "    # Parallel resolver workers; match this to the available CPU resources.")
	fmt.Fprintf(&out, "    num-threads: %d\n", s.Threads)
	cacheSizes := resourceProfileCacheSizes[s.ResourceProfile]
	fmt.Fprintln(&out, "    # Bounded cache memory derived from the selected RootGuard resource profile.")
	fmt.Fprintf(&out, "    rrset-cache-size: %s\n", cacheSizes.RRSet)
	fmt.Fprintf(&out, "    msg-cache-size: %s\n", cacheSizes.Message)
	fmt.Fprintln(&out, "    # Network mode: enable only protocol families confirmed by the RootGuard connectivity check.")
	fmt.Fprintf(&out, "    do-ip4: %s\n", yesNo(s.NetworkMode != networkModeIPv6))
	fmt.Fprintf(&out, "    do-ip6: %s\n", yesNo(s.NetworkMode != networkModeIPv4))
	fmt.Fprintf(&out, "    prefer-ip6: %s\n", yesNo(s.NetworkMode == networkModeIPv6))
	for _, domain := range s.PrivateDomains {
		fmt.Fprintln(&out, "    # Private domain: allow protected private-address answers only for this trusted DNS suffix.")
		fmt.Fprintf(&out, "    private-domain: %q\n", domain)
	}
	for _, zone := range s.ForwardZones {
		if zone.AllowUnsigned {
			fmt.Fprintln(&out, "    # Split DNS: explicitly trust unsigned answers for this private forwarding zone.")
			fmt.Fprintf(&out, "    domain-insecure: %q\n", zone.Name)
		}
		if zone.AllowPrivateAddresses {
			fmt.Fprintln(&out, "    # DNS rebinding: allow RFC1918 and other configured private-address answers only for this zone.")
			fmt.Fprintf(&out, "    private-domain: %q\n", zone.Name)
		}
	}
	for _, policy := range s.ReverseZones {
		for _, zone := range rfc1918ReverseZones[policy.Network] {
			fmt.Fprintln(&out, "    # RFC1918 reverse DNS: static returns local data or NXDOMAIN; transparent permits normal resolution.")
			fmt.Fprintf(&out, "    local-zone: %q %s\n", zone, reverseZoneType(policy.Mode))
		}
	}
	for _, zone := range s.ForwardZones {
		fmt.Fprintln(&out)
		fmt.Fprintln(&out, "# Conditional forwarding: send only this canonical DNS zone to the ordered targets.")
		fmt.Fprintln(&out, "forward-zone:")
		fmt.Fprintln(&out, "    # Zone suffix matched by this forwarding rule.")
		fmt.Fprintf(&out, "    name: %q\n", zone.Name)
		for _, server := range zone.Servers {
			fmt.Fprintln(&out, "    # Upstream resolver address; order is preserved for deterministic configuration.")
			fmt.Fprintf(&out, "    forward-addr: %s\n", server)
		}
		fmt.Fprintln(&out, "    # If enabled, fall back to normal recursion when every forward target fails.")
		fmt.Fprintf(&out, "    forward-first: %s\n", yesNo(zone.ForwardFirst))
	}
	return out.Bytes(), nil
}

func reverseZoneType(mode string) string {
	if mode == reverseModeTransparent {
		return "transparent"
	}
	return "static"
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

type Manager struct {
	hostConfigDir      string
	containerConfigDir string
	containerName      string
	run                commandRunner
	now                func() time.Time
	applyMu            sync.Mutex
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

func NewManager(hostConfigDir, containerConfigDir, containerName string) *Manager {
	return &Manager{
		hostConfigDir: hostConfigDir, containerConfigDir: containerConfigDir,
		containerName: containerName,
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
		now: time.Now,
	}
}

func (m *Manager) Load() (Settings, error) {
	data, err := os.ReadFile(filepath.Join(m.hostConfigDir, "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("decode saved settings: %w", err)
	}
	if settings.ForwardZones == nil {
		settings.ForwardZones = []ForwardZone{}
	}
	if settings.PrivateDomains == nil {
		settings.PrivateDomains = []string{}
	}
	if settings.ReverseZones == nil {
		settings.ReverseZones = DefaultSettings().ReverseZones
	}
	if settings.NetworkMode == "" {
		settings.NetworkMode = networkModeIPv4
	}
	if settings.ResourceProfile == "" {
		settings.ResourceProfile = resourceProfileMedium
	}
	if settings.ServeExpiredTTL == 0 {
		settings.ServeExpiredTTL = 86400
	}
	if !jsonFieldExists(data, "serve_expired_client_timeout") {
		settings.ServeExpiredClientTimeout = 1800
	}
	return settings, settings.Validate()
}

func jsonFieldExists(data []byte, field string) bool {
	var document map[string]json.RawMessage
	if json.Unmarshal(data, &document) != nil {
		return false
	}
	_, exists := document[field]
	return exists
}

type cacheSizes struct {
	RRSet   string
	Message string
}

var resourceProfileCacheSizes = map[string]cacheSizes{
	resourceProfileSmall:  {RRSet: "32m", Message: "16m"},
	resourceProfileMedium: {RRSet: "64m", Message: "32m"},
	resourceProfileLarge:  {RRSet: "128m", Message: "64m"},
}

func (m *Manager) ActiveConfiguration(ctx context.Context) (ActiveConfiguration, error) {
	readContainerFile := func(path string) (string, error) {
		output, err := m.run(ctx, "docker", "exec", m.containerName, "cat", path)
		if err != nil {
			return "", fmt.Errorf("read active Unbound file %s: %w: %s", path, err, strings.TrimSpace(string(output)))
		}
		return string(output), nil
	}

	base, err := readContainerFile("/etc/unbound/unbound.conf")
	if err != nil {
		return ActiveConfiguration{}, err
	}
	managed, err := readContainerFile("/etc/unbound/unbound.d/50-rootguard.conf")
	if err != nil {
		return ActiveConfiguration{}, err
	}
	custom, err := m.LoadCustom()
	if err != nil {
		return ActiveConfiguration{}, err
	}
	return ActiveConfiguration{
		BaseConfig:    base,
		ManagedConfig: managed,
		CustomConfig:  custom.Content,
		CheckedAt:     m.now().UTC(),
	}, nil
}

func (m *Manager) Apply(ctx context.Context, settings Settings) error {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	custom, err := m.LoadCustom()
	if err != nil {
		return err
	}
	return m.applyStateLocked(ctx, settings, custom.Content)
}

func (m *Manager) applyStateLocked(ctx context.Context, settings Settings, custom string) error {
	if settings.NetworkMode == networkModeDual || settings.NetworkMode == networkModeIPv6 {
		capabilities := m.NetworkCapabilities(ctx)
		if !capabilities.IPv6Available {
			return fmt.Errorf("%w: IPv6 mode is unavailable: %s", ErrInvalidSettings, capabilities.IPv6Detail)
		}
	}
	config, err := settings.Render()
	if err != nil {
		return err
	}
	custom, err = normalizeCustom(custom)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.hostConfigDir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if _, err := m.validateCombined(ctx, settings, custom); err != nil {
		return err
	}

	currentSettings, err := m.Load()
	if err != nil {
		return fmt.Errorf("load current settings: %w", err)
	}
	currentConfig, err := m.activeConfig(currentSettings)
	if err != nil {
		return err
	}
	currentCustom, err := m.LoadCustom()
	if err != nil {
		return err
	}
	if err := m.recordSnapshot(currentSettings, currentConfig, []byte(currentCustom.Content)); err != nil {
		return fmt.Errorf("record current unbound version: %w", err)
	}

	settingsData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	settingsData = append(settingsData, '\n')
	configPath := filepath.Join(m.hostConfigDir, "50-rootguard.conf")
	customPath := filepath.Join(m.hostConfigDir, "90-rootguard-custom.conf")
	settingsPath := filepath.Join(m.hostConfigDir, "settings.json")
	oldConfig, configExisted, err := readOptional(configPath)
	if err != nil {
		return fmt.Errorf("read previous unbound config: %w", err)
	}
	oldSettings, settingsExisted, err := readOptional(settingsPath)
	if err != nil {
		return fmt.Errorf("read previous unbound settings: %w", err)
	}
	oldCustom, customExisted, err := readOptional(customPath)
	if err != nil {
		return fmt.Errorf("read previous custom unbound config: %w", err)
	}

	if err := writeAtomic(configPath, config, 0644); err != nil {
		return fmt.Errorf("activate unbound config: %w", err)
	}
	if err := writeAtomic(settingsPath, settingsData, 0600); err != nil {
		_ = restoreFile(configPath, oldConfig, configExisted, 0644)
		return fmt.Errorf("activate settings: %w", err)
	}
	if err := writeOrRemove(customPath, []byte(custom), 0644); err != nil {
		_ = restoreFile(configPath, oldConfig, configExisted, 0644)
		_ = restoreFile(settingsPath, oldSettings, settingsExisted, 0600)
		return fmt.Errorf("activate custom config: %w", err)
	}

	output, err := m.run(ctx, "docker", "exec", m.containerName, "unbound-checkconf", "/etc/unbound/unbound.conf")
	if err != nil {
		rollbackErr := restoreState(configPath, settingsPath, customPath, oldConfig, oldSettings, oldCustom, configExisted, settingsExisted, customExisted)
		return fmt.Errorf("validate effective unbound config: %w: %s; files restored: %v", err, output, rollbackErr)
	}

	output, err = m.run(ctx, "docker", "restart", m.containerName)
	if err != nil {
		rollbackErr := restoreState(configPath, settingsPath, customPath, oldConfig, oldSettings, oldCustom, configExisted, settingsExisted, customExisted)
		rollbackOutput, restartErr := m.run(ctx, "docker", "restart", m.containerName)
		if rollbackErr != nil || restartErr != nil {
			return fmt.Errorf("restart unbound: %w: %s; rollback failed: %v; rollback restart: %v: %s", err, output, rollbackErr, restartErr, rollbackOutput)
		}
		return fmt.Errorf("restart unbound: %w: %s; previous configuration restored", err, output)
	}
	if err := m.recordSnapshot(settings, config, []byte(custom)); err != nil {
		return fmt.Errorf("record active unbound version: %w", err)
	}
	return nil
}

func writeOrRemove(path string, data []byte, mode os.FileMode) error {
	if len(data) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeAtomic(path, data, mode)
}

func restoreState(configPath, settingsPath, customPath string, config, settings, custom []byte, configExisted, settingsExisted, customExisted bool) error {
	return errors.Join(
		restoreFile(configPath, config, configExisted, 0644),
		restoreFile(settingsPath, settings, settingsExisted, 0600),
		restoreFile(customPath, custom, customExisted, 0644),
	)
}

func (m *Manager) activeConfig(settings Settings) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(m.hostConfigDir, "50-rootguard.conf"))
	if errors.Is(err, os.ErrNotExist) {
		return settings.Render()
	}
	if err != nil {
		return nil, fmt.Errorf("read active unbound config: %w", err)
	}
	return data, nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, mode); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func readOptional(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func restoreFile(path string, data []byte, existed bool, mode os.FileMode) error {
	if !existed {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeAtomic(path, data, mode)
}
