#!/usr/bin/env bash
set -euo pipefail

# Easy Rathole public bootstrap installer
# Usage (recommended):
#   curl -fsSL https://raw.githubusercontent.com/pruedence21/easy-ipos5-tunnel/main/public-install.sh | sudo bash

REPO_URL="${REPO_URL:-https://github.com/pruedence21/easy-ipos5-tunnel}"
REPO_BRANCH="${REPO_BRANCH:-main}"
REPO_BASE_DIR="${REPO_BASE_DIR:-/opt/easy-rathole}"

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

fail() {
  log "ERROR: $*"
  exit 1
}

require_root() {
  [[ "$EUID" -eq 0 ]] || fail "Jalankan dengan sudo/root. Contoh: curl ... | sudo bash"
}

ensure_git() {
  if command -v git >/dev/null 2>&1; then
    return 0
  fi

  if command -v apt-get >/dev/null 2>&1; then
    log "git belum terpasang, mencoba install via apt-get..."
    apt-get update -y
    DEBIAN_FRONTEND=noninteractive apt-get install -y git
    command -v git >/dev/null 2>&1 || fail "Gagal install git via apt-get"
    return 0
  fi

  fail "git tidak ditemukan. Install git terlebih dahulu lalu jalankan ulang script."
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
  ensure_git

  local owner repo
  mapfile -t parsed < <(parse_repo "$REPO_URL")
  owner="${parsed[0]}"
  repo="${parsed[1]}"

  local work_tree="${REPO_BASE_DIR}/src/${repo}"
  mkdir -p "$(dirname "$work_tree")"

  if [[ -d "${work_tree}/.git" ]]; then
    local current_origin=""
    current_origin="$(git -C "$work_tree" remote get-url origin 2>/dev/null || true)"
    if [[ -n "$current_origin" && "$current_origin" != "$REPO_URL" && "$current_origin" != "${REPO_URL%.git}" && "$current_origin" != "${REPO_URL%.git}.git" ]]; then
      fail "Direktori ${work_tree} sudah berisi repo lain (origin: ${current_origin}). Hapus/ubah REPO_BASE_DIR dulu."
    fi

    log "Repo sudah ada, melakukan update terbaru (git pull)..."
    git -C "$work_tree" fetch --prune origin
    git -C "$work_tree" checkout "$REPO_BRANCH"
    git -C "$work_tree" pull --ff-only origin "$REPO_BRANCH"
  else
    if [[ -e "$work_tree" && ! -d "$work_tree/.git" ]]; then
      fail "Path ${work_tree} sudah ada tapi bukan git repo. Pindahkan/hapus dulu, atau ganti REPO_BASE_DIR."
    fi

    log "Cloning source from ${REPO_URL} (branch: ${REPO_BRANCH})"
    git clone --branch "$REPO_BRANCH" --single-branch "$REPO_URL" "$work_tree"
  fi

  [[ -f "${work_tree}/install.sh" ]] || fail "install.sh tidak ditemukan dalam repo"

  log "Running installer from ${work_tree}"
  bash "${work_tree}/install.sh"
}

main "$@"
