#!/usr/bin/env bash
set -euo pipefail

EASY_RATHOLE_ROOT="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
EASY_RATHOLE_STATE_FILE="${EASY_RATHOLE_STATE_FILE:-${EASY_RATHOLE_ROOT}/state/install-state.json}"

usage() {
  cat <<'EOF'
easy-rathole - root CLI setup dan debugging IPOS5TunnelPublik

Usage:
  sudo easy-rathole status
  sudo easy-rathole setup [--db-sync] [--vps-db-only]
  sudo easy-rathole db-sync status
  sudo easy-rathole db-sync finalize
  sudo easy-rathole db-sync repair
  sudo easy-rathole debug [--lines N]
  sudo easy-rathole logs [rathole|dashboard|bucardo] [-n N]
  sudo easy-rathole doctor

Environment:
  EASY_RATHOLE_ROOT=/opt/easy-rathole
  EASY_RATHOLE_STATE_FILE=/opt/easy-rathole/state/install-state.json

Notes:
  - Perintah operasional butuh root karena membaca state private,
    systemd, Docker, PostgreSQL, dan Bucardo.
  - setup memakai source_dir dari state file bila tersedia.
EOF
}

require_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    return 0
  fi
  if command -v sudo >/dev/null 2>&1; then
    exec sudo -E "$0" "$@"
  fi
  printf 'Perintah ini wajib root. Jalankan dengan sudo.\n' >&2
  exit 1
}

state_get() {
  local key="$1"
  local default_value="${2:-}"
  python3 - "$EASY_RATHOLE_STATE_FILE" "$key" "$default_value" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
keys = sys.argv[2].split(".")
default = sys.argv[3]

try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    print(default)
    raise SystemExit(0)

value = data
for key in keys:
    if not isinstance(value, dict) or key not in value:
        print(default)
        raise SystemExit(0)
    value = value[key]

if isinstance(value, bool):
    print("true" if value else "false")
elif isinstance(value, (dict, list)):
    print(json.dumps(value, separators=(",", ":")))
elif value is None:
    print(default)
else:
    print(value)
PY
}

source_dir() {
  local configured
  configured="$(state_get "source_dir" "")"
  if [[ -n "$configured" && -d "$configured" ]]; then
    printf '%s\n' "$configured"
    return 0
  fi
  if [[ -d "${EASY_RATHOLE_ROOT}/src/easy-ipos5-tunnel" ]]; then
    printf '%s\n' "${EASY_RATHOLE_ROOT}/src/easy-ipos5-tunnel"
    return 0
  fi
  printf 'Source repo tidak ditemukan. Pastikan state source_dir valid atau set EASY_RATHOLE_SOURCE_DIR.\n' >&2
  return 1
}

script_path() {
  local rel="$1"
  local root
  root="${EASY_RATHOLE_SOURCE_DIR:-$(source_dir)}"
  printf '%s/%s\n' "$root" "$rel"
}

service_status() {
  local service="$1"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl is-active "$service" 2>/dev/null || true
  else
    printf 'unknown\n'
  fi
}

connect_host_for_bind() {
  case "$1" in
    "0.0.0.0"|"::"|"[::]") printf '127.0.0.1\n' ;;
    *) printf '%s\n' "$1" ;;
  esac
}

print_state_summary() {
  python3 - "$EASY_RATHOLE_STATE_FILE" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception as exc:
    print(f"State: unavailable ({exc})")
    raise SystemExit(0)

sync = data.get("db_sync_mode") if isinstance(data.get("db_sync_mode"), dict) else {}
rows = sync.get("databases") if isinstance(sync.get("databases"), list) else []
summary = {
    "total": len(rows),
    "synced": sum(1 for row in rows if isinstance(row, dict) and row.get("status") == "synced"),
    "pending": sum(1 for row in rows if isinstance(row, dict) and str(row.get("status", "")).startswith("pending")),
    "error": sum(1 for row in rows if isinstance(row, dict) and row.get("status") == "error"),
    "dropped": sum(1 for row in rows if isinstance(row, dict) and row.get("status") == "dropped"),
}

print(f"State file       : {path}")
print(f"Public IP        : {data.get('public_ip', '-')}")
print(f"Dashboard        : http://{data.get('public_ip', '<ip>')}:{data.get('dashboard_port', 8088)}")
print(f"Rathole control  : {data.get('rathole_control_port', '-')}")
print(f"Exposed ports    : {', '.join(str(p) for p in data.get('exposed_ports', [])) or '-'}")
print(f"DB sync enabled  : {sync.get('enabled', False)}")
print(f"DB sync summary  : {summary}")
print(f"Updated at       : {data.get('updated_at', '-')}")
PY
}

cmd_status() {
  print_state_summary
  printf '\nServices:\n'
  printf '  rathole                 : %s\n' "$(service_status "$(state_get "rathole_service_name" "rathole")")"
  printf '  easy-rathole-dashboard  : %s\n' "$(service_status "$(state_get "dashboard_service_name" "easy-rathole-dashboard")")"
  if command -v bucardo >/dev/null 2>&1; then
    printf '  bucardo                 : available\n'
  else
    printf '  bucardo                 : not installed\n'
  fi
}

cmd_setup() {
  local root
  root="${EASY_RATHOLE_SOURCE_DIR:-$(source_dir)}"
  [[ -x "${root}/install.sh" || -f "${root}/install.sh" ]] || {
    printf 'install.sh tidak ditemukan di %s\n' "$root" >&2
    exit 1
  }

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --db-sync)
        export EASY_RATHOLE_INSTALL_DB_SYNC=1
        ;;
      --vps-db-only)
        export EASY_RATHOLE_INSTALL_VPS_DB=1
        ;;
      -h|--help)
        usage
        return 0
        ;;
      *)
        printf 'Argumen setup tidak dikenal: %s\n' "$1" >&2
        exit 2
        ;;
    esac
    shift
  done

  export EASY_RATHOLE_ROOT
  export EASY_RATHOLE_STATE_FILE
  bash "${root}/install.sh"
}

cmd_db_sync_status() {
  python3 - "$EASY_RATHOLE_STATE_FILE" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception as exc:
    print(f"State DB sync tidak bisa dibaca: {exc}")
    raise SystemExit(1)

sync = data.get("db_sync_mode")
if not isinstance(sync, dict) or sync.get("enabled") is not True:
    print("DB sync belum aktif.")
    raise SystemExit(0)

rows = sync.get("databases") if isinstance(sync.get("databases"), list) else []
summary = {
    "total": len(rows),
    "synced": sum(1 for row in rows if isinstance(row, dict) and row.get("status") == "synced"),
    "pending": sum(1 for row in rows if isinstance(row, dict) and str(row.get("status", "")).startswith("pending")),
    "error": sum(1 for row in rows if isinstance(row, dict) and row.get("status") == "error"),
    "dropped": sum(1 for row in rows if isinstance(row, dict) and row.get("status") == "dropped"),
}
print("DB sync registry")
print(f"  summary           : {summary}")
print(f"  last discovery    : {sync.get('last_discovery_at', '-')}")
print(f"  waiting client    : {sync.get('waiting_for_client', False)}")
print(f"  bucardo configured: {sync.get('bucardo_configured', False)}")
print("")
for row in sorted((r for r in rows if isinstance(r, dict)), key=lambda r: str(r.get("name", "")).lower()):
    name = row.get("name", "-")
    status = row.get("status", "-")
    phase = row.get("phase", "-")
    sync_name = row.get("sync_name", "-")
    print(f"{name} | {status} | {phase} | {sync_name}")
    if row.get("last_error"):
        print(f"  error : {row.get('last_error')}")
    if row.get("last_error_detail"):
        print(f"  detail: {row.get('last_error_detail')}")
    unsupported = row.get("unsupported_tables")
    if isinstance(unsupported, list) and unsupported:
        print(f"  unsupported tables: {', '.join(str(item) for item in unsupported)}")
PY

  if command -v bucardo >/dev/null 2>&1; then
    printf '\nBucardo status:\n'
    bucardo status || true
    printf '\nBucardo sync list:\n'
    bucardo list sync || true
  fi
}

cmd_db_sync_finalize() {
  local script
  script="$(script_path "scripts/install_db_sync_bucardo.sh")"
  [[ -f "$script" ]] || {
    printf 'Script finalisasi DB sync tidak ditemukan: %s\n' "$script" >&2
    exit 1
  }
  export EASY_RATHOLE_ROOT
  export EASY_RATHOLE_STATE_FILE
  bash "$script"
}

cmd_logs() {
  local service="${1:-rathole}"
  local lines=100
  if [[ $# -gt 0 ]]; then
    shift
  fi
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -n|--lines)
        lines="${2:-100}"
        shift 2
        ;;
      *)
        printf 'Argumen logs tidak dikenal: %s\n' "$1" >&2
        exit 2
        ;;
    esac
  done

  case "$service" in
    dashboard) service="easy-rathole-dashboard" ;;
    rathole|easy-rathole-dashboard|bucardo) ;;
    *)
      printf 'Service tidak valid: %s\n' "$service" >&2
      exit 2
      ;;
  esac

  journalctl -u "$service" -n "$lines" --no-pager
}

cmd_debug() {
  local lines=80
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -n|--lines)
        lines="${2:-80}"
        shift 2
        ;;
      *)
        printf 'Argumen debug tidak dikenal: %s\n' "$1" >&2
        exit 2
        ;;
    esac
  done

  cmd_status
  printf '\nListening ports:\n'
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp | grep -E ':5444|:5445|:5480|:5485|:8088|:2333' || true
  else
    printf 'ss tidak tersedia\n'
  fi

  printf '\nDB sync:\n'
  cmd_db_sync_status || true

  printf '\nRecent rathole logs:\n'
  cmd_logs rathole --lines "$lines" || true

  printf '\nRecent dashboard logs:\n'
  cmd_logs dashboard --lines "$lines" || true
}

cmd_doctor() {
  local failed=0
  printf 'Doctor checks\n'

  for cmd in python3 systemctl ss; do
    if command -v "$cmd" >/dev/null 2>&1; then
      printf '  ok   command %s\n' "$cmd"
    else
      printf '  fail command %s missing\n' "$cmd"
      failed=1
    fi
  done

  if [[ -f "$EASY_RATHOLE_STATE_FILE" ]]; then
    printf '  ok   state file exists\n'
  else
    printf '  fail state file missing: %s\n' "$EASY_RATHOLE_STATE_FILE"
    failed=1
  fi

  for service in "$(state_get "rathole_service_name" "rathole")" "$(state_get "dashboard_service_name" "easy-rathole-dashboard")"; do
    if systemctl is-active --quiet "$service"; then
      printf '  ok   service %s active\n' "$service"
    else
      printf '  warn service %s not active\n' "$service"
    fi
  done

  local vps_addr private_addr
  vps_addr="$(state_get "db_sync_mode.vps_db_addr" "127.0.0.1:5444")"
  private_addr="$(state_get "db_sync_mode.private_db_tunnel_addr" "127.0.0.1:5445")"
  for addr in "$vps_addr" "$private_addr"; do
    local host port
    host="${addr%:*}"
    port="${addr##*:}"
    host="$(connect_host_for_bind "$host")"
    if [[ "$port" =~ ^[0-9]+$ ]] && timeout 2 bash -c "cat < /dev/null > /dev/tcp/${host}/${port}" >/dev/null 2>&1; then
      printf '  ok   tcp %s reachable\n' "$addr"
    else
      printf '  warn tcp %s not reachable\n' "$addr"
    fi
  done

  return "$failed"
}

main() {
  local cmd="${1:-help}"
  case "$cmd" in
    -h|--help|help)
      usage
      ;;
    status)
      require_root "$@"
      shift
      cmd_status "$@"
      ;;
    setup)
      require_root "$@"
      shift
      cmd_setup "$@"
      ;;
    db-sync)
      require_root "$@"
      shift
      case "${1:-status}" in
        status)
          cmd_db_sync_status
          ;;
        finalize|repair)
          cmd_db_sync_finalize
          ;;
        *)
          printf 'Subcommand db-sync tidak dikenal: %s\n' "${1:-}" >&2
          exit 2
          ;;
      esac
      ;;
    debug)
      require_root "$@"
      shift
      cmd_debug "$@"
      ;;
    logs)
      require_root "$@"
      shift
      cmd_logs "$@"
      ;;
    doctor)
      require_root "$@"
      shift
      cmd_doctor "$@"
      ;;
    *)
      printf 'Command tidak dikenal: %s\n' "$cmd" >&2
      usage >&2
      exit 2
      ;;
  esac
}

main "$@"
