package main

import "regexp"

// rule describes one allowed Docker Engine API call: an exact method and
// an (unversioned) path pattern. validate is nil for calls that only
// operate on an already-identified, existing resource (inspect, ps, pull,
// restart, cp, version, compose's own network/volume bookkeeping) - the
// path+method allowlist alone is enough there, since none of them grant
// new capability. validate is set for the three calls that DO grant new
// capability (create a container, create an exec instance, attach a
// container to a network) - see validate.go for what each one rejects.
type rule struct {
	method   string
	pattern  *regexp.Regexp
	validate bodyValidator
}

// apiVersionPrefix strips the docker CLI's optional negotiated API version
// prefix (e.g. "/v1.51/containers/json") before matching against the
// patterns below, which are all written unversioned.
var apiVersionPrefix = regexp.MustCompile(`^/v[0-9]+\.[0-9]+`)

func normalizePath(path string) string {
	return apiVersionPrefix.ReplaceAllString(path, "")
}

// rules is the complete allowlist. Grouped and commented by which real
// call site(s) need it - every entry traces back to an actual `docker`/
// `docker compose` subcommand this repo's own code issues today
// (rootguard-core/internal/dockercli, rootguard-updater/docker.go), not a
// guess at what Docker's API offers in general.
//
// The `docker compose`-internal entries (networks/volumes create+list,
// container start/wait/remove) are this proxy's best-effort modeling of
// what `docker compose up`/`pull`/`create` need under the hood, reasoned
// from the public Docker Engine API rather than an observed packet
// capture - flagged here deliberately rather than presented as verified.
// They need a real live check against actual guided-setup-deploy and
// self-update traffic once this proxy is wired into compose.release.yaml
// (tracked as the explicit next step, not assumed done by this change).
var rules = []rule{
	// docker version / _ping - the CLI negotiates the API version and
	// checks connectivity before every real command it runs.
	{method: "GET", pattern: regexp.MustCompile(`^/version$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/_ping$`)},
	{method: "HEAD", pattern: regexp.MustCompile(`^/_ping$`)},

	// Found live wiring this proxy into the real stack (a real guided-setup
	// deploy and self-update run, exactly the validation the package doc
	// comment said was still outstanding): `docker compose` itself queries
	// daemon capabilities via GET /info before certain operations, and
	// polls each service's GET /containers/{id}/stats for its own startup
	// progress display - neither is issued by RootGuard's own code
	// directly, both are read-only and grant no capability.
	{method: "GET", pattern: regexp.MustCompile(`^/info$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/containers/[^/]+/stats$`)},

	// docker pull / docker compose pull (Core + Updater)
	{method: "POST", pattern: regexp.MustCompile(`^/images/create$`), validate: validateImageCreate},

	// docker image inspect, docker inspect, docker ps (Core + Updater).
	// Image references legitimately contain slashes (a registry host plus
	// path segments, e.g. "ghcr.io/foxly-it/rootguard-core") - unlike a
	// container/network/volume name or ID, so this one can't use `[^/]+`.
	{method: "GET", pattern: regexp.MustCompile(`^/images/.+/json$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/containers/[^/]+/json$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/containers/json$`)},

	// docker cp, both directions (Core: backup export/restore, update rollback).
	// HEAD is docker cp's own preflight - found live wiring this proxy
	// into the real stack: the CLI checks the target path's existence/
	// mode via HEAD before the real GET/PUT, read-only and grants nothing
	// beyond what GET already would.
	{method: "GET", pattern: regexp.MustCompile(`^/containers/[^/]+/archive$`)},
	{method: "HEAD", pattern: regexp.MustCompile(`^/containers/[^/]+/archive$`)},
	{method: "PUT", pattern: regexp.MustCompile(`^/containers/[^/]+/archive$`)},

	// docker restart (Core: backup restore)
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/restart$`)},
	// docker compose down's own per-service stop, issued before removal -
	// found live wiring this proxy into the real stack (Core's backup-
	// restore cleanup path runs `compose ... down --volumes
	// --remove-orphans`, installer/manager.go). Grants nothing beyond
	// what restart already does to an existing container.
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/stop$`)},

	// docker run (Core: one-off chown-helper and self-image-verification
	// containers) and docker compose up's own per-service container
	// creation - the one call that can request a privileged container or
	// an arbitrary host bind mount, hence the body validator.
	{method: "POST", pattern: regexp.MustCompile(`^/containers/create$`), validate: validateContainerCreate},
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/start$`)},
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/wait$`)},
	{method: "DELETE", pattern: regexp.MustCompile(`^/containers/[^/]+$`)},

	// docker run without -d (Core's chown-helper and port-probe containers,
	// installer/manager.go and updater/manager.go) attaches to relay the
	// container's own output back to the CLI's stdout - found live wiring
	// this proxy into the real stack. Body-validated: an attach that also
	// requests stdin would let the caller write arbitrary data into the
	// container's stdin stream, which none of these fixed, non-interactive
	// entrypoints (chown/stat/true) need and none of Core's own `docker
	// run` invocations request (-i is never passed).
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/attach$`), validate: validateAttach},

	// docker exec (Core only: reloading rootguard-blockpage, plus
	// rootguard-unbound's own config-syntax checks and diagnostics) - the
	// second capability-granting call, hence the body validator.
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/exec$`), validate: validateExecCreate},
	{method: "POST", pattern: regexp.MustCompile(`^/exec/[^/]+/start$`)},
	// docker exec's own exit-code check after running - found live wiring
	// this proxy into the real stack. Read-only, grants nothing: {id} here
	// is an exec instance ID only reachable by having already passed
	// validateExecCreate above.
	{method: "GET", pattern: regexp.MustCompile(`^/exec/[^/]+/json$`)},

	// docker network connect (Core only: joining rootguard-dns) - the
	// third capability-granting call.
	{method: "POST", pattern: regexp.MustCompile(`^/networks/[^/]+/connect$`), validate: validateNetworkConnect},
	// docker network disconnect (Core: backup-restore cleanup, detaching
	// itself from rootguard-dns before recreating it) - found live wiring
	// this proxy into the real stack. Unlike connect, disconnect only
	// ever removes an existing association, never grants one, so no body
	// validator is needed.
	{method: "POST", pattern: regexp.MustCompile(`^/networks/[^/]+/disconnect$`)},

	// docker compose's own network/volume bookkeeping for the guided-setup
	// DNS-stack bootstrap (create/pull/create in installer/manager.go) -
	// see the package doc comment above on verification status.
	{method: "GET", pattern: regexp.MustCompile(`^/networks$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/networks/[^/]+$`)},
	{method: "POST", pattern: regexp.MustCompile(`^/networks/create$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/volumes$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/volumes/[^/]+$`)},
	{method: "POST", pattern: regexp.MustCompile(`^/volumes/create$`)},

	// docker volume ls/rm (Updater only: post-update cleanup of orphaned
	// volumes labeled io.rootguard.cleanup=true)
	{method: "DELETE", pattern: regexp.MustCompile(`^/volumes/[^/]+$`)},

	// docker image rm (Updater only: post-update cleanup of the old image)
	{method: "DELETE", pattern: regexp.MustCompile(`^/images/[^/]+$`)},
}

func matchRule(method, path string) (rule, bool) {
	normalized := normalizePath(path)
	for _, r := range rules {
		if r.method == method && r.pattern.MatchString(normalized) {
			return r, true
		}
	}
	return rule{}, false
}
