#!/usr/bin/env bash
set -Eeuo pipefail

repository_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -f -- \
    "$tmp_dir/bin/docker" \
    "$tmp_dir/repo/scripts/check-debian-pins.sh" \
    "$tmp_dir/repo/rootguard-unbound/Dockerfile"
  rmdir -- "$tmp_dir/bin" "$tmp_dir/repo/scripts" \
    "$tmp_dir/repo/rootguard-unbound" "$tmp_dir/repo" "$tmp_dir"
}
trap cleanup EXIT

mkdir -p "$tmp_dir/bin" "$tmp_dir/repo/scripts" "$tmp_dir/repo/rootguard-unbound"
cp "$repository_dir/scripts/check-debian-pins.sh" "$tmp_dir/repo/scripts/check-debian-pins.sh"

cat >"$tmp_dir/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
[[ "${PIN_TEST_MODE:?}" == "operational-error" ]] && exit 125

after_separator=0
for argument in "$@"; do
  if [[ "$after_separator" -eq 0 ]]; then
    [[ "$argument" == "--" ]] && after_separator=1
    continue
  fi
  case "$argument" in
    ca-certificates)
      if [[ "$PIN_TEST_MODE" == "missing-candidate" ]]; then
        candidate=""
      elif [[ "$PIN_TEST_MODE" == "prefix-candidate" ]]; then
        candidate="20250419+deb13u1"
      else
        candidate="20250419"
      fi
      ;;
    curl)
      if [[ "$PIN_TEST_MODE" == "current" || "$PIN_TEST_MODE" == "prefix-candidate" ]]; then
        candidate="8.14.1-2+deb13u3"
      else
        candidate="8.14.1-2+deb13u4"
      fi
      ;;
    *)
      exit 3
      ;;
  esac
  printf '%s=%s\n' "$argument" "$candidate"
done
EOF
chmod +x "$tmp_dir/bin/docker"
export PATH="$tmp_dir/bin:$PATH"

write_fixture() {
  cat >"$tmp_dir/repo/rootguard-unbound/Dockerfile" <<'EOF'
FROM debian:13-slim@example AS builder
FROM debian:13-slim@example
RUN apt-get install -y \
        ca-certificates=20250419 \
        curl=8.14.1-2+deb13u3 \
    && true
RUN apt-get install -y \
        curl=8.14.1-2+deb13u3 \
    && true
EOF
}

expect_status() {
  local expected="$1"
  shift
  local status
  set +e
  "$tmp_dir/repo/scripts/check-debian-pins.sh" "$@" >/dev/null 2>&1
  status=$?
  set -e
  [[ "$status" -eq "$expected" ]]
}

write_fixture
export PIN_TEST_MODE=current
expect_status 0

export PIN_TEST_MODE=drift
expect_status 1
grep -Fq 'curl=8.14.1-2+deb13u3' "$tmp_dir/repo/rootguard-unbound/Dockerfile"

expect_status 0 --fix
[[ "$(grep -Fc 'curl=8.14.1-2+deb13u4' "$tmp_dir/repo/rootguard-unbound/Dockerfile")" -eq 2 ]]
if grep -Fq 'curl=8.14.1-2+deb13u3' "$tmp_dir/repo/rootguard-unbound/Dockerfile"; then
  echo "stale curl pin remained after --fix" >&2
  exit 1
fi

# #767: the new version extends the old one (20250419 -> 20250419+deb13u1);
# the old pin is a prefix of the new pin and must not count as "still there".
write_fixture
export PIN_TEST_MODE=prefix-candidate
expect_status 0 --fix
grep -Fq 'ca-certificates=20250419+deb13u1 ' "$tmp_dir/repo/rootguard-unbound/Dockerfile"
if grep -Eq 'ca-certificates=20250419([[:space:]]|$)' "$tmp_dir/repo/rootguard-unbound/Dockerfile"; then
  echo "stale ca-certificates pin remained after --fix" >&2
  exit 1
fi
expect_status 0 --fix
[[ "$(grep -Fc 'ca-certificates=20250419+deb13u1 ' "$tmp_dir/repo/rootguard-unbound/Dockerfile")" -eq 1 ]]

write_fixture
export PIN_TEST_MODE=missing-candidate
expect_status 2 --fix
grep -Fq 'curl=8.14.1-2+deb13u3' "$tmp_dir/repo/rootguard-unbound/Dockerfile"

export PIN_TEST_MODE=operational-error
expect_status 125

echo "check-debian-pins tests passed"
