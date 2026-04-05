#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

EASY_RATHOLE_HARDENING="${EASY_RATHOLE_HARDENING:-1}"
EASY_RATHOLE_RUN_UPGRADE="${EASY_RATHOLE_RUN_UPGRADE:-1}"
EASY_RATHOLE_DISABLE_SSH_PASSWORD="${EASY_RATHOLE_DISABLE_SSH_PASSWORD:-0}"
EASY_RATHOLE_SSH_ALLOW_CIDR="${EASY_RATHOLE_SSH_ALLOW_CIDR:-}"

get_ssh_port() {
  local port="22"
  if command -v sshd >/dev/null 2>&1; then
    local detected
    detected="$(sshd -T 2>/dev/null | awk '/^port / {print $2; exit}' || true)"
    if [[ -n "$detected" && "$detected" =~ ^[0-9]+$ ]]; then
      port="$detected"
    fi
  fi
  echo "$port"
}

has_authorized_keys() {
  if [[ -s /root/.ssh/authorized_keys ]]; then
    return 0
  fi

  if find /home -maxdepth 3 -type f -path '*/.ssh/authorized_keys' -size +0c 2>/dev/null | grep -q .; then
    return 0
  fi

  return 1
}

configure_unattended_upgrades() {
  log INFO "Configuring unattended security upgrades..."

  cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF

  cat > /etc/apt/apt.conf.d/52easy-rathole-unattended-upgrades <<'EOF'
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::MailOnlyOnError "true";
EOF

  dpkg-reconfigure -f noninteractive unattended-upgrades >/dev/null 2>&1 || true
}

configure_sysctl_hardening() {
  log INFO "Applying network sysctl hardening..."

  cat > /etc/sysctl.d/99-easy-rathole-hardening.conf <<'EOF'
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.tcp_syncookies = 1
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.default.accept_redirects = 0
EOF

  sysctl --system >/dev/null || true
}

configure_ssh_baseline() {
  log INFO "Applying SSH baseline hardening..."

  local drop_in="/etc/ssh/sshd_config.d/99-easy-rathole-hardening.conf"
  ensure_dir "/etc/ssh/sshd_config.d" 755

  cat > "$drop_in" <<'EOF'
PermitEmptyPasswords no
MaxAuthTries 4
LoginGraceTime 30
X11Forwarding no
AllowAgentForwarding no
ClientAliveInterval 300
ClientAliveCountMax 2
EOF

  if [[ "$EASY_RATHOLE_DISABLE_SSH_PASSWORD" == "1" ]]; then
    if ! has_authorized_keys; then
      fail "EASY_RATHOLE_DISABLE_SSH_PASSWORD=1 but no authorized_keys found. Refusing unsafe SSH hardening."
    fi
    cat >> "$drop_in" <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
EOF
    log INFO "SSH password authentication disabled (key-based auth required)."
  fi

  systemctl reload ssh >/dev/null 2>&1 || systemctl reload sshd >/dev/null 2>&1 || true
}

configure_fail2ban() {
  local ssh_port="$1"
  log INFO "Configuring fail2ban for SSH protection..."

  ensure_dir "/etc/fail2ban/jail.d" 755
  cat > /etc/fail2ban/jail.d/easy-rathole.local <<EOF
[sshd]
enabled = true
port = ${ssh_port}
bantime = 1h
findtime = 10m
maxretry = 5
backend = systemd
EOF

  systemctl enable --now fail2ban >/dev/null 2>&1 || true
  systemctl restart fail2ban >/dev/null 2>&1 || true
}

configure_ufw_baseline() {
  local ssh_port="$1"
  log INFO "Configuring UFW baseline policy..."

  ufw default deny incoming >/dev/null || true
  ufw default allow outgoing >/dev/null || true

  if [[ -n "$EASY_RATHOLE_SSH_ALLOW_CIDR" ]]; then
    ufw allow from "$EASY_RATHOLE_SSH_ALLOW_CIDR" to any port "$ssh_port" proto tcp >/dev/null || true
  else
    ufw allow "${ssh_port}/tcp" >/dev/null || true
  fi

  if ! ufw status 2>/dev/null | grep -q "Status: active"; then
    ufw --force enable >/dev/null
  fi
}

main() {
  require_root
  ensure_ubuntu_22_plus

  if [[ "$EASY_RATHOLE_HARDENING" != "1" ]]; then
    log WARN "Hardening disabled via EASY_RATHOLE_HARDENING=${EASY_RATHOLE_HARDENING}. Skipping server preparation."
    return 0
  fi

  log INFO "Preparing fresh VPS with secure baseline..."
  apt-get update -y
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    curl \
    python3 \
    ufw \
    fail2ban \
    openssh-server \
    unattended-upgrades \
    apt-transport-https

  if [[ "$EASY_RATHOLE_RUN_UPGRADE" == "1" ]]; then
    log INFO "Running security package upgrade..."
    DEBIAN_FRONTEND=noninteractive apt-get upgrade -y
  fi

  local ssh_port
  ssh_port="$(get_ssh_port)"

  configure_unattended_upgrades
  configure_sysctl_hardening
  configure_ssh_baseline
  configure_fail2ban "$ssh_port"
  configure_ufw_baseline "$ssh_port"

  local state_file="${EASY_RATHOLE_STATE_FILE:-/opt/easy-rathole/state/install-state.json}"
  state_merge_json "$state_file" "{\
    \"hardening_applied\": true, \
    \"hardening_ssh_port\": ${ssh_port}, \
    \"hardening_disable_ssh_password\": ${EASY_RATHOLE_DISABLE_SSH_PASSWORD}, \
    \"hardening_ssh_allow_cidr\": \"${EASY_RATHOLE_SSH_ALLOW_CIDR}\"\
  }"

  log INFO "Server preparation completed."
}

main "$@"
