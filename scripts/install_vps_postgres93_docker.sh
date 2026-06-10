#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

json_get() {
  local state_file="$1"
  local expr="$2"
  local fallback="$3"
  python3 - "$state_file" "$expr" "$fallback" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
expr = sys.argv[2].split(".")
fallback = sys.argv[3]

try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    print(fallback)
    raise SystemExit(0)

value = data
for key in expr:
    if not isinstance(value, dict) or key not in value:
        print(fallback)
        raise SystemExit(0)
    value = value[key]

if isinstance(value, (dict, list)):
    print(json.dumps(value, separators=(",", ":")))
elif isinstance(value, bool):
    print("true" if value else "false")
else:
    print(value)
PY
}

split_host_port() {
  local addr="$1"
  local fallback_host="$2"
  local fallback_port="$3"
  python3 - "$addr" "$fallback_host" "$fallback_port" <<'PY'
import sys

addr = sys.argv[1].strip()
fallback_host = sys.argv[2]
fallback_port = sys.argv[3]

if ":" not in addr:
    print(fallback_host)
    print(fallback_port)
    raise SystemExit(0)

host, port = addr.rsplit(":", 1)
host = host.strip() or fallback_host
port = port.strip() if port.strip().isdigit() else fallback_port
print(host)
print(port)
PY
}

main() {
  require_root
  ensure_ubuntu_22_plus
  ensure_command python3

  local easy_root="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
  local state_file="${EASY_RATHOLE_STATE_FILE:-${easy_root}/state/install-state.json}"
  local data_dir="${EASY_RATHOLE_VPS_DB_DATA_DIR:-${easy_root}/postgres93/data}"
  local container_name="${EASY_RATHOLE_VPS_DB_CONTAINER:-postgres93}"
  local image="${EASY_RATHOLE_VPS_DB_IMAGE:-postgres:9.3}"
  local vps_addr
  if [[ -n "${EASY_RATHOLE_VPS_DB_ADDR:-}" ]]; then
    vps_addr="$EASY_RATHOLE_VPS_DB_ADDR"
  else
    vps_addr="$(json_get "$state_file" "db_sync_mode.vps_db_addr" "${EASY_RATHOLE_VPS_DB_BIND_HOST:-0.0.0.0}:${EASY_RATHOLE_VPS_DB_PORT:-5444}")"
    if [[ "$vps_addr" == "127.0.0.1:5444" || "$vps_addr" == "localhost:5444" ]]; then
      vps_addr="${EASY_RATHOLE_VPS_DB_BIND_HOST:-0.0.0.0}:${EASY_RATHOLE_VPS_DB_PORT:-5444}"
    fi
  fi

  mapfile -t addr_parts < <(split_host_port "$vps_addr" "${EASY_RATHOLE_VPS_DB_BIND_HOST:-0.0.0.0}" "${EASY_RATHOLE_VPS_DB_PORT:-5444}")
  local bind_host="${addr_parts[0]}"
  local bind_port="${addr_parts[1]}"

  local dbname="${EASY_RATHOLE_VPS_DB_NAME:-$(json_get "$state_file" "db_sync_mode.dbname" "postgres")}"
  local dbuser="${EASY_RATHOLE_VPS_DB_USER:-$(json_get "$state_file" "db_sync_mode.vps_db_user" "sysi5adm")}"
  local dbpass="${EASY_RATHOLE_VPS_DB_PASSWORD:-$(json_get "$state_file" "db_sync_mode.vps_db_password" "u&aV23cc.o82dtr1x89c")}"

  log INFO "Menginstal prerequisite Docker untuk PostgreSQL 9.3 VPS..."
  apt-get update -y
  DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io ca-certificates
  systemctl enable docker >/dev/null
  systemctl start docker
  ensure_command docker

  ensure_dir "$data_dir" 700

  if docker ps -a --format '{{.Names}}' | grep -Fxq "$container_name"; then
    log INFO "Container ${container_name} sudah ada; memastikan status running."
    docker start "$container_name" >/dev/null
  else
    log INFO "Menarik image PostgreSQL VPS: ${image}"
    if ! docker pull "$image"; then
      fail "Gagal pull ${image}. Jika tag postgres:9.3 tidak tersedia di registry, set EASY_RATHOLE_VPS_DB_IMAGE ke image PostgreSQL 9.3 yang sudah dipin/tersedia."
    fi

    log INFO "Membuat container ${container_name} (${bind_host}:${bind_port} -> 5432)"
    docker run -d \
      --name "$container_name" \
      --restart unless-stopped \
      -e POSTGRES_USER="$dbuser" \
      -e POSTGRES_PASSWORD="$dbpass" \
      -e POSTGRES_DB="$dbname" \
      -v "${data_dir}:/var/lib/postgresql/data" \
      -p "${bind_host}:${bind_port}:5432" \
      "$image" >/dev/null
  fi

  log INFO "Menunggu PostgreSQL VPS siap di ${bind_host}:${bind_port}/${dbname}"
  local attempts=0
  until docker exec -e PGPASSWORD="$dbpass" "$container_name" \
    psql -U "$dbuser" -d "$dbname" -c "SELECT 1;" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if (( attempts > 60 )); then
      fail "PostgreSQL VPS container belum ready setelah timeout. Cek: docker logs ${container_name}"
    fi
    sleep 2
  done

  local now
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  python3 - "$state_file" "$bind_host" "$bind_port" "$image" "$container_name" "$data_dir" "$dbname" "$dbuser" "$dbpass" "$now" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
bind_host, bind_port, image, container_name, data_dir, dbname, dbuser, dbpass, now = sys.argv[2:11]

try:
    data = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {}
except Exception:
    data = {}

sync = data.get("db_sync_mode")
if not isinstance(sync, dict):
    sync = {}

sync.update(
    {
        "enabled": True,
        "vps_db_addr": f"{bind_host}:{bind_port}",
        "private_db_tunnel_addr": sync.get("private_db_tunnel_addr") or "127.0.0.1:5445",
        "private_db_backend_mode": sync.get("private_db_backend_mode") or "direct",
        "dbname": dbname,
        "vps_db_user": dbuser,
        "vps_db_password": dbpass,
        "private_db_user": sync.get("private_db_user") or dbuser,
        "private_db_password": sync.get("private_db_password") or dbpass,
        "initial_clone_done": bool(sync.get("initial_clone_done", False)),
        "bucardo_configured": bool(sync.get("bucardo_configured", False)),
        "waiting_for_client": bool(sync.get("waiting_for_client", False)),
    }
)

data["vps_postgres"] = {
    "installed": True,
    "engine": "docker",
    "image": image,
    "container_name": container_name,
    "bind_addr": f"{bind_host}:{bind_port}",
    "data_dir": data_dir,
    "dbname": dbname,
    "user": dbuser,
}
data["db_sync_mode"] = sync
data["updated_at"] = now
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
PY
  chmod 0600 "$state_file"
  log INFO "PostgreSQL 9.3 VPS siap di ${bind_host}:${bind_port}."
}

main "$@"
