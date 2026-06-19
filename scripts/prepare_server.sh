#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

NUSA_TUNNEL_HARDENING="${NUSA_TUNNEL_HARDENING:-1}"
NUSA_TUNNEL_RUN_UPGRADE="${NUSA_TUNNEL_RUN_UPGRADE:-1}"
NUSA_TUNNEL_DISABLE_SSH_PASSWORD="${NUSA_TUNNEL_DISABLE_SSH_PASSWORD:-0}"
NUSA_TUNNEL_SSH_ALLOW_CIDR="${NUSA_TUNNEL_SSH_ALLOW_CIDR:-}"

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
  log INFO "Mengonfigurasi pembaruan keamanan otomatis..."

  cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF

  cat > /etc/apt/apt.conf.d/52nusatunnel-unattended-upgrades <<'EOF'
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::MailOnlyOnError "true";
EOF

  dpkg-reconfigure -f noninteractive unattended-upgrades >/dev/null 2>&1 || true
}

configure_sysctl_hardening() {
  log INFO "Menerapkan hardening sysctl jaringan..."

  cat > /etc/sysctl.d/99-nusatunnel-hardening.conf <<'EOF'
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
  log INFO "Menerapkan hardening baseline SSH..."

  local drop_in="/etc/ssh/sshd_config.d/99-nusatunnel-hardening.conf"
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

  if [[ "$NUSA_TUNNEL_DISABLE_SSH_PASSWORD" == "1" ]]; then
    if ! has_authorized_keys; then
      fail "NUSA_TUNNEL_DISABLE_SSH_PASSWORD=1 tetapi authorized_keys tidak ditemukan. Hardening SSH dibatalkan demi keamanan."
    fi
    cat >> "$drop_in" <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
EOF
    log INFO "Autentikasi SSH berbasis password dinonaktifkan (wajib key-based auth)."
  fi

  systemctl reload ssh >/dev/null 2>&1 || systemctl reload sshd >/dev/null 2>&1 || true
}

configure_fail2ban() {
  local ssh_port="$1"
  log INFO "Mengonfigurasi fail2ban untuk proteksi SSH..."

  ensure_dir "/etc/fail2ban/jail.d" 755
  cat > /etc/fail2ban/jail.d/nusatunnel.local <<EOF
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
  log INFO "Mengonfigurasi kebijakan baseline UFW..."

  ufw default deny incoming >/dev/null || true
  ufw default allow outgoing >/dev/null || true

  if [[ -n "$NUSA_TUNNEL_SSH_ALLOW_CIDR" ]]; then
    ufw allow from "$NUSA_TUNNEL_SSH_ALLOW_CIDR" to any port "$ssh_port" proto tcp >/dev/null || true
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

  if [[ "$NUSA_TUNNEL_HARDENING" != "1" ]]; then
    log WARN "Hardening dinonaktifkan via NUSA_TUNNEL_HARDENING=${NUSA_TUNNEL_HARDENING}. Melewati persiapan server."
    return 0
  fi

  log INFO "Menyiapkan VPS dengan baseline keamanan..."
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

  if [[ "$NUSA_TUNNEL_RUN_UPGRADE" == "1" ]]; then
    log INFO "Menjalankan upgrade paket keamanan..."
    DEBIAN_FRONTEND=noninteractive apt-get upgrade -y
  fi

  local ssh_port
  ssh_port="$(get_ssh_port)"

  configure_unattended_upgrades
  configure_sysctl_hardening
  configure_ssh_baseline
  configure_fail2ban "$ssh_port"
  configure_ufw_baseline "$ssh_port"

  local state_file="${NUSA_TUNNEL_STATE_FILE:-/opt/nusatunnel/state/install-state.json}"
  state_merge_json "$state_file" "{\
    \"hardening_applied\": true, \
    \"hardening_ssh_port\": ${ssh_port}, \
    \"hardening_disable_ssh_password\": ${NUSA_TUNNEL_DISABLE_SSH_PASSWORD}, \
    \"hardening_ssh_allow_cidr\": \"${NUSA_TUNNEL_SSH_ALLOW_CIDR}\"\
  }"

  log INFO "Persiapan server selesai."
}

main "$@"
