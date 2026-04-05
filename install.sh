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

  local ports=("$control_port" "5444" "5480" "5485")

  if command -v ufw >/dev/null 2>&1; then
    if ufw status 2>/dev/null | grep -q "Status: active"; then
      log INFO "Configuring UFW rules..."
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
      log WARN "UFW is installed but inactive. Skipping firewall automation."
    fi
  elif command -v firewall-cmd >/dev/null 2>&1; then
    if systemctl is-active --quiet firewalld; then
      log INFO "Configuring firewalld rules..."
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
      log WARN "firewalld detected but not active. Skipping firewall automation."
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

  log INFO "Preparing server security baseline..."
  bash "${SCRIPT_DIR}/scripts/prepare_server.sh"

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
  local hardening_applied
  local hardening_ssh_port

  public_ip="$(state_get "${EASY_RATHOLE_STATE_FILE}" "public_ip" "<unknown>")"
  control_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "rathole_control_port" "<unknown>")"
  dashboard_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "dashboard_port" "8088")"
  admin_username="$(state_get "${EASY_RATHOLE_STATE_FILE}" "admin_username" "admin")"
  credentials_file="$(state_get "${EASY_RATHOLE_STATE_FILE}" "credentials_file" "${EASY_RATHOLE_ROOT}/state/dashboard-credentials.txt")"
  hardening_applied="$(state_get "${EASY_RATHOLE_STATE_FILE}" "hardening_applied" "false")"
  hardening_ssh_port="$(state_get "${EASY_RATHOLE_STATE_FILE}" "hardening_ssh_port" "22")"

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

Security baseline:
  - hardening applied : ${hardening_applied}
  - SSH port allowed  : ${hardening_ssh_port}
============================================================
EOF
}

main "$@"
