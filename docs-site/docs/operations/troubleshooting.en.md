# Common problems

??? question "Port 53 is already in use"
    Preflight detects this automatically - including local resolvers such as systemd-resolved or dnsmasq that aren't visible as Docker containers - and names the cause in the error message. Stop the conflicting service, or use another `ROOTGUARD_DNS_PORT` for testing; router operation normally requires port 53.

??? question "The UI loads, but APIs fail"
    Check `docker compose ps` and the Core and Updater logs. Sign in again after a session expires. A 401 means no valid session; a 502 usually points to an internal service.

??? question "I forgot the administrator password"
    Starting with 0.1.0-alpha.2, choose "Forgot password?" and use the independent `ROOTGUARD_RECOVERY_TOKEN` from your local `.env` file. The new password must contain at least twelve characters; all existing sessions are then signed out. If no recovery key is configured, reset `ROOTGUARD_ADMIN_PASSWORD` locally and recreate only the WebApp container in a controlled manner.

??? question "DNS only works on the host"
    Use the host's LAN IP on clients, not `localhost`. Check the firewall, TCP/UDP 53, and whether the router enforces its own DNS or DoH.

??? question "An update was rolled back"
    Read the message under Stack & Updates and inspect Helper/Core logs. A rollback is a safety mechanism: previous images remain active until the target image or configuration is corrected.
