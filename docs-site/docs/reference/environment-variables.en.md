# Important environment variables

## Secrets and access

| Variable | Description |
| --- | --- |
| `ROOTGUARD_API_TOKEN` | Internal secret shared by WebApp, Core, and Updater. |
| `ROOTGUARD_ADMIN_USER` | Username for the WebGUI; defaults to `admin`. |
| `ROOTGUARD_ADMIN_PASSWORD` | Required strong password for the WebGUI. |
| `ROOTGUARD_RECOVERY_TOKEN` | Separate random key required from 0.1.0-alpha.2 for local password recovery; never reuse the password or API token. |
| `ROOTGUARD_WEB_BIND` | Host binding for the WebGUI, e.g. `0.0.0.0`. |
| `ROOTGUARD_WEB_PORT` | Host port for the WebGUI, e.g. `8080`. |

## Initial-setup images

| Variable | Description |
| --- | --- |
| `ROOTGUARD_CORE_IMAGE` | Image for the Core container. |
| `ROOTGUARD_WEBAPP_IMAGE` | Image for the WebApp container. |
| `ROOTGUARD_UPDATER_IMAGE` | Image for the Updater container. |
| `ROOTGUARD_UNBOUND_IMAGE` | Allowlisted Unbound image for initial setup. |
| `ROOTGUARD_ADGUARD_IMAGE` | Allowlisted AdGuard image for initial setup. |
| `ROOTGUARD_ADGUARD_BETA_IMAGE` | Alternative, also allowlisted AdGuard beta image. |
| `ROOTGUARD_BLOCKPAGE_IMAGE` | Image for the Blockpage container. |
| `ROOTGUARD_ATTESTATION_PROXY_IMAGE` | Image for the Attestation Proxy container. |

## Update targets

These values determine what an update advances to - they are never accepted from the browser, only set server-side in `.env`.

| Variable | Description |
| --- | --- |
| `ROOTGUARD_CORE_UPDATE_IMAGE` | Server-side target for Core updates. |
| `ROOTGUARD_WEBAPP_UPDATE_IMAGE` | Server-side target for WebApp updates. |
| `ROOTGUARD_UNBOUND_UPDATE_IMAGE` | Server-side target for Unbound updates. |
| `ROOTGUARD_ADGUARD_UPDATE_IMAGE` | Server-side target for AdGuard updates. |
| `ROOTGUARD_BLOCKPAGE_UPDATE_IMAGE` | Server-side target for Blockpage updates. |
| `ROOTGUARD_UPDATER_UPDATE_IMAGE` | Server-side target for Updater self-updates. |
| `ROOTGUARD_ATTESTATION_PROXY_UPDATE_IMAGE` | Server-side target for Attestation Proxy updates. |

!!! example "Full example (release stack)"
    ```ini title=".env"
    ROOTGUARD_CORE_IMAGE=ghcr.io/foxly-it/rootguard-core:1.0.0@sha256:2fd43b4943cb5b26daf71e79eb9d6bec8e42d2b1e07d40ab7ed425b065113c41
    ROOTGUARD_WEBAPP_IMAGE=ghcr.io/foxly-it/rootguard-webapp:1.0.0@sha256:fda7b2b50a56a8a95851b59642a6543e76f8bb4dc4104df1b249916f3da2c45e
    ROOTGUARD_UPDATER_IMAGE=ghcr.io/foxly-it/rootguard-updater:1.0.0@sha256:99c9edd94d9f6b5fcb1f52ea291a37076a3803c02d8e4b4c8352e7c85908d9d4
    ROOTGUARD_UNBOUND_IMAGE=ghcr.io/foxly-it/rootguard-unbound:1.0.0@sha256:962d247fc4c125f37e3cac4f7f4c8902814f2d204fb64ceb7ccd1fbde65dfb58
    ROOTGUARD_ADGUARD_IMAGE=adguard/adguardhome:v0.107.79@sha256:aba9e3bf0613be3ba3755e1fc311b126e2c24bec25e18b6483894a88283074f0
    ROOTGUARD_BLOCKPAGE_IMAGE=ghcr.io/foxly-it/rootguard-blockpage:1.0.0@sha256:26ff3e2687797c9bf02ca7969d4288f3bbf6517ff134dc30f484854ba367a097
    ROOTGUARD_ATTESTATION_PROXY_IMAGE=ghcr.io/foxly-it/rootguard-attestation-proxy:1.0.0@sha256:1577ec69a34e0cd465b8ad05cdb0c64c75f684b95058858b1783a65efa2fc400

    ROOTGUARD_API_TOKEN=replace-with-a-long-random-token
    ROOTGUARD_ADMIN_USER=admin
    ROOTGUARD_ADMIN_PASSWORD=replace-with-a-strong-password
    ROOTGUARD_RECOVERY_TOKEN=replace-with-a-separate-long-random-recovery-key

    ROOTGUARD_WEB_BIND=0.0.0.0
    ROOTGUARD_WEB_PORT=8080
    ```

    The full, current template with every update target lives at [`.env.release.example`](https://github.com/foxly-it/rootguard/blob/main/.env.release.example){: target="_blank" rel="noopener" } in the repository root.
