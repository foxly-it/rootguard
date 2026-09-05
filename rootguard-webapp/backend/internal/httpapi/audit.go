package httpapi

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// auditMaxEvents bounds the persisted log the same way the Stack Center's
// service logs are bounded - a fixed retention count, not unbounded growth,
// so a busy or attacked instance can't turn this into a disk-filling log.
const auditMaxEvents = 500

type auditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	Username  string    `json:"username,omitempty"`
	RemoteIP  string    `json:"remote_ip"`
	Detail    string    `json:"detail,omitempty"`
}

const (
	auditLoginSuccess     = "login_success"
	auditLoginFailure     = "login_failure"
	auditLoginRateLimited = "login_rate_limited"
	auditLogout           = "logout"
	auditRecoverySuccess  = "recovery_success"
	auditRecoveryFailure  = "recovery_failure"
	auditSessionRevoked   = "session_revoked"
	auditAccountUpdated   = "account_updated"
	auditAccountFailure   = "account_update_failure"
	// auditAccountPartial marks the rare double-failure case where the
	// credential write succeeded but the session-invalidation write (and
	// the attempt to roll the credential write back) both failed - a
	// genuinely inconsistent state worth distinguishing from a clean
	// account_update_failure in the audit log.
	auditAccountPartial = "account_update_partial"
)

// Destructive-action base event names. guardDestructive appends "_success",
// "_failure", or "_rate_limited" to whichever of these matches the route
// being guarded - see destructive.go.
const (
	auditUnboundSettingsApplied          = "unbound_settings_applied"
	auditUnboundSettingsRestored         = "unbound_settings_restored"
	auditUnboundImportApplied            = "unbound_import_applied"
	auditUnboundCustomApplied            = "unbound_custom_applied"
	auditUnboundDiagnosticLoggingStarted = "unbound_diagnostic_logging_started"
	auditUnboundDiagnosticLoggingStopped = "unbound_diagnostic_logging_stopped"
	auditServiceAction                   = "service_action"
	auditServiceUpdateStarted            = "service_update_started"
	auditBackupSettingsChanged           = "backup_settings_changed"
	auditCleanupRun                      = "cleanup_run"
	auditBackupExport                    = "backup_export"
	auditBackupRestorePreview            = "backup_restore_preview"
	auditBackupRestore                   = "backup_restore"
	auditControlPlaneUpdateInstall       = "control_plane_update_install"
	auditUpdaterSelfUpdateInstall        = "updater_self_update_install"
	auditInstallationDeploy              = "installation_deploy"
	auditAdGuardBootstrap                = "adguard_bootstrap"
	auditAdGuardFilteringToggled         = "adguard_filtering_toggled"
	auditAdGuardProtectionToggled        = "adguard_protection_toggled"
	auditFritzBoxDiscover                = "fritzbox_discover"
	auditUnboundForwardCheck             = "unbound_forward_check"
	auditReverseDNSDiscover              = "reverse_dns_discover"
)

func (a *SessionAuth) recordAudit(event, username, remoteIP string) {
	a.recordAuditDetail(event, username, remoteIP, "")
}

func (a *SessionAuth) recordAuditDetail(event, username, remoteIP, detail string) {
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	a.auditEvents = append(a.auditEvents, auditEvent{
		Timestamp: time.Now(),
		Event:     event,
		Username:  username,
		RemoteIP:  remoteIP,
		Detail:    detail,
	})
	if len(a.auditEvents) > auditMaxEvents {
		a.auditEvents = a.auditEvents[len(a.auditEvents)-auditMaxEvents:]
	}
	_ = a.persistAuditLocked()
}

func (a *SessionAuth) auditSnapshot() []auditEvent {
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	// Newest first - that's what an operator checking "did something
	// happen recently" wants to see without scrolling.
	snapshot := make([]auditEvent, len(a.auditEvents))
	for i, event := range a.auditEvents {
		snapshot[len(a.auditEvents)-1-i] = event
	}
	return snapshot
}

func (a *SessionAuth) loadAudit() {
	if a.auditPath == "" {
		return
	}
	data, err := os.ReadFile(a.auditPath)
	if err != nil {
		return
	}
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	_ = json.Unmarshal(data, &a.auditEvents)
}

// persistAuditLocked writes the audit log to disk. Its only call site
// (recordAuditDetail) discards the return value entirely ("_ =
// a.persistAuditLocked()", found in review), which on a full disk or
// permissions problem meant the audit trail could silently stop being
// durable while every caller kept reporting success - logged here, at
// the one place that actually knows the write failed.
func (a *SessionAuth) persistAuditLocked() (returnErr error) {
	defer func() {
		if returnErr != nil {
			log.Printf("audit log: failed to persist state: %v", returnErr)
		}
	}()
	if a.auditPath == "" {
		return nil
	}
	data, err := json.Marshal(a.auditEvents)
	if err != nil {
		return err
	}
	return writeAtomicFile(a.auditPath, data, 0600)
}
