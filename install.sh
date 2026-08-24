#!/usr/bin/env bash
# RootGuard one-line installer - for people who just want a working DNS
# filter, not a Docker Compose tutorial. Detects and installs Docker if
# needed (via Foxly's dockerinstall.sh, Debian/Ubuntu only), downloads the
# current release's compose.release.yaml/.env.release.example, generates
# the two secret tokens, asks for a WebGUI username/password, and starts
# the stack.
#
# This is *in addition to* the manual quick start in docs.html, not a
# replacement - anyone who wants to see/edit every step by hand still can.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/foxly-it/rootguard/main/install.sh | bash
#
# Non-interactive / scripted use: set ROOTGUARD_ADMIN_USER and
# ROOTGUARD_ADMIN_PASSWORD in the environment first, or pass
# --non-interactive (generates a random password and prints it once).

set -Eeuo pipefail

RG_REPO="foxly-it/rootguard"
DOCKERINSTALL_URL="https://raw.githubusercontent.com/foxly-it/dockerinstall/main/dockerinstall.sh"
NON_INTERACTIVE=0
TARGET_DIR="rootguard"

for arg in "$@"; do
  case "$arg" in
    --non-interactive) NON_INTERACTIVE=1 ;;
    --dir=*) TARGET_DIR="${arg#--dir=}" ;;
    -h|--help)
      echo "Usage: install.sh [--non-interactive] [--dir=PATH]"
      exit 0
      ;;
    *) echo "Unbekannte Option: $arg" >&2; exit 1 ;;
  esac
done

log() { printf '>> %s\n' "$*"; }
die() { printf 'FEHLER: %s\n' "$*" >&2; exit 1; }

# curl | bash means this script's own stdin is the download pipe, not the
# terminal - a plain `read` here would read from that (already-exhausted)
# pipe and either hang or silently get empty input. /dev/tty is the actual
# keyboard for whoever is running this interactively.
prompt() {
  local __var="$1" __message="$2" __default="${3:-}"
  if [ "$NON_INTERACTIVE" = "1" ] || [ ! -r /dev/tty ]; then
    printf -v "$__var" '%s' "$__default"
    return
  fi
  local __reply
  read -r -p "$__message" __reply < /dev/tty
  printf -v "$__var" '%s' "${__reply:-$__default}"
}

prompt_secret() {
  local __var="$1" __message="$2"
  if [ "$NON_INTERACTIVE" = "1" ] || [ ! -r /dev/tty ]; then
    printf -v "$__var" ''
    return
  fi
  local __reply
  read -r -s -p "$__message" __reply < /dev/tty
  echo >&2
  printf -v "$__var" '%s' "$__reply"
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    head -c 48 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

# Same picking logic as rootguard-core's own updater
# (internal/updater/github_release.go): GitHub's own /releases/latest
# excludes prereleases, and every RootGuard release is published as one -
# so the full, newest-first list is queried and the first tag matching
# RootGuard's own release-tag pattern wins. Kept dependency-free (no jq)
# on purpose - this runs on whatever bare system a new user has, before
# anything beyond curl is guaranteed to exist.
resolve_latest_tag() {
  local body
  body="$(curl -fsSL -H 'Accept: application/vnd.github+json' \
    "https://api.github.com/repos/${RG_REPO}/releases")" \
    || die "Konnte die GitHub-Releases-API nicht erreichen."
  printf '%s' "$body" \
    | grep -o '"tag_name": *"[^"]*"' \
    | sed -E 's/.*"([^"]*)"$/\1/' \
    | grep -E '^v0\.1\.0-(alpha|beta)\.[0-9]+$' \
    | head -1
}

docker_present() {
  command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1
}

# Runs the actual `docker` calls below - falls back to sudo automatically:
# either Docker was just installed (group membership needs a fresh login
# to take effect, so the current shell isn't in the docker group yet even
# right after --add-user), or it was already present but this user was
# simply never added to the group.
docker_cmd() {
  if docker info >/dev/null 2>&1; then
    docker "$@"
  else
    sudo docker "$@"
  fi
}

install_docker() {
  log "Docker wurde nicht gefunden - Installation über Foxly dockerinstall wird gestartet."
  command -v sudo >/dev/null 2>&1 || die "sudo wird benötigt, um Docker zu installieren."
  local installer
  installer="$(mktemp)"
  trap 'rm -f "$installer"' RETURN
  curl -fsSL "$DOCKERINSTALL_URL" -o "$installer" \
    || die "Docker-Installer konnte nicht heruntergeladen werden."
  chmod +x "$installer"
  sudo "$installer" install --non-interactive --no-hello --add-user="$(id -un)" \
    || die "Docker-Installation fehlgeschlagen. Manuelle Anleitung: https://docs.docker.com/engine/install/"
  log "Docker wurde installiert."
}

main() {
  if docker_present; then
    log "Docker ist bereits vorhanden."
  else
    install_docker
  fi

  if [ -e "$TARGET_DIR" ]; then
    die "'$TARGET_DIR' existiert bereits - vermutlich läuft hier schon eine Installation. Abgebrochen, um nichts zu überschreiben."
  fi

  log "Ermittle die aktuelle RootGuard-Version…"
  local tag
  tag="$(resolve_latest_tag)"
  [ -n "$tag" ] || die "Keine RootGuard-Version über die GitHub-Releases-API gefunden."
  log "Aktuelle Version: $tag"

  mkdir -p "$TARGET_DIR"
  cd "$TARGET_DIR"

  local raw_base="https://raw.githubusercontent.com/${RG_REPO}/${tag}"
  curl -fsSL -o compose.release.yaml "${raw_base}/compose.release.yaml" \
    || die "compose.release.yaml konnte nicht heruntergeladen werden."
  curl -fsSL -o .env "${raw_base}/.env.release.example" \
    || die "Beispielkonfiguration konnte nicht heruntergeladen werden."

  local api_token recovery_token admin_user admin_password
  api_token="$(random_secret)"
  recovery_token="$(random_secret)"

  prompt admin_user "Benutzername für die RootGuard-WebGUI [admin]: " "admin"

  local generated_password=0
  admin_password="${ROOTGUARD_ADMIN_PASSWORD:-}"
  if [ -z "$admin_password" ]; then
    prompt_secret admin_password "Passwort für die RootGuard-WebGUI (leer = zufällig generieren): "
    echo
  fi
  if [ -z "$admin_password" ]; then
    admin_password="$(random_secret | head -c 20)"
    generated_password=1
  fi

  # Values are passed through the environment, never interpolated into a
  # sed/awk program's own text - a password containing '&', '#', or a
  # backslash would otherwise corrupt the substitution (or worse) with any
  # editor-style in-place replacement.
  ROOTGUARD_API_TOKEN="$api_token" \
  ROOTGUARD_RECOVERY_TOKEN="$recovery_token" \
  ROOTGUARD_ADMIN_USER="$admin_user" \
  ROOTGUARD_ADMIN_PASSWORD="$admin_password" \
  awk '
    /^ROOTGUARD_API_TOKEN=/      { print "ROOTGUARD_API_TOKEN=" ENVIRON["ROOTGUARD_API_TOKEN"]; next }
    /^ROOTGUARD_RECOVERY_TOKEN=/ { print "ROOTGUARD_RECOVERY_TOKEN=" ENVIRON["ROOTGUARD_RECOVERY_TOKEN"]; next }
    /^ROOTGUARD_ADMIN_USER=/     { print "ROOTGUARD_ADMIN_USER=" ENVIRON["ROOTGUARD_ADMIN_USER"]; next }
    /^ROOTGUARD_ADMIN_PASSWORD=/ { print "ROOTGUARD_ADMIN_PASSWORD=" ENVIRON["ROOTGUARD_ADMIN_PASSWORD"]; next }
    { print }
  ' .env > .env.tmp
  mv .env.tmp .env
  chmod 600 .env

  log "Starte den RootGuard-Stack…"
  docker_cmd compose -f compose.release.yaml up -d \
    || die "RootGuard konnte nicht gestartet werden - läuft der Docker-Dienst? ('systemctl status docker' prüfen)."

  echo
  log "Fertig! Öffne http://localhost:8080/login"
  log "Benutzername: ${admin_user}"
  if [ "$generated_password" = "1" ]; then
    log "Generiertes Passwort (jetzt notieren, wird nicht erneut angezeigt): ${admin_password}"
  fi
  log "Danach dem geführten Setup in der WebGUI folgen."
}

main
