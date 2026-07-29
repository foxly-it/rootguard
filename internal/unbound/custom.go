package unbound

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxCustomConfigBytes = 64 << 10

var ErrInvalidCustomConfig = errors.New("invalid custom unbound configuration")

type CustomDocument struct {
	Content  string `json:"content"`
	MaxBytes int    `json:"max_bytes"`
}

type CustomAdvice struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Line        int    `json:"line,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type CustomPreview struct {
	Changed    bool           `json:"changed"`
	Content    string         `json:"content"`
	Validation string         `json:"validation"`
	Advice     []CustomAdvice `json:"advice"`
}

type DirectiveReference struct {
	Name        string `json:"name"`
	Section     string `json:"section"`
	Example     string `json:"example"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

func (m *Manager) LoadCustom() (CustomDocument, error) {
	data, err := os.ReadFile(filepath.Join(m.hostConfigDir, "90-rootguard-custom.conf"))
	if errors.Is(err, os.ErrNotExist) {
		return CustomDocument{MaxBytes: MaxCustomConfigBytes}, nil
	}
	if err != nil {
		return CustomDocument{}, fmt.Errorf("read custom unbound config: %w", err)
	}
	return CustomDocument{Content: string(data), MaxBytes: MaxCustomConfigBytes}, nil
}

func (m *Manager) PreviewCustom(ctx context.Context, content string) (CustomPreview, error) {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()

	normalized, err := normalizeCustom(content)
	if err != nil {
		return CustomPreview{}, err
	}
	settings, err := m.Load()
	if err != nil {
		return CustomPreview{}, err
	}
	validation, err := m.validateCombined(ctx, settings, normalized)
	if err != nil {
		return CustomPreview{}, err
	}
	current, err := m.LoadCustom()
	if err != nil {
		return CustomPreview{}, err
	}
	return CustomPreview{
		Changed:    current.Content != normalized,
		Content:    normalized,
		Validation: validation,
		Advice:     adviseCustom(normalized),
	}, nil
}

func (m *Manager) ApplyCustom(ctx context.Context, content string) (CustomDocument, error) {
	normalized, err := normalizeCustom(content)
	if err != nil {
		return CustomDocument{}, err
	}
	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	settings, err := m.Load()
	if err != nil {
		return CustomDocument{}, err
	}
	if err := m.applyStateLocked(ctx, settings, normalized); err != nil {
		return CustomDocument{}, err
	}
	return CustomDocument{Content: normalized, MaxBytes: MaxCustomConfigBytes}, nil
}

func (m *Manager) validateCombined(ctx context.Context, settings Settings, custom string) (string, error) {
	if err := validateGuidedConflicts(settings, custom); err != nil {
		return "", err
	}
	managed, err := settings.Render()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(m.hostConfigDir, 0755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}
	candidate := filepath.Join(m.hostConfigDir, ".rootguard-combined.candidate")
	combined := append(append(bytes.Clone(managed), '\n'), []byte(custom)...)
	if err := os.WriteFile(candidate, combined, 0644); err != nil {
		return "", fmt.Errorf("write validation candidate: %w", err)
	}
	defer os.Remove(candidate)
	containerCandidate := filepath.Join(m.containerConfigDir, filepath.Base(candidate))
	output, err := m.run(ctx, "docker", "exec", m.containerName, "unbound-checkconf", containerCandidate)
	detail := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("%w: unbound-checkconf: %s", ErrInvalidCustomConfig, detail)
	}
	if detail == "" {
		detail = "unbound-checkconf: no errors"
	}
	return detail, nil
}

func validateGuidedConflicts(settings Settings, custom string) error {
	if len(settings.ForwardZones) > 0 && containsDirective(custom, "forward-zone") {
		return fmt.Errorf("%w: expert forward-zone blocks cannot be combined with guided conditional forwarding", ErrInvalidCustomConfig)
	}
	ownsPrivateDomains := len(settings.PrivateDomains) > 0
	for _, zone := range settings.ForwardZones {
		ownsPrivateDomains = ownsPrivateDomains || zone.AllowPrivateAddresses
	}
	if ownsPrivateDomains && containsDirective(custom, "private-domain") {
		return fmt.Errorf("%w: expert private-domain directives cannot be combined with guided private-domain settings", ErrInvalidCustomConfig)
	}
	for _, policy := range settings.ReverseZones {
		for _, zone := range rfc1918ReverseZones[policy.Network] {
			if containsZoneDirective(custom, "local-zone", zone) {
				return fmt.Errorf("%w: expert local-zone %q conflicts with guided RFC1918 reverse handling", ErrInvalidCustomConfig, zone)
			}
		}
	}
	return nil
}

func containsZoneDirective(content, expected, zone string) bool {
	for _, line := range strings.Split(content, "\n") {
		if directiveKey(line) == expected && strings.Contains(line, `"`+zone+`"`) {
			return true
		}
	}
	return false
}

func containsDirective(content, expected string) bool {
	for _, line := range strings.Split(content, "\n") {
		if directiveKey(line) == expected {
			return true
		}
	}
	return false
}

func normalizeCustom(content string) (string, error) {
	if len(content) > MaxCustomConfigBytes {
		return "", fmt.Errorf("%w: maximum size is %d bytes", ErrInvalidCustomConfig, MaxCustomConfigBytes)
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return "", fmt.Errorf("%w: content must be valid UTF-8 without NUL bytes", ErrInvalidCustomConfig)
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimRight(content, " \t\n")
	if content != "" {
		content += "\n"
	}
	for lineNumber, line := range strings.Split(content, "\n") {
		key := directiveKey(line)
		if reason, blocked := blockedDirectives[key]; blocked {
			return "", fmt.Errorf("%w: line %d: %s (%s)", ErrInvalidCustomConfig, lineNumber+1, key, reason)
		}
	}
	return content, nil
}

func directiveKey(line string) string {
	line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
	if line == "" {
		return ""
	}
	key, _, found := strings.Cut(line, ":")
	if !found {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(key))
}

var blockedDirectives = map[string]string{
	"include":                      "additional file access is not allowed",
	"include-toplevel":             "additional file access is not allowed",
	"chroot":                       "container security is managed by RootGuard",
	"directory":                    "container paths are managed by RootGuard",
	"username":                     "the container user is managed by RootGuard",
	"logfile":                      "file logging is not allowed in the read-only container",
	"pidfile":                      "runtime files are managed by the image",
	"interface":                    "listen addresses are managed by RootGuard",
	"port":                         "the resolver port is managed by RootGuard",
	"remote-control":               "remote control is not exposed by RootGuard",
	"control-enable":               "remote control is not exposed by RootGuard",
	"control-interface":            "remote control is not exposed by RootGuard",
	"server-key-file":              "secret and file paths are managed by RootGuard",
	"server-cert-file":             "secret and file paths are managed by RootGuard",
	"control-key-file":             "secret and file paths are managed by RootGuard",
	"control-cert-file":            "secret and file paths are managed by RootGuard",
	"root-hints":                   "trust bootstrap is managed by the Unbound image",
	"auto-trust-anchor-file":       "DNSSEC trust anchors are managed by the Unbound image",
	"trust-anchor-file":            "DNSSEC trust anchors are managed by the Unbound image",
	"module-config":                "DNSSEC validation modules are managed by the Unbound image",
	"val-permissive-mode":          "DNSSEC validation must not be weakened",
	"tls-service-key":              "TLS listener secrets are managed by RootGuard",
	"tls-service-pem":              "TLS listener secrets are managed by RootGuard",
	"tls-port":                     "resolver listener ports are managed by RootGuard",
	"qname-minimisation":           "use the guided RootGuard setting instead",
	"prefetch":                     "use the guided RootGuard setting instead",
	"prefetch-key":                 "use the guided RootGuard setting instead",
	"aggressive-nsec":              "use the guided RootGuard setting instead",
	"serve-expired":                "use the guided RootGuard setting instead",
	"serve-expired-ttl":            "use the guided RootGuard setting instead",
	"serve-expired-client-timeout": "use the guided RootGuard setting instead",
	"cache-min-ttl":                "use the guided RootGuard setting instead",
	"cache-max-ttl":                "use the guided RootGuard setting instead",
	"num-threads":                  "use the guided RootGuard setting instead",
	"rrset-cache-size":             "use the guided RootGuard resource profile instead",
	"msg-cache-size":               "use the guided RootGuard resource profile instead",
	"do-ip4":                       "use the guided RootGuard network mode instead",
	"do-ip6":                       "use the guided RootGuard network mode instead",
	"prefer-ip6":                   "use the guided RootGuard network mode instead",
}

func adviseCustom(content string) []CustomAdvice {
	advice := make([]CustomAdvice, 0)
	for index, line := range strings.Split(content, "\n") {
		key := directiveKey(line)
		lower := strings.ToLower(strings.TrimSpace(line))
		add := func(id, severity, title, description, suggestion string) {
			advice = append(advice, CustomAdvice{ID: fmt.Sprintf("%s-%d", id, index+1), Severity: severity, Line: index + 1, Title: title, Description: description, Suggestion: suggestion})
		}
		switch {
		case (key == "hide-identity" || key == "hide-version" || key == "harden-glue" || key == "harden-dnssec-stripped") && strings.HasSuffix(lower, ": no"):
			add("hardening-disabled", "warning", "Schutzfunktion deaktiviert", "Diese Zeile schwächt Datenschutz oder DNS-Härtung.", "Nur bei einem nachgewiesenen Kompatibilitätsproblem deaktivieren.")
		case key == "access-control" && strings.Contains(lower, " allow"):
			add("access-control", "warning", "Zusätzlicher Client-Zugriff", "Eine Allow-Regel kann den erreichbaren Resolver-Kreis erweitern.", "Netzbereich eng begrenzen und niemals einen offenen Resolver erzeugen.")
		case key == "forward-addr" || key == "forward-host":
			add("forwarding", "recommendation", "Externer Forwarder konfiguriert", "Abfragen für diese Zone werden an einen festgelegten Resolver weitergegeben.", "Datenschutz, DNSSEC-Verhalten und Verfügbarkeit des Zielservers prüfen.")
		case key == "local-zone" || key == "local-data":
			add("local-data", "success", "Lokale DNS-Regel erkannt", "Lokale Zonen und Antworten verbleiben innerhalb deines RootGuard-Resolvers.", "Regeln mit eindeutigen internen Domainnamen dokumentieren.")
		}
	}
	if len(advice) == 0 {
		advice = append(advice, CustomAdvice{ID: "custom-reviewed", Severity: "success", Title: "Keine offensichtlichen Risiken erkannt", Description: "Die statischen RootGuard-Regeln und unbound-checkconf akzeptieren den Entwurf.", Suggestion: "Auswirkungen auf Auflösung und DNSSEC nach der Aktivierung diagnostizieren."})
	}
	return advice
}

func DirectiveReferences() []DirectiveReference {
	return []DirectiveReference{
		{Name: "server:", Section: "Server", Example: "server:\n", Description: "Beginnt einen Block mit allgemeinen Resolver-Einstellungen.", Risk: "low"},
		{Name: "hide-identity", Section: "Server", Example: "    hide-identity: yes", Description: "Verbirgt die Serverkennung gegenüber DNS-Clients.", Risk: "low"},
		{Name: "hide-version", Section: "Server", Example: "    hide-version: yes", Description: "Verbirgt die installierte Unbound-Version.", Risk: "low"},
		{Name: "harden-glue", Section: "Server", Example: "    harden-glue: yes", Description: "Akzeptiert Glue-Daten nur innerhalb ihres zulässigen Bereichs.", Risk: "low"},
		{Name: "harden-dnssec-stripped", Section: "Server", Example: "    harden-dnssec-stripped: yes", Description: "Behandelt unerwartet entfernte DNSSEC-Daten als Fehler.", Risk: "low"},
		{Name: "aggressive-nsec", Section: "Server", Example: "    aggressive-nsec: yes", Description: "Nutzt validierte NSEC-Antworten zur effizienten Negativauflösung.", Risk: "low"},
		{Name: "rrset-roundrobin", Section: "Server", Example: "    rrset-roundrobin: yes", Description: "Variiert die Reihenfolge gleichwertiger Resource Records.", Risk: "low"},
		{Name: "private-address", Section: "Server", Example: "    private-address: 192.168.0.0/16", Description: "Schützt vor privaten Adressen in öffentlichen DNS-Antworten.", Risk: "medium"},
		{Name: "private-domain", Section: "Server", Example: "    private-domain: \"home.arpa\"", Description: "Erlaubt private Antworten für eine ausdrücklich benannte Zone.", Risk: "medium"},
		{Name: "access-control", Section: "Server", Example: "    access-control: 192.168.1.0/24 allow", Description: "Legt fest, welche Clients den Resolver direkt verwenden dürfen.", Risk: "high"},
		{Name: "local-zone", Section: "Server", Example: "    local-zone: \"home.arpa.\" static", Description: "Definiert eine lokal beantwortete DNS-Zone.", Risk: "medium"},
		{Name: "local-data", Section: "Server", Example: "    local-data: \"router.home.arpa. 300 IN A 192.168.1.1\"", Description: "Fügt einen lokalen DNS-Datensatz hinzu.", Risk: "medium"},
		{Name: "forward-zone:", Section: "Forward Zone", Example: "forward-zone:\n    name: \"corp.example.\"", Description: "Leitet ausschließlich eine bestimmte Zone an andere Resolver weiter.", Risk: "medium"},
		{Name: "name", Section: "Zone", Example: "    name: \"corp.example.\"", Description: "Bestimmt den Namen einer Forward-, Stub- oder Auth-Zone.", Risk: "medium"},
		{Name: "forward-addr", Section: "Forward Zone", Example: "    forward-addr: 192.0.2.53", Description: "Legt die Zieladresse eines Forwarders fest.", Risk: "high"},
		{Name: "forward-tls-upstream", Section: "Forward Zone", Example: "    forward-tls-upstream: yes", Description: "Verwendet DNS-over-TLS für Forward-Ziele.", Risk: "medium"},
		{Name: "stub-zone:", Section: "Stub Zone", Example: "stub-zone:\n    name: \"internal.example.\"", Description: "Delegiert eine Zone an autoritative interne Nameserver.", Risk: "medium"},
		{Name: "stub-addr", Section: "Stub Zone", Example: "    stub-addr: 192.168.1.53", Description: "Legt die Zieladresse eines autoritativen Stub-Servers fest.", Risk: "medium"},
	}
}
