package backupexport

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestExportCreatesDecryptableChecksummedArchiveAndRemovesStage(t *testing.T) {
	dataDir := t.TempDir()
	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "settings.json"), []byte("settings"), 0600); err != nil {
		t.Fatal(err)
	}
	exporter := New(Options{
		DataDir:      dataDir,
		LocalSources: []Source{{ArchivePath: "rootguard/config", Path: local}},
		ContainerSources: []ContainerSource{{
			ArchivePath: "services/adguard/config", Container: "rootguard-adguard", Path: "/opt/adguardhome/conf",
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			if len(arguments) != 3 || arguments[0] != "cp" || arguments[1] != "rootguard-adguard:/opt/adguardhome/conf" {
				return nil, errors.New("unexpected Docker command")
			}
			if err := os.MkdirAll(arguments[2], 0700); err != nil {
				return nil, err
			}
			return nil, os.WriteFile(filepath.Join(arguments[2], "AdGuardHome.yaml"), []byte("adguard"), 0600)
		},
	})
	var encrypted bytes.Buffer
	const passphrase = "correct horse battery staple"
	if err := exporter.Export(context.Background(), passphrase, &encrypted); err != nil {
		t.Fatal(err)
	}
	entries, manifest := decryptArchive(t, encrypted.Bytes(), passphrase)
	if string(entries["rootguard/config/settings.json"]) != "settings" || string(entries["services/adguard/config/AdGuardHome.yaml"]) != "adguard" {
		t.Fatalf("expected sources missing from archive: %v", entries)
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Format != "tar+gzip+age-scrypt" || len(manifest.Files) != 2 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.Files[0].Path != "rootguard/config/settings.json" || manifest.Files[0].SHA256 == "" {
		t.Fatalf("manifest is not deterministic/checksummed: %+v", manifest.Files)
	}
	remaining, err := os.ReadDir(dataDir)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("plaintext staging directory remained: entries=%v err=%v", remaining, err)
	}
	identity, err := age.NewScryptIdentity("wrong passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := age.Decrypt(bytes.NewReader(encrypted.Bytes()), identity); err == nil {
		t.Fatal("wrong passphrase decrypted backup")
	}
}

func TestExportRejectsShortPassphrasesAndSymlinks(t *testing.T) {
	local := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(local, "unsafe")); err != nil {
		t.Fatal(err)
	}
	exporter := New(Options{DataDir: t.TempDir(), LocalSources: []Source{{ArchivePath: "rootguard/config", Path: local}}})
	if err := exporter.Export(context.Background(), "too-short", io.Discard); !errors.Is(err, ErrInvalidPassphrase) {
		t.Fatalf("expected invalid passphrase, got %v", err)
	}
	if err := exporter.Export(context.Background(), "long-enough-passphrase", io.Discard); err == nil {
		t.Fatal("expected symlink source to be rejected")
	}
}

func decryptArchive(t *testing.T, encrypted []byte, passphrase string) (map[string][]byte, Manifest) {
	t.Helper()
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := age.Decrypt(bytes.NewReader(encrypted), identity)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := gzip.NewReader(decrypted)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewReader(compressed)
	entries := map[string][]byte{}
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		entries[header.Name], err = io.ReadAll(archive)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	return entries, manifest
}
