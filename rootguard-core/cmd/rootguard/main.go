package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/foxly-it/rootguard-core/internal/adguard"
	"github.com/foxly-it/rootguard-core/internal/api"
	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/backuprestore"
	"github.com/foxly-it/rootguard-core/internal/controlplane"
	"github.com/foxly-it/rootguard-core/internal/installer"
	"github.com/foxly-it/rootguard-core/internal/stack"
	"github.com/foxly-it/rootguard-core/internal/unbound"
	"github.com/foxly-it/rootguard-core/internal/updater"
)

// logPersistError makes an installer/updater manager's otherwise-silent
// state-write failures ("_ = m.persistLocked()" at nearly every call
// site - found in review) visible in the container's own logs, instead
// of only in the return value almost nothing checks. Not a fix for the
// underlying full-disk/permissions problem itself, just the difference
// between "invisible" and "diagnosable" when Status quietly stops
// reflecting reality.
func logPersistError(component string) func(error) {
	return func(err error) {
		log.Printf("%s: failed to persist state: %v", component, err)
	}
}

func main() {
	token := os.Getenv("ROOTGUARD_API_TOKEN")
	if token == "" {
		log.Fatal("ROOTGUARD_API_TOKEN must be set")
	}
	requireSecretStrength("ROOTGUARD_API_TOKEN", token, minTokenLength)

	// Test-only escape hatch, same shape and same reason as
	// rootguard-updater's own ROOTGUARD_UPDATER_SKIP_ATTESTATION: E2E
	// fixtures (backup-restore.yml's "Verify backup export and restore",
	// which builds this binary and Unbound/Blockpage locally from the
	// checkout under test) have no real cosign attestation to check -
	// unlike the updater's own pull-skip flag, this one disables a real
	// security control, so it's worth spelling out plainly: any
	// deployment that sets this in production loses every attestation
	// gate this binary enforces (installation deploy, service updates,
	// updater self-update) entirely. Applies uniformly to every manager
	// below rather than a separate flag per manager - "skip attestation"
	// is a property of this whole binary's enforcement, not of any one
	// call site.
	var attestationVerifier installer.AttestationVerifierFunc
	var updaterAttestationVerifier updater.AttestationVerifierFunc
	if strings.EqualFold(os.Getenv("ROOTGUARD_SKIP_ATTESTATION"), "true") {
		attestationVerifier = func(context.Context, string, string) error { return nil }
		updaterAttestationVerifier = func(context.Context, string, string) error { return nil }
	}

	port := envOrDefault("PORT", "8081")
	manager := unbound.NewManager(
		envOrDefault("UNBOUND_CONFIG_DIR", "/var/lib/rootguard/unbound"),
		envOrDefault("UNBOUND_CONTAINER_CONFIG_DIR", "/etc/unbound/unbound.d"),
		envOrDefault("UNBOUND_CONTAINER_NAME", "rootguard-unbound"),
	)
	// Test-only escape hatch, same shape and same reason as
	// ROOTGUARD_SKIP_ATTESTATION above: /api/unbound/diagnostics queries
	// real internet domains by design (a real deployment genuinely wants
	// to know "can my resolver reach and validate the real internet"),
	// but ci.yml's own integration test of this exact endpoint doesn't
	// want its result depending on the CI runner's own network
	// conditions - found in review, confirmed live: main's own
	// independent, scheduled run of that check failed on a transient
	// DNS hiccup unrelated to any code change. Unset in every real
	// deployment; Diagnose keeps its real-domain defaults unless both
	// are explicitly set.
	manager.SetDiagnosticDomains(
		os.Getenv("ROOTGUARD_UNBOUND_DIAGNOSTIC_RESOLUTION_DOMAIN"),
		os.Getenv("ROOTGUARD_UNBOUND_DIAGNOSTIC_DNSSEC_DOMAIN"),
	)
	adguardManager := adguard.NewManager(
		envOrDefault("ADGUARD_INSTALLER_URL", "http://rootguard-adguard:3000"),
		envOrDefault("ADGUARD_API_URL", "http://rootguard-adguard:80"),
		envOrDefault("ADGUARD_DATA_DIR", "/var/lib/rootguard/adguard"),
		envOrDefault("ADGUARD_UPSTREAM", "rootguard-unbound:5335"),
		envOrDefault("ROOTGUARD_BLOCKPAGE_AUTH_DIR", "/var/lib/rootguard/adguard-auth"),
	)
	installationManager := installer.NewManager(installer.Options{
		DataDir:             envOrDefault("ROOTGUARD_INSTALLATION_DIR", "/var/lib/rootguard/installation"),
		CoreContainer:       envOrDefault("ROOTGUARD_CORE_CONTAINER", "rootguard-core"),
		UnboundImage:        envOrDefault("ROOTGUARD_UNBOUND_IMAGE", "ghcr.io/foxly-it/rootguard-unbound:latest"),
		AdGuardImage:        envOrDefault("ROOTGUARD_ADGUARD_IMAGE", "adguard/adguardhome:v0.107.78"),
		AdGuardBetaImage:    envOrDefault("ROOTGUARD_ADGUARD_BETA_IMAGE", "adguard/adguardhome:beta"),
		BlockpageImage:      envOrDefault("ROOTGUARD_BLOCKPAGE_IMAGE", "ghcr.io/foxly-it/rootguard-blockpage:latest"),
		DNSNetworkCIDR:      "172.29.53.0/24",
		OnPersistError:      logPersistError("installation manager"),
		AttestationVerifier: attestationVerifier,
		Bootstrap: func(ctx context.Context, dnsBindAddress string) error {
			status, err := adguardManager.Bootstrap(ctx, dnsBindAddress)
			if err != nil {
				return err
			}
			if !status.Healthy || !status.UpstreamReady {
				return fmt.Errorf("AdGuard Home bootstrap completed without a healthy protected upstream")
			}
			return nil
		},
	})
	githubClient := &http.Client{Timeout: 8 * time.Second}
	updateManager := updater.NewManager(updater.Options{
		DataDir:             envOrDefault("ROOTGUARD_UPDATE_DIR", "/var/lib/rootguard/updates"),
		ComposeDir:          envOrDefault("ROOTGUARD_INSTALLATION_DIR", "/var/lib/rootguard/installation"),
		OnPersistError:      logPersistError("update manager"),
		AttestationVerifier: updaterAttestationVerifier,
		Services: []updater.ServiceSpec{
			{
				Name: "adguard", DisplayName: "AdGuard Home",
				Container:   "rootguard-adguard",
				TargetImage: envOrDefault("ROOTGUARD_ADGUARD_UPDATE_IMAGE", "adguard/adguardhome:latest"),
				BackupPaths: []string{"/opt/adguardhome/conf", "/opt/adguardhome/work"},
			},
			{
				Name: "unbound", DisplayName: "Unbound",
				Container:   "rootguard-unbound",
				TargetImage: envOrDefault("ROOTGUARD_UNBOUND_UPDATE_IMAGE", "ghcr.io/foxly-it/rootguard-unbound:latest"),
				ResolveTarget: func(ctx context.Context) (string, error) {
					return updater.ResolveLatestReleaseImage(ctx, githubClient, "unbound")
				},
				BackupPaths: []string{"/etc/unbound/unbound.d", "/var/lib/unbound"},
				OwnershipMigrations: []updater.VolumeOwnershipMigration{{
					Volume: "rootguard-unbound-state",
					Path:   "/var/lib/unbound",
					UID:    100,
					GID:    101,
				}},
			},
		},
		Verify: func(ctx context.Context, service string) error {
			switch service {
			case "adguard":
				status, err := adguardManager.Status(ctx)
				if err != nil {
					return err
				}
				if !status.Healthy || !status.UpstreamReady {
					return fmt.Errorf("AdGuard Home is not healthy or its protected upstream changed")
				}
			case "unbound":
				report := manager.Diagnose(ctx)
				if !report.Healthy {
					return fmt.Errorf("unbound diagnostics failed")
				}
			default:
				return fmt.Errorf("unknown service %q", service)
			}
			return nil
		},
	})
	controlPlaneClient := controlplane.NewClient(
		envOrDefault("ROOTGUARD_CONTROL_PLANE_UPDATER_URL", "http://rootguard-updater:8082"),
		envOrDefault("ROOTGUARD_CONTROL_PLANE_UPDATER_TOKEN", token),
	)
	controlPlaneClient.WithTargetResolver("core", func(ctx context.Context) (string, error) {
		return updater.ResolveLatestReleaseImage(ctx, githubClient, "core")
	})
	controlPlaneClient.WithTargetResolver("webapp", func(ctx context.Context) (string, error) {
		return updater.ResolveLatestReleaseImage(ctx, githubClient, "webapp")
	})
	// The RootGuard Updater can't safely replace its own running container
	// (it would kill itself mid-operation), so Core manages that update
	// instead - the same way it already manages AdGuard/Unbound, just
	// against the separate control-plane compose.yaml/project rather than
	// Core's own generated data-plane one.
	controlPlaneComposeFile := envOrDefault("ROOTGUARD_COMPOSE_FILE", "/opt/rootguard/compose.yaml")
	// A single shared manager for both services, not two separate manager
	// instances - found live, 2026-09-04: an earlier version of this code
	// gave the attestation proxy its own parallel manager (reasoning: the
	// install handler and the frontend both assumed exactly one service
	// per manager), but that meant two independent mutexes both driving
	// `docker compose --project-name rootguard ... up -d --no-deps
	// <service>` against the identical compose project with nothing
	// serializing them against each other - a real, UI-triggerable race
	// (interleaved container recreation) that never existed before this
	// component joined self-update management, since composeUp's own
	// project/file were previously only ever touched by one manager at a
	// time. A shared manager's single mutex (StartUpdate already
	// serializes every operation on one Manager, regardless of which
	// service) closes that race as a free side effect - the two
	// assumptions that motivated splitting it were themselves smaller
	// than believed: the install handler already needed generalizing to
	// take a service name either way, and the frontend fix is a one-line
	// `services.find(...)` instead of `services[0]`, the same pattern
	// already used for `updaterRuntime` two lines below it.
	updaterSelfUpdateManager := updater.NewManager(updater.Options{
		DataDir:             envOrDefault("ROOTGUARD_SELF_UPDATE_DIR", "/var/lib/rootguard/updater-self-update"),
		ComposeDir:          filepath.Dir(controlPlaneComposeFile),
		ComposeProject:      envOrDefault("ROOTGUARD_COMPOSE_PROJECT", "rootguard"),
		OnPersistError:      logPersistError("updater self-update manager"),
		AttestationVerifier: updaterAttestationVerifier,
		Services: []updater.ServiceSpec{
			{
				Name: "updater", DisplayName: "RootGuard Updater",
				Container:   "rootguard-updater",
				TargetImage: envOrDefault("ROOTGUARD_UPDATER_UPDATE_IMAGE", "ghcr.io/foxly-it/rootguard-updater:latest"),
				ResolveTarget: func(ctx context.Context) (string, error) {
					return updater.ResolveLatestReleaseImage(ctx, githubClient, "updater")
				},
			},
			{
				// No BackupPaths: the container is read_only and
				// stateless, nothing to snapshot before a swap.
				Name: "attestation-proxy", DisplayName: "Attestation Proxy",
				Container:   "rootguard-attestation-proxy",
				TargetImage: envOrDefault("ROOTGUARD_ATTESTATION_PROXY_UPDATE_IMAGE", "ghcr.io/foxly-it/rootguard-attestation-proxy:latest"),
				ResolveTarget: func(ctx context.Context) (string, error) {
					return updater.ResolveLatestReleaseImage(ctx, githubClient, "attestation-proxy")
				},
			},
		},
		Verify: func(ctx context.Context, service string) error {
			switch service {
			case "updater":
				_, err := controlPlaneClient.Status(ctx)
				return err
			case "attestation-proxy":
				return stack.CheckAttestationProxyReachable()
			default:
				return fmt.Errorf("unknown service %q", service)
			}
		},
	})
	backupExporter := backupexport.New(backupexport.Options{
		DataDir: envOrDefault("ROOTGUARD_EXPORT_DIR", "/var/lib/rootguard/exports"),
		LocalSources: []backupexport.Source{
			{ArchivePath: "rootguard/unbound", Path: envOrDefault("UNBOUND_CONFIG_DIR", "/var/lib/rootguard/unbound")},
			{ArchivePath: "rootguard/adguard", Path: envOrDefault("ADGUARD_DATA_DIR", "/var/lib/rootguard/adguard")},
			{ArchivePath: "rootguard/adguard-auth", Path: envOrDefault("ROOTGUARD_BLOCKPAGE_AUTH_DIR", "/var/lib/rootguard/adguard-auth")},
			{ArchivePath: "rootguard/installation", Path: envOrDefault("ROOTGUARD_INSTALLATION_DIR", "/var/lib/rootguard/installation")},
		},
		ContainerSources: []backupexport.ContainerSource{
			{ArchivePath: "services/adguard/config", Container: "rootguard-adguard", Path: "/opt/adguardhome/conf"},
			{ArchivePath: "services/adguard/work", Container: "rootguard-adguard", Path: "/opt/adguardhome/work"},
			{ArchivePath: "services/unbound/state", Container: "rootguard-unbound", Path: "/var/lib/unbound"},
		},
	})
	backupRestorer := backuprestore.New(backuprestore.Options{
		DataDir:        envOrDefault("ROOTGUARD_EXPORT_DIR", "/var/lib/rootguard/exports"),
		UnboundDir:     envOrDefault("UNBOUND_CONFIG_DIR", "/var/lib/rootguard/unbound"),
		AdGuardDir:     envOrDefault("ADGUARD_DATA_DIR", "/var/lib/rootguard/adguard"),
		AdGuardAuthDir: envOrDefault("ROOTGUARD_BLOCKPAGE_AUTH_DIR", "/var/lib/rootguard/adguard-auth"),
		Installer:      installationManager,
	})
	reconcileContext, reconcileCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := installationManager.Reconcile(reconcileContext); err != nil {
		log.Printf("RootGuard installation reconciliation warning: %v", err)
	}
	reconcileCancel()

	// Run docker stats/inspect in the background on their own cadence
	// instead of per HTTP request - see the dashboardRefreshInterval
	// comment in internal/stack/metrics.go for why. context.Background() is
	// deliberate: these loops are meant to run for the whole process
	// lifetime, same as the HTTP server itself, and need no explicit
	// teardown beyond process exit.
	stack.StartMetricsCollector(context.Background())
	stack.StartStatusCollector(context.Background())

	handler := api.RegisterRoutes(api.Dependencies{
		Token:             token,
		Unbound:           manager,
		AdGuard:           adguardManager,
		Installer:         installationManager,
		Updater:           updateManager,
		ControlPlane:      controlPlaneClient,
		UpdaterSelfUpdate: updaterSelfUpdateManager,
		AdGuardDNSAddress: envOrDefault("ADGUARD_DNS_ADDRESS", "rootguard-adguard:53"),
		BackupExporter:    backupExporter,
		BackupRestorer:    backupRestorer,
	})

	server := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("RootGuard Core API listening on :%s", port)
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-signalContext.Done():
		log.Print("RootGuard Core shutting down")
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("RootGuard Core shutdown error: %v", err)
		}
		if err := <-serverErrors; !errors.Is(err, http.ErrServerClosed) {
			log.Printf("RootGuard Core server error during shutdown: %v", err)
		}
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

const (
	minTokenLength = 32
	// placeholderPrefix matches every secret value in .env.release.example
	// ("replace-with-a-long-random-token", ...) - it's long enough to pass
	// minTokenLength on its own, so an operator who copies the example file
	// without editing it would otherwise start up "successfully" with a
	// publicly known token.
	placeholderPrefix = "replace-with-"
)

// requireSecretStrength exits the process if value is too short or is
// still an unedited .env.release.example placeholder.
func requireSecretStrength(name, value string, minLength int) {
	if strings.HasPrefix(strings.ToLower(value), placeholderPrefix) {
		log.Fatalf("%s is still set to its .env.release.example placeholder value - set a real secret", name)
	}
	if len(value) < minLength {
		log.Fatalf("%s must be at least %d characters", name, minLength)
	}
}
