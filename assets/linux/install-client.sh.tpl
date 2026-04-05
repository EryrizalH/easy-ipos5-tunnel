#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="{{LINUX_SERVICE_NAME}}"
INSTALL_ROOT="/opt/easy-rathole-client"
BIN_PATH="/usr/local/bin/rathole"
CONFIG_PATH="${INSTALL_ROOT}/client.toml"
WORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

require_root() {
  [[ "$EUID" -eq 0 ]] || {
    echo "Please run with sudo"
    exit 1
  }
}

detect_arch() {
  case "$(uname -m)" in
    x86_64) echo "rathole-x86_64-unknown-linux-gnu.zip" ;;
    aarch64|arm64) echo "rathole-aarch64-unknown-linux-musl.zip" ;;
    *) echo "Unsupported architecture $(uname -m)"; exit 1 ;;
  esac
}

download_rathole() {
  local asset
  asset="$(detect_arch)"
  local api="https://api.github.com/repos/rathole-org/rathole/releases/latest"
  local url
  url="$(curl -fsSL "$api" | python3 -c 'import json,sys
asset=sys.argv[1]
data=json.load(sys.stdin)
for item in data.get("assets", []):
    if item.get("name") == asset:
        print(item.get("browser_download_url", ""))
        break' "$asset")"

  [[ -n "$url" ]] || {
    echo "Failed to resolve rathole download URL"
    exit 1
  }

  local tmp
  tmp="$(mktemp -d)"
  curl -fL "$url" -o "$tmp/rathole.zip"
  unzip -qo "$tmp/rathole.zip" -d "$tmp"
  install -m 0755 "$tmp/rathole" "$BIN_PATH"
  rm -rf "$tmp"
}

install_service() {
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Easy Rathole Client
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
  if ! command -v python3 >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y python3
  fi

  install -d -m 755 "$INSTALL_ROOT"
  install -m 0644 "${WORK_DIR}/client.toml" "$CONFIG_PATH"

  download_rathole
  install_service
  echo "Installed and started service: ${SERVICE_NAME}"
}

main "$@"
