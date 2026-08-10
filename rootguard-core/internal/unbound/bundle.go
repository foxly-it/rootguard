package unbound

import (
	"context"
	"errors"
	"time"
)

// BundleSchemaVersion identifies the shape of ConfigBundle. Bump it whenever
// a field is added, removed, or reinterpreted so an older RootGuard release
// can reject a bundle it can't apply safely instead of silently misreading it.
const BundleSchemaVersion = 1

var ErrIncompatibleBundle = errors.New("incompatible configuration bundle")

// ConfigBundle is the complete logical resolver configuration - guided
// settings and expert custom config together - as a single portable unit for
// backup or migration between RootGuard instances. Distinct from the
// per-version snapshots in HistoryEntry, which exist for in-place rollback
// rather than export.
type ConfigBundle struct {
	SchemaVersion int       `json:"schema_version"`
	ExportedAt    time.Time `json:"exported_at"`
	Settings      Settings  `json:"settings"`
	CustomConfig  string    `json:"custom_config"`
}

type BundlePreview struct {
	Changed        bool     `json:"changed"`
	Changes        []Change `json:"changes"`
	CustomChanged  bool     `json:"custom_changed"`
	RenderedConfig string   `json:"rendered_config"`
}

func (m *Manager) Export() (ConfigBundle, error) {
	settings, err := m.Load()
	if err != nil {
		return ConfigBundle{}, err
	}
	custom, err := m.LoadCustom()
	if err != nil {
		return ConfigBundle{}, err
	}
	return ConfigBundle{
		SchemaVersion: BundleSchemaVersion,
		ExportedAt:    m.now().UTC(),
		Settings:      settings,
		CustomConfig:  custom.Content,
	}, nil
}

// PreviewBundle validates settings and custom together, as the pair they'll
// become on import - unlike Preview/PreviewCustom, which each validate their
// argument against the OTHER side's currently active value and would
// spuriously reject an import that changes both sides at once to a state
// that's only mutually consistent once both land together.
func (m *Manager) PreviewBundle(ctx context.Context, settings Settings, custom string) (BundlePreview, error) {
	m.applyMu.Lock()
	defer m.applyMu.Unlock()

	normalized, err := normalizeCustom(custom)
	if err != nil {
		return BundlePreview{}, err
	}
	config, err := settings.Render()
	if err != nil {
		return BundlePreview{}, err
	}
	if _, err := m.validateCombined(ctx, settings, normalized); err != nil {
		return BundlePreview{}, err
	}
	currentSettings, err := m.Load()
	if err != nil {
		return BundlePreview{}, err
	}
	currentCustom, err := m.LoadCustom()
	if err != nil {
		return BundlePreview{}, err
	}
	changes := settingsChanges(currentSettings, settings)
	return BundlePreview{
		Changed:        len(changes) > 0,
		Changes:        changes,
		CustomChanged:  currentCustom.Content != normalized,
		RenderedConfig: string(config),
	}, nil
}

// ClassifyImport parses content as an unbound.conf and classifies its
// directives against the currently active settings - a read-only preview
// step. The caller turns the result into a ConfigBundle and runs it through
// PreviewBundle/ApplyBundle like any other import, which is where real
// unbound-checkconf validation happens.
//
// The expert-adoptable portion is always computed against an EMPTY starting
// custom config, not whatever's currently active: the imported file is
// meant to represent a complete desired configuration, not a patch to layer
// onto the existing one. Starting from the active custom config instead
// would make re-classifying the same (or an overlapping) file after an
// earlier activation silently double every already-adopted directive - and
// since a reconstructed clause block like forward-zone: doesn't get a
// trailing clause header to close it, unbound's context-carries-until-the-
// next-header grammar can turn the second copy's server: directives into a
// genuine syntax error inside that clause. Guided settings still build on
// top of the currently active Settings (see ImportUnboundConf) - only the
// custom-config side is reset per classification.
func (m *Manager) ClassifyImport(content string) (ImportResult, error) {
	settings, err := m.Load()
	if err != nil {
		return ImportResult{}, err
	}
	return ImportUnboundConf(settings, "", content)
}

func (m *Manager) ApplyBundle(ctx context.Context, settings Settings, custom string) error {
	normalized, err := normalizeCustom(custom)
	if err != nil {
		return err
	}
	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	return m.applyStateLocked(ctx, settings, normalized)
}
