# Rootless Docker daemon compatibility

Tracks the second, host-level phase of the Docker-socket hardening work in
`ROADMAP.md`'s Post-1.0 section, planned right after `rootguard-docker-proxy`
was wired into `compose.release.yaml` (see `docs/threat-model.md`, actor 1).
That phase closed the risk of a compromised Core/Updater reaching the
Docker socket directly; this one asks whether the socket's own holder -
the Docker daemon itself - can run unprivileged too.

**Status: supported, with three settings that aren't optional.** A full
RootGuard stack - control plane, guided setup, AdGuard/Unbound, and the
backup/restore migration path - was deployed and verified end to end
under rootless Docker, hands-on, not just from documentation. Three
things must be true for it to work correctly, all covered below: the
`pasta` network/port driver must be configured, the guided setup's DNS
bind address must be `0.0.0.0` rather than a specific host IP, and port
53 needs `net.ipv4.ip_unprivileged_port_start` lowered rather than a
capability on the RootlessKit/pasta binaries. `install.sh` (since the
change described in `ROADMAP.md`) detects a rootless daemon automatically
and wires the Docker socket path through without requiring any of this -
but the three items above are still the operator's own responsibility,
since they're properties of the host's rootless Docker setup, not
something RootGuard's own compose stack can configure for it.

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

**Confirmed live**, twice, on two different rootless Docker setups
(Docker 29.8.1, sending genuine UDP packets from a separate physical LAN
client - loopback traffic doesn't exercise the code path that loses the
source IP):

| Configuration | Source IP the container observed |
| --- | --- |
| Default, `slirp4netns` + `builtin` port driver (RootlessKit 3.0.x) | `172.17.0.1` - the Docker bridge gateway |
| Default, `gvisor-tap-vsock` + `builtin` port driver (RootlessKit 3.1.0 - the current Debian trixie default) | `172.17.0.1` - same failure, different underlying network driver |
| `pasta` (`DOCKERD_ROOTLESS_ROOTLESSKIT_NET=pasta`, `DOCKERD_ROOTLESS_ROOTLESSKIT_PORT_DRIVER=implicit`) | The client's real LAN address, correct every time, on both RootlessKit versions |

The second row matters: RootlessKit changed its own default network
driver between the first and second round of testing on this same
Debian release, and the failure mode was identical either way - this is
a property of the `builtin` port driver's NAT approach, not any one
specific network driver, so it isn't something a future Docker/
RootlessKit point release is likely to fix on its own. `pasta` requires
Docker Engine 25.0 or later. Set both environment variables above
wherever `dockerd-rootless.sh` is started (a systemd user-unit drop-in,
or exported before a manual start) - **this is not an optional
performance tweak, it's a correctness requirement** for anyone running
RootGuard under rootless Docker.

## The DNS bind address must be `0.0.0.0`, not a specific host IP

RootGuard's guided setup pre-fills the DNS bind address with the specific
IP the WebGUI was opened over (e.g. `192.168.178.7`) - the right default
for a normal, rootful installation, where Docker publishes ports directly
in the host's own network namespace and binding a specific interface
address works exactly as `docker run -p <ip>:53:53` implies.

Under rootless Docker with `pasta`, this fails even after everything
above is configured correctly:

```
failed to bind host port 192.168.178.7:53/tcp: cannot assign requested address
```

**Confirmed live**: the identical `docker run -p <ip>:PORT:PORT` command
that fails against a specific host IP succeeds immediately against
`0.0.0.0` on the same rootless daemon, same port, moments apart. `pasta`
runs port forwarding from inside RootlessKit's own network namespace,
which doesn't have the host's real interface addresses configured on it
directly - only `0.0.0.0` ("accept on every address I forward at all")
resolves inside that namespace the way rootful Docker's host-namespace
binding does. This isn't specific to port 53: a plain, unprivileged test
port hit the identical error against a specific IP and worked
immediately against `0.0.0.0`.

**Practical effect**: when installing RootGuard's guided setup under
rootless Docker, change the pre-filled DNS bind address to `0.0.0.0`
before running preflight - this also applies to the same field when using
the guided restore for a rootless target (the archived config's original
bind address, e.g. from a prior rootful installation, needs the same
override, and restore's own preview step will catch a mismatch and let
it be corrected before applying). This is a real rough edge in the
guided setup's own default, not yet addressed in the wizard itself - see
`ROADMAP.md` for whether a future change teaches the wizard to default to
`0.0.0.0` when it detects a rootless daemon, rather than leaving this to
documentation alone.

## Privileged port binding (DNS needs port 53)

Rootless Docker needs an explicit opt-in to bind ports below 1024, since
the rootless daemon runs as an unprivileged user. RootlessKit's own
`docs/port.md` and `pasta`'s own manual page both cover this, but they
point to **different, incompatible answers** depending on which port
driver is active - a distinction the first round of this verification
missed, since it tested the capability-based method against the
`builtin` port driver, not `pasta`.

- **`net.ipv4.ip_unprivileged_port_start` (confirmed working with
  `pasta`)**: `sysctl -w net.ipv4.ip_unprivileged_port_start=53` (or
  lower) makes port 53 an ordinary, unprivileged port system-wide, so
  `pasta` needs no special capability to bind it at all. This is also
  `pasta`'s own manual page's explicitly *recommended* method, not
  merely an alternative.
- **`setcap cap_net_bind_service=ep` (confirmed NOT sufficient for
  `pasta`'s automatic port forwarding, despite working for the `builtin`
  port driver)**: granting the capability to `rootlesskit` does nothing
  under `port-driver=implicit`, since the actual bind happens in a
  separate `pasta`/`pasta.avx2` child process, not `rootlesskit` itself.
  Granting it to `pasta`/`pasta.avx2` directly (as `pasta`'s own manual
  page shows) *still* doesn't work for automatically-detected ports:
  `pasta`'s manual page states outright that "this will not work for
  automatic detection and forwarding of ports with pasta, because pasta
  will relinquish this capability at runtime" - confirmed live, exactly
  as documented: a container's published port 53 silently never appeared
  as a host-level listening socket with the capability set, on either
  binary, across a clean daemon restart. Port 8080 (unprivileged) worked
  immediately throughout, isolating the failure to the privileged-port
  path specifically, not the driver or the container in general.

**Practical effect**: anyone using `pasta` (the driver this document
already makes non-negotiable) must use the sysctl method for port 53 -
the capability-based method some general rootless-Docker guides
recommend does not apply here. RootlessKit's own docs describe the
capability method as scoped to one binary and therefore preferable in
principle, but that trade-off is moot once `pasta` is in the picture.

## What was verified, hands-on, and how

Verification ran on the `.7` test host (Debian 13, Docker 29.8.1) across
two rounds - not a synthetic environment, and not documentation-only.

### Round 1: the client-IP question, in isolation

The host was, at the time, an **unprivileged** Proxmox LXC container.
Setting up rootless Docker there hit four distinct, successive failures,
each a known and separately documented limitation of unprivileged LXC
containers specifically (none of these would occur on a plain bare-metal
host or a real VM):

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

### Round 2: the full stack, end to end

On the same host, now a standing privileged LXC, a second round exercised
everything Round 1 deliberately deferred:

- A genuinely fresh `rguser` account (standard Debian subuid/subgid
  defaults - the cramped allocation from Round 1 was purely an
  unprivileged-LXC artifact and doesn't apply to a privileged one),
  `dockerd-rootless-setuptool.sh install` under a real, lingering
  (`loginctl enable-linger`) systemd user session, `pasta` configured via
  a `docker.service.d` systemd override.
- `install.sh` (current `main`) run as `rguser` against the rootless
  daemon: the rootless-detection log line and LXC note both printed
  correctly, `ROOTGUARD_DOCKER_SOCKET_PATH` was written to `.env` with
  the real `/run/user/<uid>/docker.sock` path, and the full control-plane
  stack (core, webapp, updater, attestation-proxy, docker-proxy) came up
  healthy - confirmed, via the rootful daemon showing zero RootGuard
  containers and `docker-proxy`'s own mount source, that it was genuinely
  using the rootless socket throughout.
- The guided setup wizard, run against this instance (with the `0.0.0.0`
  bind-address override above), successfully deployed AdGuard Home and
  Unbound. **AdGuard's own query log** - not just the raw-socket
  primitive from Round 1 - was read back via its API after a real `dig`
  query from another LAN host and showed the correct real client IP, the
  one thing Round 1 couldn't confirm on a fully bootstrapped instance.
- **The backup/restore migration path**: an encrypted backup was
  exported from a separate, real rootful installation (with a
  deliberately non-default Unbound setting changed beforehand, to make
  the check meaningful rather than trivial), then restored onto this
  rootless installation via the guided restore flow (with the same
  `0.0.0.0` bind-address override applied to the restore's own config).
  The restore completed successfully, the non-default setting was
  present afterward, and the resulting stack was fully healthy and
  resolved DNS correctly. This is the concrete, now-verified upgrade path
  for an existing rootful installation that wants to move to rootless.
- Along the way, this same end-to-end exercise found two real,
  independent bugs in `rootguard-docker-proxy`'s allowlist - unrelated to
  rootless Docker itself, but only surfaced by actually driving the
  WebGUI's Unbound-settings and cleanup features rather than just
  deploying the stack and stopping there. Both were root-caused and fixed
  (see `rootguard-docker-proxy/README.md` and `CHANGELOG.md`); the fixes
  apply equally to rootful and rootless installations.

## Recommendation

Rootless Docker is a supported configuration for RootGuard. The
client-IP risk that would have ruled it out entirely turned out to have
a confirmed, working fix (`pasta`), and the full stack - guided setup,
per-client filtering confirmed through AdGuard's own query log, and the
backup/restore migration path from an existing rootful installation -
has now been verified end to end, hands-on. Anyone running it needs to
get right, and know is required rather than optional:

1. Configure the `pasta` network/port driver - the default silently
   breaks per-client filtering.
2. Use `0.0.0.0` as the DNS bind address in the guided setup (or guided
   restore), not the specific host IP the wizard pre-fills.
3. Lower `net.ipv4.ip_unprivileged_port_start` to 53 or below - the
   capability-based alternative some general guides suggest does not
   work with `pasta`'s automatic port forwarding.

`install.sh` handles the fourth piece - detecting the rootless daemon and
wiring `docker-proxy`'s socket path - automatically, and surfaces (1) and
a reminder about privileged ports as plain informational log lines when
it detects rootless Docker, without ever installing, configuring, or
recommending one mode over the other.
