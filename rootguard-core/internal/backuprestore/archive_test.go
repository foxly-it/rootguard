package backuprestore

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/installer"
)

const testPassphrase = "correct horse battery staple"

func TestExtractValidatesExportAndReturnsInstalledConfig(t *testing.T) {
	source := t.TempDir()
	status := installer.Status{State: installer.StateInstalled, Config: &installer.Config{DNSBindAddress: "192.0.2.10", DNSPort: 53, AdGuardChannel: "stable"}}
	data, _ := json.Marshal(status)
	if err := os.WriteFile(filepath.Join(source, "status.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	credentials := t.TempDir()
	if err := os.WriteFile(filepath.Join(credentials, "credentials.json"), []byte(`{"username":"rootguard","password":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	adguard := t.TempDir()
	if err := os.WriteFile(filepath.Join(adguard, "AdGuardHome.yaml"), []byte("schema_version: 29"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(adguard, "filters", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adguard, "filters", "nested", "1.txt"), []byte("filter"), 0600); err != nil {
		t.Fatal(err)
	}
	exporter := backupexport.New(backupexport.Options{DataDir: t.TempDir(), LocalSources: []backupexport.Source{
		{ArchivePath: "rootguard/installation", Path: source},
		{ArchivePath: "rootguard/adguard", Path: credentials},
		{ArchivePath: "services/adguard/config", Path: adguard},
	}})
	var encrypted bytes.Buffer
	if err := exporter.Export(context.Background(), testPassphrase, &encrypted); err != nil {
		t.Fatal(err)
	}
	stage, preview, err := Extract(t.TempDir(), testPassphrase, bytes.NewReader(encrypted.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stage)
	if preview.Config.DNSBindAddress != "192.0.2.10" || preview.FileCount != 4 || preview.SchemaVersion != 1 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(stage, "rootguard", "installation", "status.json")); err != nil {
		t.Fatal(err)
	}
}

func TestExtractRejectsWrongPassphraseAndRemovesStage(t *testing.T) {
	dataDir := t.TempDir()
	if _, _, err := Extract(dataDir, strings.Repeat("x", 12), strings.NewReader("not age")); err == nil {
		t.Fatal("expected invalid encrypted input")
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("plaintext stage remained: %v %v", entries, err)
	}
}

func TestExtractRejectsUnsafeAndManifestMismatchedArchives(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		entries map[string]string
	}{
		{name: "unsafe path", entries: map[string]string{"../escape": "bad", "manifest.json": `{}`}},
		{name: "unlisted file", entries: map[string]string{"rootguard/installation/status.json": `{}`, "manifest.json": `{"schema_version":1,"created_at":"2026-08-11T00:00:00Z","format":"tar+gzip+age-scrypt","files":[]}`}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			archive := encryptedTar(t, testCase.entries)
			if _, _, err := Extract(t.TempDir(), testPassphrase, bytes.NewReader(archive)); err == nil {
				t.Fatal("expected archive rejection")
			}
		})
	}
}

// TestExtractAcceptsExactlyTheEntryLimit is the regression test for a
// follow-up review finding: the entry-count guard used to check count
// (starting at 0, incremented per loop iteration before archive.Next() was
// even called) against MaxFiles *before* reading each entry - so once
// exactly MaxFiles entries had already been read successfully, the guard
// fired on the next iteration without ever looking far enough to confirm a
// genuine MaxFiles+1th entry actually exists. An archive with precisely
// MaxFiles entries - at the limit, not over it - was rejected as "too
// many" even though it never had more than the limit allows. Fixed by
// counting (and checking) only after a real entry has actually been read;
// this test builds an archive with exactly MaxFiles entries and confirms
// it's now accepted.
func TestExtractAcceptsExactlyTheEntryLimit(t *testing.T) {
	// Deliberately no manifest.json among these - building a manifest
	// whose file list/hashes match 100000 real extracted entries is a lot
	// of unrelated overhead for what this test actually targets: whether
	// the entry-count guard itself rejects an at-the-limit archive. So the
	// assertion below is narrower than "the whole restore succeeds" - it's
	// "count == MaxFiles never produces the 'too many entries' error" -
	// whatever Extract fails on afterward (here, the always-expected
	// missing manifest) is a separate, already-covered code path.
	entries := make(map[string]string, MaxFiles)
	for i := range MaxFiles {
		entries[fmt.Sprintf("rootguard/installation/%d", i)] = ""
	}
	archive := encryptedTar(t, entries)
	stage, _, err := Extract(t.TempDir(), testPassphrase, bytes.NewReader(archive))
	if stage != "" {
		defer os.RemoveAll(stage)
	}
	if err != nil && strings.Contains(err.Error(), "too many entries") {
		t.Fatalf("expected an archive at exactly the entry limit to pass the count guard, got: %v", err)
	}
}

// TestExtractRejectsEntryCountOverTheLimit is the counterpart to the
// acceptance test above: an archive with one entry more than MaxFiles must
// still be rejected, and only once that genuine extra entry has actually
// been read - with a message that doesn't overclaim ("more than %d
// entries" was the old, inaccurate wording a prior round already
// corrected once).
func TestExtractRejectsEntryCountOverTheLimit(t *testing.T) {
	entries := make(map[string]string, MaxFiles+1)
	for i := range MaxFiles + 1 {
		entries[fmt.Sprintf("rootguard/installation/%d", i)] = ""
	}
	archive := encryptedTar(t, entries)
	_, _, err := Extract(t.TempDir(), testPassphrase, bytes.NewReader(archive))
	if err == nil {
		t.Fatal("expected an archive one entry over the limit to be rejected")
	}
	if strings.Contains(err.Error(), "more than") {
		t.Fatalf("error message overclaims what was actually found: %v", err)
	}
	if !strings.Contains(err.Error(), "too many entries") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func encryptedTar(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var result bytes.Buffer
	recipient, err := age.NewScryptRecipient(testPassphrase)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, _ := age.Encrypt(&result, recipient)
	compressed := gzip.NewWriter(encrypted)
	archive := tar.NewWriter(compressed)
	for name, content := range entries {
		header := &tar.Header{Name: name, Mode: 0600, Size: int64(len(content)), Typeflag: tar.TypeReg, ModTime: time.Unix(0, 0)}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(archive, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := encrypted.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}
