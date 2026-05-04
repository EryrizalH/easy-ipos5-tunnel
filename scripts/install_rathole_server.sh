#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

main() {
  require_root
  ensure_ubuntu_22_plus
  ensure_command curl
  ensure_command unzip
  ensure_command python3
  ensure_command ss
  ensure_command systemctl

  local easy_root="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
  local config_dir="${EASY_RATHOLE_CONFIG_DIR:-/etc/easy-rathole}"
  local state_file="${EASY_RATHOLE_STATE_FILE:-${easy_root}/state/install-state.json}"
  local state_dir
  state_dir="$(dirname "$state_file")"
  local cache_dir="${easy_root}/cache"
  local resources_dir="${easy_root}/resources"
  local rathole_bin="/usr/local/bin/rathole"
  local rathole_service="rathole"
  local dashboard_port="${DASHBOARD_PORT:-8088}"

  ensure_dir "$easy_root" 755
  ensure_dir "$state_dir" 750
  ensure_dir "$cache_dir" 750
  ensure_dir "$resources_dir/templates/rathole" 755
  ensure_dir "$config_dir" 750

  install -m 0644 "${PROJECT_ROOT}/templates/rathole/server.toml.tpl" "${resources_dir}/templates/rathole/server.toml.tpl"
  install -m 0644 "${PROJECT_ROOT}/templates/rathole/client.toml.tpl" "${resources_dir}/templates/rathole/client.toml.tpl"

  local control_port
  local token
  local public_ip

  control_port="$(state_get "$state_file" "rathole_control_port" "")"
  token="$(state_get "$state_file" "token" "")"
  public_ip="$(state_get "$state_file" "public_ip" "")"

  [[ -n "$control_port" ]] || control_port="$(pick_random_free_port 20000 45000)"
  [[ -n "$token" ]] || token="$(random_string 40)"
  [[ -n "$public_ip" ]] || public_ip="$(detect_public_ip)"

  local arch
  arch="$(detect_arch)"
  local release_json
  release_json="$(fetch_latest_rathole_release_json)"

  local release_tag
  release_tag="$(echo "$release_json" | get_release_tag)"
  [[ -n "$release_tag" ]] || fail "Tidak dapat menentukan tag rilis rathole"

  local asset_name
  asset_name="$(asset_name_for_linux_arch "$arch")"

  local asset_url
  asset_url="$(echo "$release_json" | get_release_asset_url "$asset_name")"
  [[ -n "$asset_url" ]] || fail "Tidak menemukan URL asset rilis untuk ${asset_name}"

  local zip_file="${cache_dir}/${asset_name}"
  local temp_extract
  temp_extract="$(mktemp -d)"

  log INFO "Mengunduh ${asset_name} (${release_tag})"
  curl -fL "$asset_url" -o "$zip_file"
  unzip -qo "$zip_file" -d "$temp_extract"

  [[ -f "${temp_extract}/rathole" ]] || fail "Downloaded archive does not contain rathole binary"
  install -m 0755 "${temp_extract}/rathole" "$rathole_bin"
  rm -rf "$temp_extract"

  local config_file="${config_dir}/server.toml"
  local service_ports_raw
  local service_ports_json
  local db_service_key
  local db_remote_bind_port
  local pos_http_service_key
  local pos_http_remote_bind_port
  local pos_worker_service_key
  local pos_worker_remote_bind_port

  service_ports_raw="$(state_get "$state_file" "service_ports" "[]")"
  service_ports_json="$(
    python3 - "$service_ports_raw" <<'PY'
import json
import sys

defaults = [
    {
        "name": "db",
        "service_key": "port_5444",
        "protocol": "tcp",
        "remote_bind_port": 5444,
        "client_local_addr": "127.0.0.1:5444",
        "client_local_port": 5444,
    },
    {
        "name": "pos_http",
        "service_key": "port_5480",
        "protocol": "tcp",
        "remote_bind_port": 5480,
        "client_local_addr": "127.0.0.1:5480",
        "client_local_port": 5480,
    },
    {
        "name": "pos_worker",
        "service_key": "port_5485",
        "protocol": "tcp",
        "remote_bind_port": 5485,
        "client_local_addr": "127.0.0.1:5485",
        "client_local_port": 5485,
    },
]

try:
    rows = json.loads(sys.argv[1])
except Exception:
    rows = []

by_name = {}
for row in rows:
    if isinstance(row, dict):
        name = str(row.get("name", "")).strip()
        if name:
            by_name[name] = row

merged = []
for default in defaults:
    row = default.copy()
    provided = by_name.get(default["name"], {})
    for key in ("service_key", "protocol", "client_local_addr"):
        value = provided.get(key)
        if isinstance(value, str) and value.strip():
            row[key] = value.strip()
    for key in ("remote_bind_port", "client_local_port"):
        value = provided.get(key)
        try:
            if value is not None:
                row[key] = int(value)
        except Exception:
            pass
    merged.append(row)

default_names = {row["name"] for row in defaults}
for row in rows:
    if not isinstance(row, dict):
        continue
    name = str(row.get("name", "")).strip()
    if not name or name in default_names:
        continue
    service_key = str(row.get("service_key", "")).strip()
    protocol = str(row.get("protocol", "tcp")).strip().lower() or "tcp"
    client_local_addr = str(row.get("client_local_addr", "")).strip()
    try:
        remote_bind_port = int(row.get("remote_bind_port"))
        client_local_port = int(row.get("client_local_port"))
    except Exception:
        continue
    if not service_key or not client_local_addr:
        continue
    merged.append(
        {
            "name": name,
            "service_key": service_key,
            "protocol": protocol,
            "remote_bind_port": remote_bind_port,
            "client_local_addr": client_local_addr,
            "client_local_port": client_local_port,
        }
    )

print(json.dumps(merged, separators=(",", ":")))
PY
  )"

  db_service_key="$(python3 - "$service_ports_json" <<'PY'
import json
import sys
rows = {row.get("name"): row for row in json.loads(sys.argv[1])}
print(rows.get("db", {}).get("service_key", "port_5444"))
PY
)"
  db_remote_bind_port="$(python3 - "$service_ports_json" <<'PY'
import json
import sys
rows = {row.get("name"): row for row in json.loads(sys.argv[1])}
print(int(rows.get("db", {}).get("remote_bind_port", 5444)))
PY
)"
  pos_http_service_key="$(python3 - "$service_ports_json" <<'PY'
import json
import sys
rows = {row.get("name"): row for row in json.loads(sys.argv[1])}
print(rows.get("pos_http", {}).get("service_key", "port_5480"))
PY
)"
  pos_http_remote_bind_port="$(python3 - "$service_ports_json" <<'PY'
import json
import sys
rows = {row.get("name"): row for row in json.loads(sys.argv[1])}
print(int(rows.get("pos_http", {}).get("remote_bind_port", 5480)))
PY
)"
  pos_worker_service_key="$(python3 - "$service_ports_json" <<'PY'
import json
import sys
rows = {row.get("name"): row for row in json.loads(sys.argv[1])}
print(rows.get("pos_worker", {}).get("service_key", "port_5485"))
PY
)"
  pos_worker_remote_bind_port="$(python3 - "$service_ports_json" <<'PY'
import json
import sys
rows = {row.get("name"): row for row in json.loads(sys.argv[1])}
print(int(rows.get("pos_worker", {}).get("remote_bind_port", 5485)))
PY
)"

  render_template "${resources_dir}/templates/rathole/server.toml.tpl" "$config_file" \
    RATHOLE_CONTROL_PORT "$control_port" \
    GLOBAL_TOKEN "$token" \
    DB_SERVICE_KEY "$db_service_key" \
    DB_REMOTE_BIND_PORT "$db_remote_bind_port" \
    POS_HTTP_SERVICE_KEY "$pos_http_service_key" \
    POS_HTTP_REMOTE_BIND_PORT "$pos_http_remote_bind_port" \
    POS_WORKER_SERVICE_KEY "$pos_worker_service_key" \
    POS_WORKER_REMOTE_BIND_PORT "$pos_worker_remote_bind_port"

  chmod 0640 "$config_file"

  cat > "/etc/systemd/system/${rathole_service}.service" <<EOF
[Unit]
Description=IPOS5TunnelPublik - Rathole Reverse Tunnel Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${rathole_bin} --server ${config_file}
Restart=always
RestartSec=3
Environment=RUST_LOG=info

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$rathole_service" >/dev/null
  systemctl restart "$rathole_service"
  systemctl is-active --quiet "$rathole_service" || fail "Gagal menjalankan service: ${rathole_service}"

  local now
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  state_merge_json "$state_file" "{\
    \"public_ip\": \"${public_ip}\", \
    \"dashboard_port\": ${dashboard_port}, \
    \"rathole_control_port\": ${control_port}, \
    \"token\": \"${token}\", \
    \"rathole_release\": \"${release_tag}\", \
    \"rathole_asset\": \"${asset_name}\", \
    \"rathole_config_path\": \"${config_file}\", \
    \"rathole_service_name\": \"${rathole_service}\", \
    \"service_ports\": ${service_ports_json}, \
    \"exposed_ports\": [${db_remote_bind_port}, ${pos_http_remote_bind_port}, ${pos_worker_remote_bind_port}], \
    \"updated_at\": \"${now}\"\
  }"

  chmod 0600 "$state_file"
  log INFO "Instalasi server rathole selesai."
}

main "$@"
