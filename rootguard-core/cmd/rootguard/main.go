package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/foxly-it/rootguard-core/internal/adguard"
	"github.com/foxly-it/rootguard-core/internal/api"
	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/backuprestore"
	"github.com/foxly-it/rootguard-core/internal/controlplane"
	"github.com/foxly-it/rootguard-core/internal/installer"
	"github.com/foxly-it/rootguard-core/internal/unbound"
	"github.com/foxly-it/rootguard-core/internal/updater"
)

func main() {
	token := os.Getenv("ROOTGUARD_API_TOKEN")
	if token == "" {
		log.Fatal("ROOTGUARD_API_TOKEN must be set")
	}

	port := envOrDefault("PORT", "8081")
	manager := unbound.NewManager(
		envOrDefault("UNBOUND_CONFIG_DIR", "/var/lib/rootguard/unbound"),
		envOrDefault("UNBOUND_CONTAINER_CONFIG_DIR", "/etc/unbound/unbound.d"),
		envOrDefault("UNBOUND_CONTAINER_NAME", "rootguard-unbound"),
	)
	adguardManager := adguard.NewManager(
		envOrDefault("ADGUARD_INSTALLER_URL", "http://rootguard-adguard:3000"),
		envOrDefault("ADGUARD_API_URL", "http://rootguard-adguard:80"),
		envOrDefault("ADGUARD_DATA_DIR", "/var/lib/rootguard/adguard"),
		envOrDefault("ADGUARD_UPSTREAM", "rootguard-unbound:5335"),
		envOrDefault("ROOTGUARD_BLOCKPAGE_AUTH_DIR", "/var/lib/rootguard/adguard-auth"),
	)
	installationManager := installer.NewManager(installer.Options{
		DataDir:          envOrDefault("ROOTGUARD_INSTALLATION_DIR", "/var/lib/rootguard/installation"),
		CoreContainer:    envOrDefault("ROOTGUARD_CORE_CONTAINER", "rootguard-core"),
		UnboundImage:     envOrDefault("ROOTGUARD_UNBOUND_IMAGE", "ghcr.io/foxly-it/rootguard-unbound:latest"),
		AdGuardImage:     envOrDefault("ROOTGUARD_ADGUARD_IMAGE", "adguard/adguardhome:v0.107.78"),
		AdGuardBetaImage: envOrDefault("ROOTGUARD_ADGUARD_BETA_IMAGE", "adguard/adguardhome:beta"),
		BlockpageImage:   envOrDefault("ROOTGUARD_BLOCKPAGE_IMAGE", "ghcr.io/foxly-it/rootguard-blockpage:latest"),
		DNSNetworkCIDR:   "172.29.53.0/24",
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
		DataDir:    envOrDefault("ROOTGUARD_UPDATE_DIR", "/var/lib/rootguard/updates"),
		ComposeDir: envOrDefault("ROOTGUARD_INSTALLATION_DIR", "/var/lib/rootguard/installation"),
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

	handler := api.RegisterRoutes(api.Dependencies{
		Token:             token,
		Unbound:           manager,
		AdGuard:           adguardManager,
		Installer:         installationManager,
		Updater:           updateManager,
		ControlPlane:      controlPlaneClient,
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
