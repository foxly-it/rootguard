package unbound

import (
	"context"
	"fmt"
	"time"
)

const DiagnosticLoggingDuration = 10 * time.Minute

type DiagnosticLoggingStatus struct {
	Active    bool       `json:"active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Level     int        `json:"level"`
}

func (m *Manager) DiagnosticLoggingStatus() DiagnosticLoggingStatus {
	m.diagnosticMu.Lock()
	defer m.diagnosticMu.Unlock()
	return m.diagnosticStatusLocked()
}

func (m *Manager) StartDiagnosticLogging(ctx context.Context) (DiagnosticLoggingStatus, error) {
	settings, err := m.Load()
	if err != nil {
		return DiagnosticLoggingStatus{}, err
	}
	if output, err := m.run(ctx, "docker", "exec", m.containerName, "unbound-control", "verbosity", "2"); err != nil {
		return DiagnosticLoggingStatus{}, fmt.Errorf("enable temporary Unbound diagnostic logging: %w: %s", err, output)
	}

	m.diagnosticMu.Lock()
	defer m.diagnosticMu.Unlock()
	if m.diagnosticTimer != nil {
		m.diagnosticTimer.Stop()
	}
	expiresAt := m.now().Add(DiagnosticLoggingDuration)
	m.diagnosticExpiresAt = &expiresAt
	m.diagnosticBaseLevel = settings.LogVerbosity
	m.diagnosticTimer = time.AfterFunc(DiagnosticLoggingDuration, m.expireDiagnosticLogging)
	return m.diagnosticStatusLocked(), nil
}

func (m *Manager) StopDiagnosticLogging(ctx context.Context) (DiagnosticLoggingStatus, error) {
	return m.stopDiagnosticLogging(ctx)
}

func (m *Manager) stopDiagnosticLogging(ctx context.Context) (DiagnosticLoggingStatus, error) {
	settings, err := m.Load()
	if err != nil {
		return DiagnosticLoggingStatus{}, err
	}
	if output, err := m.run(ctx, "docker", "exec", m.containerName, "unbound-control", "verbosity", fmt.Sprint(settings.LogVerbosity)); err != nil {
		return DiagnosticLoggingStatus{}, fmt.Errorf("restore privacy-safe Unbound logging: %w: %s", err, output)
	}

	m.diagnosticMu.Lock()
	defer m.diagnosticMu.Unlock()
	if m.diagnosticTimer != nil {
		m.diagnosticTimer.Stop()
		m.diagnosticTimer = nil
	}
	m.diagnosticExpiresAt = nil
	m.diagnosticBaseLevel = settings.LogVerbosity
	return DiagnosticLoggingStatus{Level: settings.LogVerbosity}, nil
}

func (m *Manager) diagnosticStatusLocked() DiagnosticLoggingStatus {
	if m.diagnosticExpiresAt == nil || !m.now().Before(*m.diagnosticExpiresAt) {
		return DiagnosticLoggingStatus{Level: m.diagnosticBaseLevel}
	}
	expiresAt := *m.diagnosticExpiresAt
	return DiagnosticLoggingStatus{Active: true, ExpiresAt: &expiresAt, Level: 2}
}

func (m *Manager) expireDiagnosticLogging() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := m.stopDiagnosticLogging(ctx); err == nil {
		return
	}

	m.diagnosticMu.Lock()
	defer m.diagnosticMu.Unlock()
	retryAt := m.now().Add(time.Minute)
	m.diagnosticExpiresAt = &retryAt
	m.diagnosticTimer = time.AfterFunc(time.Minute, m.expireDiagnosticLogging)
}

func (m *Manager) resetDiagnosticLoggingState(level int) {
	m.diagnosticMu.Lock()
	defer m.diagnosticMu.Unlock()
	if m.diagnosticTimer != nil {
		m.diagnosticTimer.Stop()
		m.diagnosticTimer = nil
	}
	m.diagnosticExpiresAt = nil
	m.diagnosticBaseLevel = level
}
