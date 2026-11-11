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
| `ROOTGUARD_DOCKER_PROXY_IMAGE` | Image for the Docker Proxy container. The WebGUI doesn't update it, see [Operations](../operations.md#docker-proxy). |

## Docker connection

| Variable | Description |
| --- | --- |
| `ROOTGUARD_DOCKER_SOCKET_PATH` | Path to the Docker socket on the host, defaults to `/var/run/docker.sock`. Only needed for rootless Docker (typically `/run/user/<uid>/docker.sock`); `install.sh` detects that and sets it itself. |

!!! danger "`ROOTGUARD_SKIP_ATTESTATION` is for tests only"
    `compose.release.yaml` passes this variable to Core so test environments can use locally built, unsigned images. Set to `true`, it turns off **every** signature and provenance check, for setup, updates, and self-update. Never set it on a real installation.

## Update targets

These values are never accepted from the browser, only set server-side in `.env`. For the RootGuard components, Core first asks GitHub for the newest published release and only falls back to these values when that fails. AdGuard Home follows the rule in the table.

| Variable | Description |
| --- | --- |
| `ROOTGUARD_CORE_UPDATE_IMAGE` | Server-side target for Core updates. |
| `ROOTGUARD_WEBAPP_UPDATE_IMAGE` | Server-side target for WebApp updates. |
| `ROOTGUARD_UNBOUND_UPDATE_IMAGE` | Server-side target for Unbound updates. |
| `ROOTGUARD_ADGUARD_UPDATE_IMAGE` | Server-side target for AdGuard updates. An older pin of the same image gives way to the version the running Core ships with. |
| `ROOTGUARD_BLOCKPAGE_UPDATE_IMAGE` | Server-side target for Blockpage updates. |
| `ROOTGUARD_UPDATER_UPDATE_IMAGE` | Server-side target for Updater self-updates. |
| `ROOTGUARD_ATTESTATION_PROXY_UPDATE_IMAGE` | Server-side target for Attestation Proxy updates. |

!!! example "The values you set yourself"
    ```ini title=".env"
    ROOTGUARD_API_TOKEN=replace-with-a-long-random-token
    ROOTGUARD_ADMIN_USER=admin
    ROOTGUARD_ADMIN_PASSWORD=replace-with-a-strong-password
    ROOTGUARD_RECOVERY_TOKEN=replace-with-a-separate-long-random-recovery-key

    ROOTGUARD_WEB_BIND=0.0.0.0
    ROOTGUARD_WEB_PORT=8080
    ```

    Don't write the image lines by hand: each release's [`.env.release.example`](https://github.com/foxly-it/rootguard/blob/main/.env.release.example){: target="_blank" rel="noopener" } template has every image and update target, pinned to verified digests. The [manual installation](../getting-started.md#installation) downloads the one matching the current release.
