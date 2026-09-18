# Rootless Docker daemon compatibility

Tracks the second, host-level phase of the Docker-socket hardening work in
`ROADMAP.md`'s Post-1.0 section, planned right after `rootguard-docker-proxy`
was wired into `compose.release.yaml` (see `docs/threat-model.md`, actor 1).
That phase closed the risk of a compromised Core/Updater reaching the
Docker socket directly; this one asks whether the socket's own holder -
the Docker daemon itself - can run unprivileged too.

**Status: usable, with one non-negotiable setting.** Rootless Docker can
host RootGuard without breaking its core function, but only if the
`pasta` network/port driver is configured explicitly - the default
rootless networking (`slirp4netns` + the `builtin` port driver) silently
discards the real client IP on every DNS query, which would quietly break
AdGuard Home's entire per-client filtering, statistics, and query log.
This was confirmed hands-on, not assumed from documentation. A full
end-to-end deployment of RootGuard's own compose stack under rootless
Docker has not been exercised yet - see "What remains" below - so this
isn't yet a fully supported configuration, but the one requirement that
matters most is now a settled fact, not an open risk.

## The non-negotiable setting: `pasta`, not the default

RootGuard's entire value proposition depends on every DNS query reaching
AdGuard Home with the querying device's real LAN source IP intact.
Rootless Docker's default networking proxies inbound connections through
a user-mode network stack, and there's a real, currently open upstream
report of exactly this failure mode:
[moby/moby#45742](https://github.com/moby/moby/issues/45742) documents a
container seeing the Docker-internal gateway address instead of the real
client IP under rootless Docker with `slirp4netns`. RootlessKit's own
documentation (`docs/port.md` in the `rootless-containers/rootlesskit`
repository) separately states that its "builtin" port driver doesn't
propagate the source IP for UDP at all, only TCP - and DNS is
overwhelmingly UDP.

**Confirmed live** on a real rootless Docker installation (Docker 29.8.1),
sending genuine UDP packets from a separate physical LAN client (not the
Docker host itself - loopback traffic doesn't exercise the code path that
loses the source IP):

| Configuration | Source IP the container observed |
| --- | --- |
| Default (`slirp4netns` + `builtin` port driver) | `172.17.0.1` - the Docker bridge gateway, not the real client |
| `pasta` (`DOCKERD_ROOTLESS_ROOTLESSKIT_NET=pasta`, `DOCKERD_ROOTLESS_ROOTLESSKIT_PORT_DRIVER=implicit`) | The client's real LAN address, correct across two separate test packets |

`pasta` requires Docker Engine 25.0 or later (this host ran 29.8.1). Set
both environment variables above wherever `dockerd-rootless.sh` is
started (a systemd user-unit drop-in, or exported before a manual start) -
**this is not an optional performance tweak, it's a correctness
requirement** for anyone running RootGuard under rootless Docker.

## Privileged port binding (DNS needs port 53)

Independent of the driver choice, rootless Docker also needs an explicit
opt-in to bind ports below 1024, since the rootless daemon runs as an
unprivileged user. Two documented methods (RootlessKit's `docs/port.md`);
the first was hands-on confirmed:

- **Scoped to RootlessKit** (confirmed working): `sudo setcap
  cap_net_bind_service=ep $(which rootlesskit)`, then restart the rootless
  daemon (the capability is read at exec time - an already-running daemon
  won't pick it up). Before this was set, binding port 53 failed with
  Docker's own explicit, actionable error naming both this option and the
  one below; after it, a real container successfully bound a privileged
  port. Narrower than the alternative - grants the capability to one
  binary, not the whole system.
- **System-wide** (documented, not separately tested here): add
  `net.ipv4.ip_unprivileged_port_start=0` to `/etc/sysctl.d/` and run
  `sysctl --system`. Simpler, but relaxes privileged-port binding for
  every unprivileged process on the host, not just Docker.

## What was verified, hands-on, and how

Verification ran on the `.7` test host (Debian 13, Docker 29.8.1) - not a
synthetic environment. Getting there took an unplanned detour worth
recording, since it's a real, recurring class of problem for anyone
running Docker inside a Proxmox LXC container, not a RootGuard-specific
issue:

The host is normally an **unprivileged** Proxmox LXC container. Setting up
rootless Docker there hit four distinct, successive failures, each a
known and separately documented limitation of unprivileged LXC containers
specifically (none of these would occur on a plain bare-metal host or a
real VM):

1. `newuidmap: ... Operation not permitted` - the subordinate UID/GID
   range requested for the test user exceeded what Proxmox delegates to
   an unprivileged LXC by default (65536 IDs total for the whole
   container). Fixed by choosing an in-range allocation instead.
2. `/dev/net/tun: No such file or directory` - Proxmox doesn't pass this
   device into an LXC by default; needs an explicit
   `lxc.cgroup2.devices.allow`/`lxc.mount.entry` pair in the container's
   raw Proxmox config.
3. `ioctl(TUNSETIFF): Operation not permitted` inside RootlessKit's
   `--detach-netns` mode specifically (used by its systemd-managed
   install path), despite the identical operation succeeding in a
   manually created namespace as the same user.
4. `ioctl(TUNSETIFF): Device or resource busy` even without
   `--detach-netns` - confirmed via external research (the Proxmox
   support forum and the LXC project's own issue tracker) as a known,
   unresolved limitation: unprivileged LXC containers cannot reliably
   support the TUN/TAP device manipulation nested Docker networking
   needs, "even with necessary provisions." The only
   documented fix is converting the LXC to a genuinely **privileged**
   container.

With the operator's explicit, informed sign-off, the test host was
converted to a privileged LXC (via Proxmox's backup/restore-with-different-
privilege-flag path, not an in-place toggle, which Proxmox doesn't support
safely). **Once privileged, rootless Docker worked immediately** - `docker
run hello-world`, the privileged-port test, and the `pasta` source-IP test
above all succeeded with no further obstacles. This means points 1-4 are
purely an artifact of nesting rootless Docker inside an *unprivileged*
Proxmox LXC specifically, not a rootless-Docker-in-general problem, and
not something a real RootGuard host (bare metal, a VM, or even a
privileged LXC) would ever hit.

## What remains

- **A full `compose.release.yaml` deployment under rootless Docker** was
  deliberately not attempted, since the only available test host already
  runs the real, live RootGuard installation on the same ports (53, 8080)
  - standing up a second full stack risked interfering with a production
    service for a marginal gain the raw-socket test already covered. The
  wiring itself needs no RootGuard code changes: `rootguard-docker-proxy`
  already exposes `ROOTGUARD_DOCKER_PROXY_SOCKET` as a configurable path
  (see `rootguard-docker-proxy/README.md`), so pointing its volume mount
  at the rootless socket (`/run/user/<uid>/docker.sock` instead of
  `/var/run/docker.sock`) is the only change required - untested, but
  structurally understood.
- **AdGuard Home's own query log**, not just a raw UDP socket, showing the
  correct client IP - the raw-socket test uses the identical `recvfrom()`
  primitive AdGuard's DNS server relies on, but wasn't confirmed through
  AdGuard's own logging on a fully bootstrapped instance.
- **The backup/restore migration path** `ROADMAP.md` commits to (export an
  encrypted backup from an existing rootful installation, install fresh
  under rootless Docker, restore onto it) - the intended shape, not yet
  exercised end to end.

## Recommendation

Rootless Docker is no longer a purely theoretical option for RootGuard -
the one finding that would have ruled it out (client IPs being silently
lost) turned out to have a confirmed, working fix. It isn't yet a fully
supported configuration pending the full-stack and migration-path testing
above, ideally on a dedicated host rather than one already running a live
installation. Anyone trying it before that testing lands must still set
the `pasta` driver explicitly - the default configuration remains actively
unsafe for RootGuard's per-client filtering, confirmed, not assumed.
