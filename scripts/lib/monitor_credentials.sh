#!/usr/bin/env bash

collect_monitor_credentials() {
  local config_dir="${NUSA_TUNNEL_CONFIG_DIR:-/etc/nusatunnel}"
  local credential_file="$config_dir/postgres-monitor.env"
  if [[ -f "$credential_file" && -z "${NUSA_TUNNEL_PG_MONITOR_USER:-}" && -z "${NUSA_TUNNEL_PG_MONITOR_PASSWORD:-}" ]]; then
    [[ ! -L "$credential_file" ]] || fail "File kredensial monitor tidak boleh symlink"
    [[ "$(stat -c '%u:%a' "$credential_file")" == "0:600" ]] || fail "File kredensial monitor harus milik root dengan izin 0600"
    local helper_root
    helper_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
    python3 "$helper_root/scripts/release.py" monitor-env-check --output "$credential_file" || fail "Isi file kredensial monitor belum lengkap."
    return
  fi
  if [[ -z "${NUSA_TUNNEL_PG_MONITOR_USER:-}" || -z "${NUSA_TUNNEL_PG_MONITOR_PASSWORD:-}" ]]; then
    local tty_fd
    if ! { exec {tty_fd}<>/dev/tty; } 2>/dev/null; then
      fail "Terminal tidak tersedia; isi NUSA_TUNNEL_PG_MONITOR_USER dan NUSA_TUNNEL_PG_MONITOR_PASSWORD."
    fi
    if [[ -z "${NUSA_TUNNEL_PG_MONITOR_USER:-}" ]]; then
      printf 'Username database IPOS 5: ' >&"$tty_fd"
      IFS= read -r -u "$tty_fd" NUSA_TUNNEL_PG_MONITOR_USER || fail "Input kredensial dibatalkan."
    fi
    if [[ -z "${NUSA_TUNNEL_PG_MONITOR_PASSWORD:-}" ]]; then
      printf 'Password database IPOS 5: ' >&"$tty_fd"
      IFS= read -rs -u "$tty_fd" NUSA_TUNNEL_PG_MONITOR_PASSWORD || fail "Input kredensial dibatalkan."
      printf '\n' >&"$tty_fd"
    fi
    exec {tty_fd}>&-
  fi
  [[ -n "$NUSA_TUNNEL_PG_MONITOR_USER" && -n "$NUSA_TUNNEL_PG_MONITOR_PASSWORD" ]] || fail "Kredensial database wajib diisi."
  export NUSA_TUNNEL_PG_MONITOR_USER NUSA_TUNNEL_PG_MONITOR_PASSWORD
  # Validate before hardening, without persisting secret values yet.
  python3 - <<'PY'
import os
for key in ('NUSA_TUNNEL_PG_MONITOR_USER', 'NUSA_TUNNEL_PG_MONITOR_PASSWORD'):
    if any(char in os.environ[key] for char in '\r\n\0'):
        raise SystemExit('Kredensial tidak boleh mengandung newline/NUL.')
PY
}

save_monitor_credentials() {
  local project_root="$1" credential_file="$2"
  if [[ -n "${NUSA_TUNNEL_PG_MONITOR_USER:-}" && -n "${NUSA_TUNNEL_PG_MONITOR_PASSWORD:-}" ]]; then
    python3 "$project_root/scripts/release.py" monitor-env --output "$credential_file"
    chown root:root "$credential_file"
  fi
  [[ -f "$credential_file" ]] || fail "Kredensial monitor belum tersedia."
}
