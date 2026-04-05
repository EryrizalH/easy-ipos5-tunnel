#!/usr/bin/env bash
set -euo pipefail

# Easy Rathole public bootstrap installer
# Usage (recommended):
#   curl -fsSL https://raw.githubusercontent.com/pruedence21/easy-ipos5-tunnel/main/public-install.sh | sudo bash

REPO_URL="${REPO_URL:-https://github.com/pruedence21/easy-ipos5-tunnel}"
REPO_BRANCH="${REPO_BRANCH:-main}"
WORKDIR=""

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

fail() {
  log "ERROR: $*"
  exit 1
}

cleanup() {
  if [[ -n "${WORKDIR}" && -d "${WORKDIR}" ]]; then
    rm -rf "${WORKDIR}"
  fi
}

require_root() {
  [[ "$EUID" -eq 0 ]] || fail "Jalankan dengan sudo/root. Contoh: curl ... | sudo bash"
}

parse_repo() {
  local cleaned="$1"
  cleaned="${cleaned%.git}"
  cleaned="${cleaned%/}"

  if [[ "$cleaned" =~ github.com/([^/]+)/([^/]+)$ ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
    return 0
  fi

  fail "REPO_URL harus format GitHub valid, contoh: https://github.com/owner/repo"
}

main() {
  require_root

  command -v curl >/dev/null 2>&1 || fail "curl tidak ditemukan"
  command -v tar >/dev/null 2>&1 || fail "tar tidak ditemukan"

  local owner repo
  mapfile -t parsed < <(parse_repo "$REPO_URL")
  owner="${parsed[0]}"
  repo="${parsed[1]}"

  local tar_url="https://codeload.github.com/${owner}/${repo}/tar.gz/refs/heads/${REPO_BRANCH}"
  WORKDIR="$(mktemp -d -t easy-rathole-bootstrap-XXXXXX)"
  trap cleanup EXIT

  log "Downloading source from ${REPO_URL} (branch: ${REPO_BRANCH})"
  curl -fL "$tar_url" -o "${WORKDIR}/repo.tar.gz"
  tar -xzf "${WORKDIR}/repo.tar.gz" -C "$WORKDIR"

  local src_dir
  src_dir="$(find "$WORKDIR" -maxdepth 1 -type d -name "${repo}-*" | head -n 1 || true)"
  [[ -n "$src_dir" ]] || fail "Gagal menemukan direktori source hasil extract"
  [[ -f "${src_dir}/install.sh" ]] || fail "install.sh tidak ditemukan dalam repo"

  log "Running installer from ${src_dir}"
  bash "${src_dir}/install.sh"
}

main "$@"
