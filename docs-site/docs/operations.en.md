# Operations

## Useful commands

```shell
# Control plane status
docker compose -f compose.release.yaml ps

# Logs
docker compose -f compose.release.yaml logs --tail=200 core webapp updater

# Restart the control plane
docker compose -f compose.release.yaml restart core webapp updater

# Start a stopped stack again
docker compose -f compose.release.yaml start

# Test DNS
dig @192.168.178.10 example.com A  # replace with your own host IP
```

Configuration and installation state is stored in named Docker volumes. Do not remove volumes with `docker compose down --volumes` unless you intentionally want a clean installation.

!!! warning "Clean reinstall"
    Setup creates additional named DNS volumes outside the control-plane Compose project. A normal stop deletes no data. Remove containers and volumes only intentionally, after a backup, and by following the matching release notes.

## Common problems

??? question "Port 53 is already in use"
    Preflight detects this automatically - including local resolvers such as systemd-resolved or dnsmasq that aren't visible as Docker containers - and names the cause in the error message. Stop the conflicting service, or use another `ROOTGUARD_DNS_PORT` for testing; router operation normally requires port 53.

??? question "The UI loads, but APIs fail"
    Check `docker compose ps` and the Core and Updater logs. Sign in again after a session expires. A 401 means no valid session; a 502 usually points to an internal service.

??? question "I forgot the administrator password"
    Choose "Forgot password?" and use the independent `ROOTGUARD_RECOVERY_TOKEN` from your local `.env` file. The new password must contain at least twelve characters; all existing sessions are then signed out. If no recovery key is configured, reset `ROOTGUARD_ADMIN_PASSWORD` locally and recreate only the WebApp container in a controlled manner.

??? question "DNS only works on the host"
    Use the host's LAN IP on clients, not `localhost`. Check the firewall, TCP/UDP 53, and whether the router enforces its own DNS or DoH.

??? question "An update was rolled back"
    Read the message under Stack & Updates and inspect Helper/Core logs. A rollback is a safety mechanism: previous images remain active until the target image or configuration is corrected.
