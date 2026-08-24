package api

import "testing"

func boolPtr(value bool) *bool { return &value }

func TestValidateAdGuardProtectionRequestRejectsMissingEnabled(t *testing.T) {
	// A JSON body of {} decodes to a zero-value bool with no way to tell it
	// apart from an explicit {"enabled":false} - found via code review: that
	// meant an empty request silently disabled protection indefinitely.
	// Enabled must be a *bool so a missing field is distinguishable and
	// rejected here instead.
	if err := validateAdGuardProtectionRequest(nil, 0); err == nil {
		t.Fatal("expected a missing enabled field to be rejected")
	}
}

func TestValidateAdGuardProtectionRequestRejectsDurationWhileEnabling(t *testing.T) {
	// {"enabled":true,"duration_seconds":600} is nonsensical - AdGuard would
	// have rejected it anyway, but as a 502 that looked like a server-side
	// failure rather than the bad client request it actually was.
	if err := validateAdGuardProtectionRequest(boolPtr(true), 600); err == nil {
		t.Fatal("expected a positive duration while enabling to be rejected")
	}
}

func TestValidateAdGuardProtectionRequestRejectsUnofferedDurations(t *testing.T) {
	for _, duration := range []int64{-1, 1, 59, 601, 3601, 86400} {
		if err := validateAdGuardProtectionRequest(boolPtr(false), duration); err == nil {
			t.Fatalf("expected duration_seconds=%d (not one of the offered choices) to be rejected", duration)
		}
	}
}

func TestValidateAdGuardProtectionRequestAcceptsTheOfferedChoices(t *testing.T) {
	for _, duration := range []int64{0, 600, 3600} {
		if err := validateAdGuardProtectionRequest(boolPtr(false), duration); err != nil {
			t.Fatalf("expected duration_seconds=%d to be accepted while disabling, got %v", duration, err)
		}
	}
	if err := validateAdGuardProtectionRequest(boolPtr(true), 0); err != nil {
		t.Fatalf("expected enabling with duration_seconds=0 to be accepted, got %v", err)
	}
}
