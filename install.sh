#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/scripts/lib/common.sh"

configure_firewall_ports() {
  local state_file="$1"
  local control_port
  local dashboard_port
  local dashboard_allow_cidr="${EASY_RATHOLE_DASHBOARD_ALLOW_CIDR:-}"
  local exposed_ports_json

  control_port="$(state_get "$state_file" "rathole_control_port" "0")"
  dashboard_port="$(state_get "$state_file" "dashboard_port" "8088")"
  exposed_ports_json="$(state_get "$state_file" "exposed_ports" "[]")"

  local remote_ports_json
  remote_ports_json="$(state_get "$state_file" "service_ports" "[]")"
  local ports=("$control_port")
  while IFS= read -r port; do
    [[ -n "$port" ]] || continue
    ports+=("$port")
  done < <(
    python3 - "$remote_ports_json" "$exposed_ports_json" <<'PY'
import json
import sys

try:
    rows = json.loads(sys.argv[1])
except Exception:
    rows = []

try:
    configured_ports = json.loads(sys.argv[2])
except Exception:
    configured_ports = []

seen = set()
for raw_port in configured_ports:
    try:
        port = int(raw_port)
    except Exception:
        continue
    if port in seen:
        continue
    seen.add(port)
    print(port)

for row in rows:
    if not isinstance(row, dict):
        continue
    if row.get("expose_public") is False:
        continue
    try:
        port = int(row.get("remote_bind_port"))
    except Exception:
        continue
    if port in seen:
        continue
    seen.add(port)
    print(port)
PY
  )

  if command -v ufw >/dev/null 2>&1; then
    if ufw status 2>/dev/null | grep -q "Status: active"; then
      log INFO "Mengonfigurasi aturan UFW..."
      for p in "${ports[@]}"; do
        [[ "$p" =~ ^[0-9]+$ ]] || continue
        ufw allow "${p}/tcp" >/dev/null || true
      done

      if [[ -n "$dashboard_allow_cidr" ]]; then
        ufw allow from "$dashboard_allow_cidr" to any port "$dashboard_port" proto tcp >/dev/null || true
        log INFO "Dashboard port ${dashboard_port} restricted to ${dashboard_allow_cidr}"
      else
        ufw allow "${dashboard_port}/tcp" >/dev/null || true
      fi
    else
      log WARN "UFW terpasang tetapi tidak aktif. Otomasi firewall dilewati."
    fi
  elif command -v firewall-cmd >/dev/null 2>&1; then
    if systemctl is-active --quiet firewalld; then
      log INFO "Mengonfigurasi aturan firewalld..."
      for p in "${ports[@]}"; do
        [[ "$p" =~ ^[0-9]+$ ]] || continue
        firewall-cmd --permanent --add-port="${p}/tcp" >/dev/null || true
      done

      if [[ -n "$dashboard_allow_cidr" ]]; then
        firewall-cmd --permanent \
          --add-rich-rule="rule family='ipv4' source address='${dashboard_allow_cidr}' port port='${dashboard_port}' protocol='tcp' accept" >/dev/null || true
        log INFO "Dashboard port ${dashboard_port} restricted to ${dashboard_allow_cidr} (firewalld rich-rule)"
      else
        firewall-cmd --permanent --add-port="${dashboard_port}/tcp" >/dev/null || true
      fi

      firewall-cmd --reload >/dev/null || true
    else
      log WARN "firewalld terdeteksi tetapi tidak aktif. Otomasi firewall dilewati."
    fi
  else
    log WARN "No supported firewall manager detected (ufw/firewalld)."
  fi
}

install_cli() {
  local state_file="$1"
  local cli_path="${EASY_RATHOLE_CLI_PATH:-/usr/local/bin/easy-rathole}"
  local now

  install -m 0755 "${SCRIPT_DIR}/scripts/easy-rathole-cli.sh" "$cli_path"
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  state_merge_json "$state_file" "{\
    \"cli_path\": \"${cli_path}\", \
    \"updated_at\": \"${now}\"\
  }"
}

initialize_db_sync_mode() {
  local state_file="$1"
  local vps_bind_host="${EASY_RATHOLE_VPS_DB_BIND_HOST:-0.0.0.0}"
  local vps_bind_port="${EASY_RATHOLE_VPS_DB_PORT:-5444}"
  local private_addr="${EASY_RATHOLE_SYNC_PRIVATE_ADDR:-127.0.0.1:5445}"
  local backend_mode="${EASY_RATHOLE_PRIVATE_DB_BACKEND_MODE:-direct}"
  local dbname="${EASY_RATHOLE_VPS_DB_NAME:-postgres}"
  local dbuser="${EASY_RATHOLE_VPS_DB_USER:-sysi5adm}"
  local dbpass="${EASY_RATHOLE_VPS_DB_PASSWORD:-u&aV23cc.o82dtr1x89c}"
  local exclude_databases="${EASY_RATHOLE_DB_SYNC_EXCLUDE_DATABASES:-postgres,template0,template1,bucardo}"
  local drop_policy="${EASY_RATHOLE_DB_SYNC_DROP_POLICY:-mirror_drop}"
  local conflict_policy="${EASY_RATHOLE_DB_SYNC_CONFLICT_POLICY:-client_wins}"

  python3 - "$state_file" "$vps_bind_host" "$vps_bind_port" "$private_addr" "$backend_mode" "$dbname" "$dbuser" "$dbpass" "$exclude_databases" "$drop_policy" "$conflict_policy" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
vps_bind_host = sys.argv[2]
vps_bind_port = sys.argv[3]
private_addr = sys.argv[4]
backend_mode = sys.argv[5]
dbname = sys.argv[6]
dbuser = sys.argv[7]
dbpass = sys.argv[8]
exclude_databases = sys.argv[9]
drop_policy = sys.argv[10]
conflict_policy = sys.argv[11]

try:
    data = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {}
except Exception:
    data = {}

existing = data.get("db_sync_mode")
if not isinstance(existing, dict):
    existing = {}

existing.update(
    {
        "enabled": True,
        "vps_db_addr": (
            f"{vps_bind_host}:{vps_bind_port}"
            if existing.get("vps_db_addr") in (None, "", "127.0.0.1:5444", "localhost:5444")
            else existing.get("vps_db_addr")
        ),
        "private_db_tunnel_addr": existing.get("private_db_tunnel_addr") or private_addr,
        "private_db_backend_mode": existing.get("private_db_backend_mode") or backend_mode,
        "dbname": existing.get("dbname") or dbname,
        "vps_db_user": existing.get("vps_db_user") or dbuser,
        "vps_db_password": existing.get("vps_db_password") or dbpass,
        "private_db_user": existing.get("private_db_user") or dbuser,
        "private_db_password": existing.get("private_db_password") or dbpass,
        "database_scope": existing.get("database_scope") or "user",
        "initial_clone_source": existing.get("initial_clone_source") or "client",
        "new_database_policy": existing.get("new_database_policy") or "auto",
        "ddl_policy": existing.get("ddl_policy") or "auto_register",
        "drop_policy": existing.get("drop_policy") or drop_policy,
        "conflict_policy": existing.get("conflict_policy") or conflict_policy,
        "exclude_databases": existing.get("exclude_databases") or exclude_databases,
        "databases": existing.get("databases") if isinstance(existing.get("databases"), list) else [],
        "initial_clone_done": bool(existing.get("initial_clone_done", False)),
        "bucardo_configured": bool(existing.get("bucardo_configured", False)),
        "waiting_for_client": bool(existing.get("waiting_for_client", False)),
    }
)
data["db_sync_mode"] = existing
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
PY
  local status=$?
  (( status == 0 )) || return "$status"
  chmod 0600 "$state_file"
}

main() {
  require_root
  ensure_ubuntu_22_plus

  export EASY_RATHOLE_ROOT="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
  export EASY_RATHOLE_CONFIG_DIR="${EASY_RATHOLE_CONFIG_DIR:-/etc/easy-rathole}"
  export EASY_RATHOLE_STATE_FILE="${EASY_RATHOLE_STATE_FILE:-${EASY_RATHOLE_ROOT}/state/install-state.json}"

  local total_steps=8
  local current_step=1

  run_step "$current_step" "$total_steps" "Hardening baseline server" \
    bash "${SCRIPT_DIR}/scripts/prepare_server.sh"
  current_step=$((current_step + 1))

  install_runtime_dependencies() {
    apt-get update -y &&
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
    curl \
    unzip \
    python3 \
    python3-pip \
    python3-venv \
    ca-certificates \
    systemd \
    iproute2
  }

  run_step "$current_step" "$total_steps" "Install dependency runtime" install_runtime_dependencies
  current_step=$((current_step + 1))

  if [[ "${EASY_RATHOLE_INSTALL_DB_SYNC:-0}" == "1" ]]; then
    run_step "$current_step" "$total_steps" "Initialize state DB sync" \
      initialize_db_sync_mode "${EASY_RATHOLE_STATE_FILE}"
  else
    skip_step "$current_step" "$total_steps" "Initialize state DB sync" "EASY_RATHOLE_INSTALL_DB_SYNC bukan 1"
  fi
  current_step=$((current_step + 1))

  local install_vps_db
  install_vps_db="${EASY_RATHOLE_INSTALL_VPS_DB:-${EASY_RATHOLE_INSTALL_DB_SYNC:-0}}"
  if [[ "$install_vps_db" == "1" ]]; then
    run_step "$current_step" "$total_steps" "Install PostgreSQL 9.5 VPS Docker" \
      bash "${SCRIPT_DIR}/scripts/install_vps_postgres95_docker.sh"
  else
    skip_step "$current_step" "$total_steps" "Install PostgreSQL 9.5 VPS Docker" "set EASY_RATHOLE_INSTALL_VPS_DB=1 untuk mengaktifkan"
  fi
  current_step=$((current_step + 1))

  run_step "$current_step" "$total_steps" "Install rathole server + service" \
    bash "${SCRIPT_DIR}/scripts/install_rathole_server.sh"
  current_step=$((current_step + 1))

  run_step "$current_step" "$total_steps" "Install dashboard + service" \
    bash "${SCRIPT_DIR}/scripts/install_dashboard.sh"
  current_step=$((current_step + 1))

  run_step "$current_step" "$total_steps" "Configure firewall ports" \
    configure_firewall_ports "${EASY_RATHOLE_STATE_FILE}"
  current_step=$((current_step + 1))

  run_step "$current_step" "$total_steps" "Install root CLI" \
    install_cli "${EASY_RATHOLE_STATE_FILE}"

  if [[ "${EASY_RATHOLE_INSTALL_DB_SYNC:-0}" == "1" ]]; then
    log INFO "DB sync finalization pending. Buka dashboard setelah client tunnel aktif, lalu jalankan Finalisasi DB Sync."
  else
    log INFO "DB sync Bucardo tidak aktif. Set EASY_RATHOLE_INSTALL_DB_SYNC=1 untuk mengaktifkan."
  fi

  local public_ip
  local control_port
  local dashboard_port
  local admin_username
  local credentials_file
  local hardening_applied
  local hardening_ssh_port
  local forward_ports
  local db_sync_enabled
  local db_sync_waiting
  local db_sync_clone_done
  local db_sync_bucardo_configured
  local cli_path

  public_ip="$(state_get "${EASY_RATHOLE_STATE_FILE}" "public_ip" "<unknown>")"
  control_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "rathole_control_port" "<unknown>")"
  dashboard_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "dashboard_port" "8088")"
  admin_username="$(state_get "${EASY_RATHOLE_STATE_FILE}" "admin_username" "admin")"
  credentials_file="$(state_get "${EASY_RATHOLE_STATE_FILE}" "credentials_file" "${EASY_RATHOLE_ROOT}/state/dashboard-credentials.txt")"
  cli_path="$(state_get "${EASY_RATHOLE_STATE_FILE}" "cli_path" "/usr/local/bin/easy-rathole")"
  hardening_applied="$(state_get "${EASY_RATHOLE_STATE_FILE}" "hardening_applied" "false")"
  hardening_ssh_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "hardening_ssh_port" "22")"
  db_sync_enabled="$(python3 - "${EASY_RATHOLE_STATE_FILE}" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    data = {}
sync = data.get("db_sync_mode")
print("true" if isinstance(sync, dict) and sync.get("enabled") is True else "false")
PY
)"
  db_sync_waiting="$(python3 - "${EASY_RATHOLE_STATE_FILE}" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    data = {}
sync = data.get("db_sync_mode")
print("true" if isinstance(sync, dict) and sync.get("waiting_for_client") is True else "false")
PY
)"
  db_sync_clone_done="$(python3 - "${EASY_RATHOLE_STATE_FILE}" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    data = {}
sync = data.get("db_sync_mode")
print("true" if isinstance(sync, dict) and sync.get("initial_clone_done") is True else "false")
PY
)"
  db_sync_bucardo_configured="$(python3 - "${EASY_RATHOLE_STATE_FILE}" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    data = {}
sync = data.get("db_sync_mode")
print("true" if isinstance(sync, dict) and sync.get("bucardo_configured") is True else "false")
PY
)"
  forward_ports="$(
    python3 - "${EASY_RATHOLE_STATE_FILE}" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
if not path.exists():
    print("5444, 5480, 5485")
    raise SystemExit(0)

try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    print("5444, 5480, 5485")
    raise SystemExit(0)

rows = data.get("service_ports")
ports = []
seen = set()
for raw_port in data.get("exposed_ports", []):
    try:
        port = int(raw_port)
    except Exception:
        continue
    if port in seen:
        continue
    seen.add(port)
    ports.append(str(port))

if isinstance(rows, list):
    for row in rows:
        if not isinstance(row, dict):
            continue
        if row.get("expose_public") is False:
            continue
        try:
            port = int(row.get("remote_bind_port"))
        except Exception:
            continue
        if port in seen:
            continue
        seen.add(port)
        ports.append(str(port))

if not ports:
    ports = [str(p) for p in data.get("exposed_ports", [5444, 5480, 5485])]

print(", ".join(ports))
PY
  )"

  cat <<EOF

============================================================
Instalasi IPOS5TunnelPublik selesai.

URL Dashboard     : http://${public_ip}:${dashboard_port}
Pengguna Dashboard: ${admin_username}
Sumber Password   : ${credentials_file}

Control Rathole   : ${control_port}
Port Forward      : ${forward_ports}

Services:
  - rathole
  - easy-rathole-dashboard

Root CLI:
  - sudo ${cli_path} status
  - sudo ${cli_path} debug
  - sudo ${cli_path} db-sync finalize

Baseline keamanan:
  - hardening diterapkan : ${hardening_applied}
  - port SSH diizinkan   : ${hardening_ssh_port}

DB sync:
  - aktif                 : ${db_sync_enabled}
  - initial clone         : ${db_sync_clone_done}
  - Bucardo configured    : ${db_sync_bucardo_configured}
  - waiting for client    : ${db_sync_waiting}
  - finalisasi            : dashboard atau sudo ${cli_path} db-sync finalize
============================================================
EOF
}

main "$@"
