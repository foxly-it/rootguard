package main

import "testing"

// TestFinishOnNoChangeLeavesCurrentImageUntouched is the regression test
// for a review finding: the no_change path passed the resolved
// *candidate* images/IDs into finish() the same way a real, successful
// update does, even though composeUp never ran and nothing was actually
// applied to the running container. finish() unconditionally overwrote
// CurrentImage/CurrentID with whatever was passed in, so a no-op check
// could permanently record a reference that was never actually deployed.
func TestFinishOnNoChangeLeavesCurrentImageUntouched(t *testing.T) {
	specs := []serviceSpec{{Name: "core", DisplayName: "Core", TargetImage: "ghcr.io/foxly-it/rootguard-core:1.0.1"}}
	m := newManager(t.TempDir(), "compose.yaml", "rootguard", specs, nil)
	m.status.Services[0].CurrentImage = "ghcr.io/foxly-it/rootguard-core@sha256:aaa"
	m.status.Services[0].CurrentID = "sha256:aaa"

	candidateImages := map[string]string{"core": "ghcr.io/foxly-it/rootguard-core@sha256:bbb"}
	candidateIDs := map[string]string{"core": "sha256:aaa"}

	m.finish(candidateImages, candidateIDs, "no change", false)

	if got := m.status.Services[0].CurrentImage; got != "ghcr.io/foxly-it/rootguard-core@sha256:aaa" {
		t.Fatalf("expected CurrentImage to stay untouched on a no-op finish, got %q", got)
	}
	if got := m.status.Services[0].CurrentID; got != "sha256:aaa" {
		t.Fatalf("expected CurrentID to stay untouched on a no-op finish, got %q", got)
	}
	// CandidateID/UpdateAvailable still refresh either way - only
	// CurrentImage/CurrentID are guarded by updateCurrent.
	if got := m.status.Services[0].CandidateID; got != "sha256:aaa" {
		t.Fatalf("expected CandidateID to still refresh, got %q", got)
	}
	if m.status.Services[0].UpdateAvailable {
		t.Fatal("expected UpdateAvailable to be cleared")
	}
}

// TestFinishOnRealUpdateSetsCurrentImage is
// TestFinishOnNoChangeLeavesCurrentImageUntouched's counterpart: a real,
// successful update (composeUp did run, updateCurrent=true) must still
// record the newly-applied image/ID.
func TestFinishOnRealUpdateSetsCurrentImage(t *testing.T) {
	specs := []serviceSpec{{Name: "core", DisplayName: "Core", TargetImage: "ghcr.io/foxly-it/rootguard-core:1.0.1"}}
	m := newManager(t.TempDir(), "compose.yaml", "rootguard", specs, nil)
	m.status.Services[0].CurrentImage = "ghcr.io/foxly-it/rootguard-core@sha256:aaa"
	m.status.Services[0].CurrentID = "sha256:aaa"

	candidateImages := map[string]string{"core": "ghcr.io/foxly-it/rootguard-core@sha256:bbb"}
	candidateIDs := map[string]string{"core": "sha256:bbb"}

	m.finish(candidateImages, candidateIDs, "updated", true)

	if got := m.status.Services[0].CurrentImage; got != "ghcr.io/foxly-it/rootguard-core@sha256:bbb" {
		t.Fatalf("expected CurrentImage to reflect the applied update, got %q", got)
	}
	if got := m.status.Services[0].CurrentID; got != "sha256:bbb" {
		t.Fatalf("expected CurrentID to reflect the applied update, got %q", got)
	}
}
