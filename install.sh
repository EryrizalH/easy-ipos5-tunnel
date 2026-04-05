#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/scripts/lib/common.sh"

configure_firewall_ports() {
  local state_file="$1"
  local control_port
  local dashboard_port

  control_port="$(state_get "$state_file" "rathole_control_port" "0")"
  dashboard_port="$(state_get "$state_file" "dashboard_port" "8088")"

  local ports=("$control_port" "5444" "5480" "5485" "$dashboard_port")

  if command -v ufw >/dev/null 2>&1; then
    if ufw status 2>/dev/null | grep -q "Status: active"; then
      log INFO "Configuring UFW rules..."
      for p in "${ports[@]}"; do
        [[ "$p" =~ ^[0-9]+$ ]] || continue
        ufw allow "${p}/tcp" >/dev/null || true
      done
    else
      log WARN "UFW is installed but inactive. Skipping firewall automation."
    fi
  elif command -v firewall-cmd >/dev/null 2>&1; then
    if systemctl is-active --quiet firewalld; then
      log INFO "Configuring firewalld rules..."
      for p in "${ports[@]}"; do
        [[ "$p" =~ ^[0-9]+$ ]] || continue
        firewall-cmd --permanent --add-port="${p}/tcp" >/dev/null || true
      done
      firewall-cmd --reload >/dev/null || true
    else
      log WARN "firewalld detected but not active. Skipping firewall automation."
    fi
  else
    log WARN "No supported firewall manager detected (ufw/firewalld)."
  fi
}

main() {
  require_root
  ensure_ubuntu_22_plus

  log INFO "Installing dependencies..."
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

  export EASY_RATHOLE_ROOT="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
  export EASY_RATHOLE_CONFIG_DIR="${EASY_RATHOLE_CONFIG_DIR:-/etc/easy-rathole}"
  export EASY_RATHOLE_STATE_FILE="${EASY_RATHOLE_STATE_FILE:-${EASY_RATHOLE_ROOT}/state/install-state.json}"

  log INFO "Installing rathole server..."
  bash "${SCRIPT_DIR}/scripts/install_rathole_server.sh"

  log INFO "Installing dashboard..."
  bash "${SCRIPT_DIR}/scripts/install_dashboard.sh"

  log INFO "Configuring firewall ports..."
  configure_firewall_ports "${EASY_RATHOLE_STATE_FILE}"

  local public_ip
  local control_port
  local dashboard_port
  local admin_username
  local credentials_file

  public_ip="$(state_get "${EASY_RATHOLE_STATE_FILE}" "public_ip" "<unknown>")"
  control_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "rathole_control_port" "<unknown>")"
  dashboard_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "dashboard_port" "8088")"
  admin_username="$(state_get "${EASY_RATHOLE_STATE_FILE}" "admin_username" "admin")"
  credentials_file="$(state_get "${EASY_RATHOLE_STATE_FILE}" "credentials_file" "${EASY_RATHOLE_ROOT}/state/dashboard-credentials.txt")"

  cat <<EOF

============================================================
Easy Rathole installation completed.

Dashboard URL   : http://${public_ip}:${dashboard_port}
Dashboard user  : ${admin_username}
Password source : ${credentials_file}

Rathole control : ${control_port}
Forwarded ports : 5444, 5480, 5485

Services:
  - rathole
  - easy-rathole-dashboard
============================================================
EOF
}

main "$@"
