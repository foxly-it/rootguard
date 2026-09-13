package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/adguard"
	"github.com/foxly-it/rootguard-core/internal/installer"
)

// fakeConfiguredAdGuardServer covers exactly the endpoints Bootstrap needs
// once credentials already exist (loadCredentials succeeds, so install()
// itself is never reached): waitUntilReady, configureUpstream, and the
// trailing Status() call.
func fakeConfiguredAdGuardServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/control/status":
			_ = json.NewEncoder(w).Encode(map[string]any{})
		case "/control/test_upstream_dns":
			_ = json.NewEncoder(w).Encode(map[string]string{"rootguard-unbound:5335": "OK"})
		case "/control/dns_config", "/control/filtering/config":
			w.WriteHeader(http.StatusOK)
		case "/control/dns_info":
			_ = json.NewEncoder(w).Encode(map[string]any{"upstream_dns": []string{"rootguard-unbound:5335"}, "fallback_dns": []string{}})
		case "/control/stats":
			_ = json.NewEncoder(w).Encode(map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
}

// newInstalledInstallerWithBlockpage seeds a status.json that already
// reports state:"installed" with blockpage_enabled:true - the shape
// bootstrapAdGuardHandler reads to decide whether to pass a blockPageIP
// into Bootstrap at all, matching validRestoreArchive's identical
// pre-seeding pattern in backup_restore_test.go.
func newInstalledInstallerWithBlockpage(t *testing.T, run installer.CommandRunner) *installer.Manager {
	t.Helper()
	dataDir := t.TempDir()
	status := `{"state":"installed","config":{"dns_bind_address":"192.0.2.10","dns_port":53,"adguard_channel":"stable","blockpage_enabled":true}}`
	if err := os.WriteFile(filepath.Join(dataDir, "status.json"), []byte(status), 0600); err != nil {
		t.Fatal(err)
	}
	return installer.NewManager(installer.Options{DataDir: dataDir, Run: run})
}

// TestBootstrapAdGuardHandlerReloadsBlockpageAfterRotatingItsToken is the
// regression test for a round-3 correctness-review finding: Bootstrap
// unconditionally rotates blockpage's service token whenever a blockpage
// IP is passed (see adguard.Manager.publishBlockpageServiceToken's own
// doc comment), on the assumption that whoever calls it reloads blockpage
// right after - true for installer.deploy/restoreDeploy, never true for
// this standalone HTTP handler (the AdGuard page's own "finish setup"/
// "apply best practices" button on an already-installed stack), which
// left blockpage authenticating with a token Core had already replaced.
func TestBootstrapAdGuardHandlerReloadsBlockpageAfterRotatingItsToken(t *testing.T) {
	adGuardServer := fakeConfiguredAdGuardServer()
	defer adGuardServer.Close()

	adGuardDataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(adGuardDataDir, "credentials.json"), []byte(`{"username":"rootguard","password":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	manager := adguard.NewManager(adGuardServer.URL, adGuardServer.URL, adGuardDataDir, "rootguard-unbound:5335", t.TempDir())

	var mu sync.Mutex
	var reloadCommands []string
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		reloadCommands = append(reloadCommands, strings.Join(arguments, " "))
		return nil, nil
	}
	installation := newInstalledInstallerWithBlockpage(t, run)

	handler := bootstrapAdGuardHandler(manager, installation)
	request := httptest.NewRequest(http.MethodPost, "/api/adguard/bootstrap", nil)
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	renderCalled, reloadCalled := false, false
	for _, command := range reloadCommands {
		if strings.Contains(command, "19-render-blockpage-conf.sh") {
			renderCalled = true
		}
		if strings.Contains(command, "nginx -s reload") {
			reloadCalled = true
		}
	}
	if !renderCalled || !reloadCalled {
		t.Fatalf("expected blockpage's nginx config to be re-rendered and reloaded after Bootstrap rotated its token, got commands: %v", reloadCommands)
	}
}

// TestBootstrapAdGuardHandlerSkipsReloadWithoutBlockpage confirms the fix
// above doesn't reload blockpage when it was never told to bootstrap it in
// the first place (BlockpageEnabled false) - a blockpage-less install has
// no nginx to reload, and Bootstrap itself skips the token rotation.
func TestBootstrapAdGuardHandlerSkipsReloadWithoutBlockpage(t *testing.T) {
	adGuardServer := fakeConfiguredAdGuardServer()
	defer adGuardServer.Close()

	adGuardDataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(adGuardDataDir, "credentials.json"), []byte(`{"username":"rootguard","password":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	manager := adguard.NewManager(adGuardServer.URL, adGuardServer.URL, adGuardDataDir, "rootguard-unbound:5335", t.TempDir())

	dataDir := t.TempDir()
	status := `{"state":"installed","config":{"dns_bind_address":"192.0.2.10","dns_port":53,"adguard_channel":"stable","blockpage_enabled":false}}`
	if err := os.WriteFile(filepath.Join(dataDir, "status.json"), []byte(status), 0600); err != nil {
		t.Fatal(err)
	}
	var called bool
	installation := installer.NewManager(installer.Options{DataDir: dataDir, Run: func(context.Context, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}})

	handler := bootstrapAdGuardHandler(manager, installation)
	request := httptest.NewRequest(http.MethodPost, "/api/adguard/bootstrap", nil)
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if called {
		t.Fatal("expected no docker command to run when blockpage is disabled")
	}
}
