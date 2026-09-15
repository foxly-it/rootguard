# Useful commands

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
