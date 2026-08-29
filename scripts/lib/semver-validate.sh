#!/usr/bin/env bash
# Shared by release-alpha.yml and release-version-bump.yml (both run
# entirely within this repo's own checkout, so sourcing a real file here
# is simpler than the composite-action route this used to be dismissed
# for - found in review while addressing a security audit's code-
# compression suggestions: the two workflows had drifted into keeping a
# byte-for-byte identical regex duplicated by hand, and once genuinely
# fell out of sync for real - only release-alpha.yml validated a version
# at all until this check existed in release-version-bump.yml too, so a
# typo'd manual override there had already produced a real commit, tag,
# and GitHub Release before anything caught it.
#
# install.sh can't use this the same way: it runs standalone via
# curl | bash on an end-user's machine, with no access to the rest of
# this repository at that point, so it keeps its own full comparator
# (version_gt et al. - real precedence comparison, not just validation)
# for that reason, not out of neglect.
#
# Usage: source this file, then call require_semver "$value" - exits
# non-zero (message on stderr) if $value isn't a valid SemVer 2.0
# version. Build metadata is deliberately excluded from the grammar:
# it's not representable in a Docker tag ('+' isn't a legal tag
# character), and every caller here is validating a string that's about
# to become one.
SEMVER_PATTERN='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?$'

require_semver() {
  local value="$1"
  if [[ ! "$value" =~ $SEMVER_PATTERN ]]; then
    echo "Refusing non-semantic version: $value" >&2
    exit 1
  fi
}
