#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="{{LINUX_SERVICE_NAME}}"
INSTALL_ROOT="/opt/nusatunnel-client"
BIN_PATH="/usr/local/bin/rathole"
CONFIG_PATH="${INSTALL_ROOT}/client.toml"
WORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

require_root() {
  [[ "$EUID" -eq 0 ]] || {
    echo "Silakan jalankan dengan sudo"
    exit 1
  }
}

detect_arch() {
  case "$(uname -m)" in
    x86_64) echo "rathole-x86_64-unknown-linux-gnu.zip" ;;
    aarch64|arm64) echo "rathole-aarch64-unknown-linux-musl.zip" ;;
    *) echo "Arsitektur tidak didukung: $(uname -m)"; exit 1 ;;
  esac
}

download_rathole() {
  local asset
  asset="$(detect_arch)"
  local url="https://github.com/{{RELEASE_REPOSITORY}}/releases/download/{{RELEASE_VERSION}}/$asset"
  local expected
  case "$asset" in
    rathole-x86_64-unknown-linux-gnu.zip) expected="{{LINUX_X64_SHA256}}" ;;
    rathole-aarch64-unknown-linux-musl.zip) expected="{{LINUX_ARM64_SHA256}}" ;;
  esac

  local tmp
  tmp="$(mktemp -d)"
  curl -fL "$url" -o "$tmp/rathole.zip"
  printf '%s  %s\n' "$expected" "$tmp/rathole.zip" | sha256sum -c - >/dev/null
  unzip -qo "$tmp/rathole.zip" -d "$tmp"
  install -m 0755 "$tmp/rathole" "$BIN_PATH"
  rm -rf "$tmp"
}

install_service() {
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=NusaTunnel Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${BIN_PATH} --client ${CONFIG_PATH}
Restart=always
RestartSec=3
Environment=RUST_LOG=info

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now "${SERVICE_NAME}"
}

main() {
  require_root
  if ! command -v curl >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y curl
  fi
  if ! command -v unzip >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y unzip
  fi

  install -d -m 755 "$INSTALL_ROOT"
  download_rathole
  install -m 0644 "${WORK_DIR}/client.toml" "$CONFIG_PATH"
  install_service
  echo "Service berhasil diinstal dan dijalankan: ${SERVICE_NAME}"
}

main "$@"
