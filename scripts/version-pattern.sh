#!/usr/bin/env bash
# Shared SemVer tag-recognition helpers for check-site-facts.sh and
# bump-site-versions.sh. Sourced, not executed. install.sh can't source
# this - it runs standalone on a fresh machine with no repo checkout -
# and keeps its own inline copy of the same grammar; keep both in sync by
# hand if this ever changes.
#
# Deliberately permissive, not a full SemVer 2.0 validator (leading-zero
# rejection etc. is release-alpha.yml's job, at mint time - a consumer
# here only needs to recognize any tag that gate could have let through,
# e.g. a hyphenated identifier like "rc-one.1" or a bare numeric one like
# "1"). The whole prerelease suffix is optional, since a stable release
# has none.
rootguard_version_pattern='[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?'

# rootguard_extract_versions: reads HTML text on stdin, prints one
# plausible RootGuard version mention per line. Exists because the naive
# approach - grep -Eo rootguard_version_pattern directly over page text -
# matches far more than real versions once a bare "X.Y.Z" (no prerelease)
# is allowed: the GitHub icon's own inline SVG <path d="..."> coordinate
# data, IP address examples in the .env docs, and dd.mm.yyyy dates are
# all indistinguishable from a bare version by shape alone - verified
# live against every current site/*.html, all three occur. Filters that a
# single regex can't express without lookahead (deliberately avoided -
# see check-site-facts.sh's BSD/GNU grep portability note):
#   - strips ` d="..."` attribute values first (removes all SVG noise);
#   - keeps a candidate only if it has *exactly* three dot-separated
#     numeric groups before any prerelease suffix (a fourth group means
#     it was actually a longer run, e.g. an IP address);
#   - and only if that third group is at most 3 digits (excludes a
#     dd.mm.yyyy date's 4-digit year - nothing that looks like a real
#     RootGuard patch number will hit that for a very long time).
rootguard_extract_versions() {
  sed -E 's/ d="[^"]*"//g' \
    | grep -oE '[0-9]+(\.[0-9]+){2,}(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?' \
    | awk '{
        core = $0
        sub(/-.*/, "", core)
        n = split(core, groups, ".")
        if (n == 3 && length(groups[3]) <= 3) print $0
      }'
}
