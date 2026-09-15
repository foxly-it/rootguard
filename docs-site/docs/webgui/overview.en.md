# Interface sections

| Section | Description |
| --- | --- |
| **Dashboard** | Installation, DNS endpoint, services, DNSSEC, and connection chain plus real CPU/memory and aggregated AdGuard metrics at a glance. |
| **Setup** | Network preflight and persistent AIO deployment progress. |
| **Stack & Updates** | Service state, safe updates, and history remain immediately visible; technical release details are collapsible and protected Docker cleanup is inspected only on request. |
| **Backups** | Restore points, encrypted export, and clean-install full recovery. A chooser also links directly to RootGuard Unbound bundles or importing an existing `unbound.conf`. |
| **Logs & Diagnostics** | Central searchable logs for every managed service with local filtering, optional refresh, and a redacted diagnostic report. |
| **Unbound** | Profiles, guided settings, zones, live configuration and directives in large detail views, expert editor, and rollback. |
| **AdGuard Home** | RootGuard status and protected access to the native AdGuard interface. |

## Live metrics

The dashboard aggregates CPU and memory usage only for the five explicitly allowlisted RootGuard containers. Core also uses AdGuard Home's internally authenticated API to read total DNS queries and blocked requests and derives the filter rate. Query names and client data do not leave AdGuard. The display refreshes every ten seconds and explicitly marks unavailable measurements.
