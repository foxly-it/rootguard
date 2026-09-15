# Two separate safety paths

## AdGuard Home, Unbound, and the block page

Core pulls only configured target images, compares actual image IDs, backs up persistent service paths, and replaces exactly one service. DNS, DNSSEC, and upstream are then verified. Failures trigger data and image rollback.

## Core and WebApp

The separate updater remains alive during replacement. It updates Core and WebApp only as a pair, verifies both image IDs and health endpoints, and pins both previous images on failure. Browser requests cannot choose image names or Compose arguments.

!!! warning "Updating from 0.1.0-beta.14 or earlier"
    Core versions up to and including 0.1.0-beta.14 don't automatically detect 1.0.0 as an available update - version detection was hard-limited to the old `0.1.0-(alpha|beta).N` scheme. Existing installations on that version need a one-time manual pointer to the new release; the updated Core then detects every following release normally again.

Run this once on the host RootGuard is running on (`ROOTGUARD_API_TOKEN` is the value from your `.env`):

```shell
docker exec rootguard-core wget -qO- \
  --header="Authorization: Bearer $ROOTGUARD_API_TOKEN" \
  --header="Content-Type: application/json" \
  --post-data='{"target_images":{"core":"ghcr.io/foxly-it/rootguard-core:1.0.0","webapp":"ghcr.io/foxly-it/rootguard-webapp:1.0.0"}}' \
  http://updater:8082/api/control-plane/update
```

!!! success "Real rollback test"
    Updater CI replaces real Core and WebApp fixture containers as a pair. A deliberately unhealthy WebApp candidate returns HTTP 503; the test then proves both previous running image IDs and the persisted rollback history.

## History and bounded cleanup

The Stack Center persistently retains up to 50 update, failure, rollback, and cleanup events. The manual inventory is loaded only after an explicit click and shows only older image IDs recorded by RootGuard and unused volumes labeled `io.rootguard.cleanup=true`, together with estimated reclaimable space. RootGuard checks the selection again before confirmed removal and never uses global Docker prune commands.

Internal AdGuard and Unbound update backups are protected separately. The dedicated Backups page shows their count and storage use and retains a configurable 2 to 50 restore points per service (default 5). RootGuard deletes only its own backups recognized by canonical path and manifest. Unknown data and symlinks remain visible but are never removed.

For external recovery, the Backups page creates a passphrase-encrypted age-v1 full backup with a versioned manifest and SHA-256 checksums. It contains RootGuard configuration plus persistent AdGuard/Unbound data, but no browser sessions or external `.env` secrets. The passphrase is never stored.

The same encrypted archive can be validated and restored on a clean RootGuard installation. Archive bounds, manifest, checksums, target address, port, and existing Docker resources are rechecked before any change; failed attempts remove the resources they created.

The central Logs & Diagnostics page reads only the five explicitly allowlisted services. Core limits every response to the last 30 minutes, 100 lines, and 64 KiB, removes control characters, and redacts common credential patterns; the browser cannot provide arbitrary container or path names.

For immutable Core and WebApp releases, RootGuard also verifies signed SLSA provenance, the expected GitHub workflow identity, and Sigstore transparency data. Missing or invalid attestations and temporary network failures are displayed separately.

!!! note "Immutable release images"
    The public release stack combines readable version tags with verified multi-architecture digests. Docker therefore starts the exact published artifact even if a tag were changed later.

```ini title=".env"
ROOTGUARD_CORE_UPDATE_IMAGE=ghcr.io/foxly-it/rootguard-core:1.0.0@sha256:2fd43b4943cb5b26daf71e79eb9d6bec8e42d2b1e07d40ab7ed425b065113c41
ROOTGUARD_WEBAPP_UPDATE_IMAGE=ghcr.io/foxly-it/rootguard-webapp:1.0.0@sha256:fda7b2b50a56a8a95851b59642a6543e76f8bb4dc4104df1b249916f3da2c45e
```
