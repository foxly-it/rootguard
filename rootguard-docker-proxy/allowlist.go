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

	// docker pull / docker compose pull (Core + Updater)
	{method: "POST", pattern: regexp.MustCompile(`^/images/create$`), validate: validateImageCreate},

	// docker image inspect, docker inspect, docker ps (Core + Updater).
	// Image references legitimately contain slashes (a registry host plus
	// path segments, e.g. "ghcr.io/foxly-it/rootguard-core") - unlike a
	// container/network/volume name or ID, so this one can't use `[^/]+`.
	{method: "GET", pattern: regexp.MustCompile(`^/images/.+/json$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/containers/[^/]+/json$`)},
	{method: "GET", pattern: regexp.MustCompile(`^/containers/json$`)},

	// docker cp, both directions (Core: backup export/restore, update rollback)
	{method: "GET", pattern: regexp.MustCompile(`^/containers/[^/]+/archive$`)},
	{method: "PUT", pattern: regexp.MustCompile(`^/containers/[^/]+/archive$`)},

	// docker restart (Core: backup restore)
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/restart$`)},

	// docker run (Core: one-off chown-helper and self-image-verification
	// containers) and docker compose up's own per-service container
	// creation - the one call that can request a privileged container or
	// an arbitrary host bind mount, hence the body validator.
	{method: "POST", pattern: regexp.MustCompile(`^/containers/create$`), validate: validateContainerCreate},
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/start$`)},
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/wait$`)},
	{method: "DELETE", pattern: regexp.MustCompile(`^/containers/[^/]+$`)},

	// docker exec (Core only: reloading rootguard-blockpage) - the second
	// capability-granting call, hence the body validator.
	{method: "POST", pattern: regexp.MustCompile(`^/containers/[^/]+/exec$`), validate: validateExecCreate},
	{method: "POST", pattern: regexp.MustCompile(`^/exec/[^/]+/start$`)},

	// docker network connect (Core only: joining rootguard-dns) - the
	// third capability-granting call.
	{method: "POST", pattern: regexp.MustCompile(`^/networks/[^/]+/connect$`), validate: validateNetworkConnect},

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
