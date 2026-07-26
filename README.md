# 🛡 RootGuard WebApp

![License](https://img.shields.io/badge/license-AGPL--3.0--or--later-blue.svg)
![Backend](https://img.shields.io/badge/backend-Go-00ADD8?logo=go)
![Frontend](https://img.shields.io/badge/frontend-React-61DAFB?logo=react)
![Docker](https://img.shields.io/badge/docker-ready-2496ED?logo=docker)
![Status](https://img.shields.io/badge/status-active--development-orange)

---

## 📌 Overview

RootGuard WebApp is the authenticated HTTP and UI layer of the RootGuard
ecosystem. It contains no direct Docker control code and communicates with
RootGuard Core over an internal token-protected API.

It exposes infrastructure orchestration capabilities provided by **RootGuard Core** and delivers a modern web-based control plane for DNS stack management.

The WebApp is intentionally separated from the engine layer to maintain:

- Clear responsibility boundaries  
- Replaceable transport layers  
- Modular evolution  
- Security isolation  

---

## 🏗 System Architecture

`````
Browser
   ↓
RootGuard WebApp
   ├── REST API (Go)
   └── UI (React + Vite)
   ↓
RootGuard Core (Engine)
   ↓
Docker / System Services
`````

RootGuard WebApp acts purely as a presentation and API transport layer.

The current UI includes service health and lifecycle controls, previewed and
validated Unbound settings with version history, rollback, and diagnostics,
validated operational presets and draft recommendations, and a one-click
AdGuard Home bootstrap that keeps generated credentials inside
RootGuard Core.

The Unbound expert mode manages a separate, versioned custom include with
policy checks, full `unbound-checkconf` validation, contextual directive help,
completion suggestions, advice, and automatic rollback.

The guided Unbound page also displays the actual base, managed, and custom
configuration read from the running resolver. Dashboard status is derived from
the installation, containers, DNSSEC, and protected AdGuard upstream rather
than synthetic metrics.

The **Setup** page is the AIO entry point. It accepts only the DNS bind address
and port, displays Core's non-mutating preflight checks, starts the managed DNS
data plane, and polls the persistent deployment progress. The WebApp still has
no Docker socket and contains no container specification logic.

---

## 📂 Repository Structure

`````
rootguard-webapp/
├── backend/
│   ├── cmd/
│   │   └── rootguard-webapp/
│   ├── internal/
│   │   ├── httpapi/
│   │   └── unboundctl/
│   └── go.mod
│
├── frontend/
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── vite.config.ts
│
└── Dockerfile
`````

---

## 🚀 Local Development

### Backend

````bash
cd backend
ROOTGUARD_CORE_URL=http://localhost:8081 \
ROOTGUARD_API_TOKEN=development-token \
ROOTGUARD_ADMIN_PASSWORD=change-me \
ROOTGUARD_RECOVERY_TOKEN=separate-long-random-recovery-key \
go run ./cmd/rootguard-webapp
````

### Frontend

````bash
cd frontend
npm install
npm run dev
````

---

## 🐳 Containerized Deployment

### Build Image

````bash
docker build -t rootguard-webapp:dev .
````

### Run Container

````bash
docker run -p 8080:8080 \
  -e ROOTGUARD_CORE_URL=http://rootguard-core:8081 \
  -e ROOTGUARD_API_TOKEN=replace-me \
  -e ROOTGUARD_ADMIN_PASSWORD=replace-me \
  -e ROOTGUARD_RECOVERY_TOKEN=separate-long-random-recovery-key \
  rootguard-webapp:dev
````

### Test Endpoints

````bash
curl http://localhost:8080/health
curl -u admin:replace-me http://localhost:8080/api/version
````

---

## 🔐 Design Principles

RootGuard WebApp follows strict design constraints:

- No orchestration logic duplication  
- No direct system-level manipulation  
- API-only exposure of infrastructure actions  
- Minimal runtime surface (distroless container)  
- Deterministic build process  

The login screen provides a local password-recovery flow when
`ROOTGUARD_RECOVERY_TOKEN` is configured. A successful reset stores only a
salted PBKDF2-SHA256 password verifier in the restricted session volume and
invalidates every existing session. The recovery key must be generated
independently from the administrator password and internal API token.

---

## 🛣 Roadmap

Planned development stages:

- DNS filtering and client-management UI
- Extended installer recovery and host-network diagnostics
- Live container monitoring  
- Role management and optional external identity providers
- Multi-architecture image builds  
- GitHub Actions CI/CD  
- GHCR image distribution  

---

## 📜 License

RootGuard WebApp is licensed under the GNU Affero General Public License v3.0
or later (AGPL-3.0-or-later).

See the `LICENSE` file for full details.

---

## ⚠ Development Status

This project is currently under active development.

Breaking changes may occur prior to v1.0.0.
