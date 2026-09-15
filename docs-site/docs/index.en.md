# What RootGuard provides

RootGuard combines AdGuard Home as a network filter with Unbound as a dedicated recursive, DNSSEC-validating resolver. WebApp and Core manage the stack; a separate updater safely replaces Core and WebApp as a pair.

## Six components, clear responsibilities

| Component | Role | Description |
| --- | --- | --- |
| [WebApp](https://github.com/foxly-it/rootguard/tree/main/rootguard-webapp) | UI | Login, dashboard, guided controls, and narrow API proxy |
| [Core](https://github.com/foxly-it/rootguard/tree/main/rootguard-core) | CONTROL | Orchestration, configuration, validation, and DNS lifecycle |
| [Updater](https://github.com/foxly-it/rootguard/tree/main/rootguard-updater) | UPDATE | Atomic replacement of Core and WebApp with paired rollback |
| [Unbound](https://github.com/foxly-it/rootguard/tree/main/rootguard-unbound) | DNS | Hardened recursive and DNSSEC-validating resolver |
| [Blockpage](https://github.com/foxly-it/rootguard/tree/main/rootguard-blockpage) | UI | Dedicated, plain-language block page instead of AdGuard's default response |
| [Attestation Proxy](https://github.com/foxly-it/rootguard/tree/main/rootguard-attestation-proxy) | TRUST | Narrow egress path for signed release checks, itself managed via its own update channel |

!!! tip "Getting started"
    New here? Continue with [Requirements](getting-started/requirements.md) and [Installation](getting-started/installation.md).
