package httpapi

import (
	"encoding/json"
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
}

const (
	auditLoginSuccess     = "login_success"
	auditLoginFailure     = "login_failure"
	auditLoginRateLimited = "login_rate_limited"
	auditLogout           = "logout"
	auditRecoverySuccess  = "recovery_success"
	auditRecoveryFailure  = "recovery_failure"
	auditSessionRevoked   = "session_revoked"
)

func (a *SessionAuth) recordAudit(event, username, remoteIP string) {
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	a.auditEvents = append(a.auditEvents, auditEvent{
		Timestamp: time.Now(),
		Event:     event,
		Username:  username,
		RemoteIP:  remoteIP,
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

func (a *SessionAuth) persistAuditLocked() error {
	if a.auditPath == "" {
		return nil
	}
	data, err := json.Marshal(a.auditEvents)
	if err != nil {
		return err
	}
	temp := a.auditPath + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, a.auditPath)
}
