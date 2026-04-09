#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/scripts/lib/common.sh"

configure_firewall_ports() {
  local state_file="$1"
  local control_port
  local dashboard_port
  local dashboard_allow_cidr="${EASY_RATHOLE_DASHBOARD_ALLOW_CIDR:-}"

  control_port="$(state_get "$state_file" "rathole_control_port" "0")"
  dashboard_port="$(state_get "$state_file" "dashboard_port" "8088")"

  local remote_ports_json
  remote_ports_json="$(state_get "$state_file" "service_ports" "[]")"
  local ports=("$control_port")
  while IFS= read -r port; do
    [[ -n "$port" ]] || continue
    ports+=("$port")
  done < <(
    python3 - "$remote_ports_json" <<'PY'
import json
import sys

try:
    rows = json.loads(sys.argv[1])
except Exception:
    rows = []

seen = set()
for row in rows:
    if not isinstance(row, dict):
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

main() {
  require_root
  ensure_ubuntu_22_plus

  export EASY_RATHOLE_ROOT="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
  export EASY_RATHOLE_CONFIG_DIR="${EASY_RATHOLE_CONFIG_DIR:-/etc/easy-rathole}"
  export EASY_RATHOLE_STATE_FILE="${EASY_RATHOLE_STATE_FILE:-${EASY_RATHOLE_ROOT}/state/install-state.json}"

  log INFO "Menyiapkan baseline keamanan server..."
  bash "${SCRIPT_DIR}/scripts/prepare_server.sh"

  log INFO "Menginstal dependensi..."
  apt-get update -y
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    curl \
    unzip \
    python3 \
    python3-pip \
    python3-venv \
    ca-certificates \
    systemd \
    iproute2

  log INFO "Menginstal server rathole..."
  bash "${SCRIPT_DIR}/scripts/install_rathole_server.sh"

  log INFO "Menginstal dashboard..."
  bash "${SCRIPT_DIR}/scripts/install_dashboard.sh"

  log INFO "Mengonfigurasi port firewall..."
  configure_firewall_ports "${EASY_RATHOLE_STATE_FILE}"

  local public_ip
  local control_port
  local dashboard_port
  local admin_username
  local credentials_file
  local hardening_applied
  local hardening_ssh_port
  local forward_ports

  public_ip="$(state_get "${EASY_RATHOLE_STATE_FILE}" "public_ip" "<unknown>")"
  control_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "rathole_control_port" "<unknown>")"
  dashboard_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "dashboard_port" "8088")"
  admin_username="$(state_get "${EASY_RATHOLE_STATE_FILE}" "admin_username" "admin")"
  credentials_file="$(state_get "${EASY_RATHOLE_STATE_FILE}" "credentials_file" "${EASY_RATHOLE_ROOT}/state/dashboard-credentials.txt")"
  hardening_applied="$(state_get "${EASY_RATHOLE_STATE_FILE}" "hardening_applied" "false")"
  hardening_ssh_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "hardening_ssh_port" "22")"
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
if isinstance(rows, list):
    seen = set()
    for row in rows:
        if not isinstance(row, dict):
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

Baseline keamanan:
  - hardening diterapkan : ${hardening_applied}
  - port SSH diizinkan   : ${hardening_ssh_port}
============================================================
EOF
}

main "$@"
