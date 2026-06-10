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

if isinstance(value, bool):
    print("true" if value else "false")
elif isinstance(value, (dict, list)):
    print(json.dumps(value, separators=(",", ":")))
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

connect_host_for_bind() {
  local host="$1"
  case "$host" in
    "0.0.0.0"|"::"|"[::]") echo "127.0.0.1" ;;
    *) echo "$host" ;;
  esac
}

sql_literal() {
  python3 - "$1" <<'PY'
import sys
print("'" + sys.argv[1].replace("'", "''") + "'")
PY
}

sql_ident() {
  python3 - "$1" <<'PY'
import sys
print('"' + sys.argv[1].replace('"', '""') + '"')
PY
}

name_slug() {
  python3 - "$1" <<'PY'
import hashlib
import re
import sys

name = sys.argv[1]
slug = re.sub(r"[^a-zA-Z0-9_]+", "_", name).strip("_").lower()
if not slug:
    slug = "db"
digest = hashlib.sha1(name.encode("utf-8")).hexdigest()[:10]
print((slug[:32] + "_" + digest)[:48])
PY
}

json_array_contains() {
  local json="$1"
  local needle="$2"
  python3 - "$json" "$needle" <<'PY'
import json
import sys

try:
    items = json.loads(sys.argv[1])
except Exception:
    items = []
print("true" if sys.argv[2] in items else "false")
PY
}

update_sync_state() {
  local state_file="$1"
  local patch_json="$2"
  python3 - "$state_file" "$patch_json" <<'PY'
import json
import pathlib
import sys
from datetime import UTC, datetime

path = pathlib.Path(sys.argv[1])
patch = json.loads(sys.argv[2])

try:
    data = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {}
except Exception:
    data = {}

sync = data.get("db_sync_mode")
if not isinstance(sync, dict):
    sync = {}
sync.update(patch)
data["db_sync_mode"] = sync
data["updated_at"] = datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
PY
  chmod 0600 "$state_file"
}

update_db_record() {
  local state_file="$1"
  local dbname="$2"
  local status="$3"
  local sync_name="$4"
  local message="$5"
  python3 - "$state_file" "$dbname" "$status" "$sync_name" "$message" <<'PY'
import json
import pathlib
import sys
from datetime import UTC, datetime

path = pathlib.Path(sys.argv[1])
dbname, status, sync_name, message = sys.argv[2:6]
now = datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")

try:
    data = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {}
except Exception:
    data = {}

sync = data.get("db_sync_mode")
if not isinstance(sync, dict):
    sync = {}

records = sync.get("databases")
if not isinstance(records, list):
    records = []

by_name = {row.get("name"): row for row in records if isinstance(row, dict) and row.get("name")}
row = dict(by_name.get(dbname, {}))
row.update(
    {
        "name": dbname,
        "status": status,
        "sync_name": sync_name,
        "last_checked_at": now,
    }
)
if status == "synced":
    row["last_synced_at"] = now
    row["last_error"] = ""
elif status == "dropped":
    row["dropped_at"] = now
    row["last_error"] = ""
else:
    row["last_error"] = message
if message:
    row["message"] = message

by_name[dbname] = row
sync["databases"] = [by_name[key] for key in sorted(by_name)]
data["db_sync_mode"] = sync
data["updated_at"] = now
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
PY
  chmod 0600 "$state_file"
}

registered_db_names_json() {
  local state_file="$1"
  python3 - "$state_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8"))
except Exception:
    data = {}
sync = data.get("db_sync_mode")
records = sync.get("databases") if isinstance(sync, dict) else []
names = []
if isinstance(records, list):
    for row in records:
        if not isinstance(row, dict):
            continue
        if row.get("name") and row.get("status") == "synced" and row.get("sync_name"):
            names.append(row["name"])
print(json.dumps(sorted(set(names)), separators=(",", ":")))
PY
}

psql_exec() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local sql="$6"
  PGPASSWORD="$password" psql -v ON_ERROR_STOP=1 \
    "host=${host} port=${port} dbname=${dbname} user=${user}" \
    -c "$sql" >/dev/null
}

psql_scalar() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local sql="$6"
  PGPASSWORD="$password" psql -At -v ON_ERROR_STOP=1 \
    "host=${host} port=${port} dbname=${dbname} user=${user}" \
    -c "$sql"
}

dump_roles_best_effort() {
  local host="$1"
  local port="$2"
  local user="$3"
  local password="$4"
  local output_file="$5"

  if PGPASSWORD="$password" pg_dumpall --roles-only -h "$host" -p "$port" -U "$user" >"$output_file" 2>"${output_file}.err"; then
    return 0
  fi
  log WARN "Dump roles dari ${host}:${port} gagal; clone data tetap dilanjutkan. Detail: ${output_file}.err"
  return 1
}

restore_roles_best_effort() {
  local host="$1"
  local port="$2"
  local user="$3"
  local password="$4"
  local input_file="$5"

  [[ -s "$input_file" ]] || return 0
  if PGPASSWORD="$password" psql "host=${host} port=${port} dbname=postgres user=${user}" -f "$input_file" >/dev/null 2>"${input_file}.restore.err"; then
    return 0
  fi
  log WARN "Restore roles ke ${host}:${port} tidak sepenuhnya berhasil; lanjut restore data. Detail: ${input_file}.restore.err"
  return 0
}

database_exists() {
  local host="$1"
  local port="$2"
  local user="$3"
  local password="$4"
  local dbname="$5"
  local literal
  literal="$(sql_literal "$dbname")"
  local exists
  exists="$(psql_scalar "$host" "$port" "postgres" "$user" "$password" "SELECT 1 FROM pg_database WHERE datname=${literal};" || true)"
  [[ "$exists" == "1" ]]
}

list_user_databases() {
  local host="$1"
  local port="$2"
  local user="$3"
  local password="$4"
  local exclude_csv="$5"
  local sql="SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn = true ORDER BY datname;"
  local rows
  rows="$(psql_scalar "$host" "$port" "postgres" "$user" "$password" "$sql")"
  python3 - "$exclude_csv" "$rows" <<'PY'
import sys

exclude = {item.strip() for item in sys.argv[1].split(",") if item.strip()}
for name in sys.argv[2].splitlines():
    name = name.strip()
    if name and name not in exclude:
        print(name)
PY
}

user_object_count() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  psql_scalar "$host" "$port" "$dbname" "$user" "$password" \
    "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema','bucardo') AND n.nspname NOT LIKE 'pg_toast%' AND c.relkind IN ('r','p','v','m','S','f');"
}

union_database_names() {
  local left="$1"
  local right="$2"
  local registered="$3"
  python3 - "$left" "$right" "$registered" <<'PY'
import json
import sys

names = set()
for raw in (sys.argv[1], sys.argv[2]):
    for line in raw.splitlines():
        line = line.strip()
        if line:
            names.add(line)
try:
    names.update(json.loads(sys.argv[3]))
except Exception:
    pass
for name in sorted(names):
    print(name)
PY
}

create_database() {
  local host="$1"
  local port="$2"
  local user="$3"
  local password="$4"
  local dbname="$5"
  local ident
  ident="$(sql_ident "$dbname")"
  log INFO "Membuat database ${dbname} di ${host}:${port}"
  psql_exec "$host" "$port" "postgres" "$user" "$password" "CREATE DATABASE ${ident} TEMPLATE template0;"
}

drop_database() {
  local host="$1"
  local port="$2"
  local user="$3"
  local password="$4"
  local dbname="$5"
  local ident literal
  ident="$(sql_ident "$dbname")"
  literal="$(sql_literal "$dbname")"
  log WARN "Mirror drop database ${dbname} di ${host}:${port}"
  psql_exec "$host" "$port" "postgres" "$user" "$password" "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=${literal} AND pid <> pg_backend_pid();"
  psql_exec "$host" "$port" "postgres" "$user" "$password" "DROP DATABASE IF EXISTS ${ident};"
}

clone_database() {
  local backup_dir="$1"
  local source_label="$2"
  local source_host="$3"
  local source_port="$4"
  local source_user="$5"
  local source_pass="$6"
  local target_label="$7"
  local target_host="$8"
  local target_port="$9"
  local target_user="${10}"
  local target_pass="${11}"
  local dbname="${12}"

  ensure_dir "$backup_dir" 700
  local stamp safe dump_file
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  safe="$(name_slug "$dbname")"
  dump_file="${backup_dir}/${safe}-${source_label}-to-${target_label}-${stamp}.dump"

  log INFO "Clone DB ${dbname}: ${source_label} ${source_host}:${source_port} -> ${target_label} ${target_host}:${target_port}"
  PGPASSWORD="$source_pass" pg_dump -Fc \
    -h "$source_host" \
    -p "$source_port" \
    -U "$source_user" \
    -d "$dbname" \
    -f "$dump_file"

  local restore_status=0
  PGPASSWORD="$target_pass" pg_restore --clean --if-exists \
    -h "$target_host" \
    -p "$target_port" \
    -U "$target_user" \
    -d "$dbname" \
    "$dump_file" || restore_status=$?

  if (( restore_status > 1 )); then
    fail "pg_restore ${dbname} gagal total dengan exit code ${restore_status}."
  elif (( restore_status == 1 )); then
    log WARN "pg_restore ${dbname} selesai dengan beberapa peringatan/error non-fatal."
  fi
}

sync_schema_best_effort() {
  local backup_dir="$1"
  local source_label="$2"
  local source_host="$3"
  local source_port="$4"
  local source_user="$5"
  local source_pass="$6"
  local target_label="$7"
  local target_host="$8"
  local target_port="$9"
  local target_user="${10}"
  local target_pass="${11}"
  local dbname="${12}"

  ensure_dir "$backup_dir" 700
  local stamp safe schema_file
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  safe="$(name_slug "$dbname")"
  schema_file="${backup_dir}/${safe}-${source_label}-to-${target_label}-schema-${stamp}.sql"

  if ! PGPASSWORD="$source_pass" pg_dump --schema-only --no-owner --no-privileges \
    -h "$source_host" \
    -p "$source_port" \
    -U "$source_user" \
    -d "$dbname" \
    -f "$schema_file"; then
    log WARN "Dump schema ${dbname} dari ${source_label} gagal; auto-register tetap dicoba."
    return 0
  fi

  if ! PGPASSWORD="$target_pass" psql \
    "host=${target_host} port=${target_port} dbname=${dbname} user=${target_user}" \
    -f "$schema_file" >/dev/null 2>"${schema_file}.restore.err"; then
    log WARN "Restore schema ${dbname} ke ${target_label} tidak sepenuhnya berhasil; objek yang sudah ada biasanya aman diabaikan. Detail: ${schema_file}.restore.err"
  fi
}

apply_sequence_policy() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local start_value="$6"

  PGPASSWORD="$password" psql -v ON_ERROR_STOP=1 \
    "host=${host} port=${port} dbname=${dbname} user=${user}" <<SQL
DO \$\$
DECLARE
  seq record;
  current_value bigint;
  next_value bigint;
BEGIN
  FOR seq IN
    SELECT sequence_schema, sequence_name
    FROM information_schema.sequences
    WHERE sequence_schema NOT IN ('pg_catalog', 'information_schema', 'bucardo')
  LOOP
    EXECUTE format('ALTER SEQUENCE %I.%I INCREMENT BY 2', seq.sequence_schema, seq.sequence_name);
    EXECUTE format('SELECT last_value FROM %I.%I', seq.sequence_schema, seq.sequence_name) INTO current_value;
    next_value := GREATEST(current_value, ${start_value});
    IF mod(next_value, 2) <> mod(${start_value}, 2) THEN
      next_value := next_value + 1;
    END IF;
    EXECUTE format('SELECT setval(%L, %s, true)', seq.sequence_schema || '.' || seq.sequence_name, next_value);
  END LOOP;
END
\$\$;
SQL
}

ensure_bucardo_control() {
  systemctl start postgresql || true
  rm -f /etc/bucardorc /root/.bucardorc || true
  local postgres_home
  postgres_home="$(getent passwd postgres | cut -d: -f6)"
  if [[ -n "$postgres_home" ]]; then
    rm -f "${postgres_home}/.bucardorc" || true
  fi

  local bucardo_db_exists
  bucardo_db_exists=$(sudo -u postgres psql -t -A -c "SELECT 1 FROM pg_database WHERE datname='bucardo'" 2>/dev/null || echo "0")
  if [[ "${EASY_RATHOLE_RESET_BUCARDO:-0}" == "1" ]]; then
    log WARN "EASY_RATHOLE_RESET_BUCARDO=1 terdeteksi. Menghapus database kontrol Bucardo lama..."
    sudo -u postgres psql -c "DROP DATABASE IF EXISTS bucardo;" || true
    bucardo_db_exists="0"
  fi

  if [[ "$bucardo_db_exists" != "1" ]]; then
    log INFO "Inisialisasi database kontrol Bucardo..."
    sudo -u postgres bucardo install --batch --quiet --dbuser=postgres
  else
    log INFO "Database kontrol Bucardo sudah terdeteksi. Menjalankan upgrade/validasi..."
    sudo -u postgres bucardo upgrade --batch --quiet || true
  fi

  sudo -u postgres psql -c "ALTER USER bucardo WITH PASSWORD 'bucardo';" || true
  cat <<EOF >/etc/bucardorc
dbhost=127.0.0.1
dbport=5432
dbuser=bucardo
dbpass=bucardo
EOF
  cp /etc/bucardorc /root/.bucardorc || true
  postgres_home="$(getent passwd postgres | cut -d: -f6)"
  if [[ -n "$postgres_home" && -d "$postgres_home" ]]; then
    cp /etc/bucardorc "${postgres_home}/.bucardorc" || true
    chown postgres:postgres "${postgres_home}/.bucardorc" || true
    chmod 0600 "${postgres_home}/.bucardorc" || true
  fi
  chmod 0600 /etc/bucardorc /root/.bucardorc || true
}

remove_bucardo_objects() {
  local dbname="$1"
  local slug
  slug="$(name_slug "$dbname")"
  bucardo remove sync "ipos5_2way_${slug}" --force >/dev/null 2>&1 || true
  bucardo remove dbgroup "ipos5_2way_dbs_${slug}" --force >/dev/null 2>&1 || true
  bucardo remove relgroup "ipos5_2way_rel_${slug}" --force >/dev/null 2>&1 || true
  bucardo remove db "vps_${slug}" --force >/dev/null 2>&1 || true
  bucardo remove db "private_${slug}" --force >/dev/null 2>&1 || true
}

register_bucardo_sync() {
  local dbname="$1"
  local vps_host="$2"
  local vps_port="$3"
  local vps_user="$4"
  local vps_pass="$5"
  local private_host="$6"
  local private_port="$7"
  local private_user="$8"
  local private_pass="$9"

  local slug sync_name dbgroup relgroup vps_db private_db
  slug="$(name_slug "$dbname")"
  sync_name="ipos5_2way_${slug}"
  dbgroup="ipos5_2way_dbs_${slug}"
  relgroup="ipos5_2way_rel_${slug}"
  vps_db="vps_${slug}"
  private_db="private_${slug}"

  log INFO "Mendaftarkan Bucardo untuk database ${dbname}"
  remove_bucardo_objects "$dbname"
  bucardo add db "$vps_db" dbhost="$vps_host" dbport="$vps_port" dbname="$dbname" dbuser="$vps_user" dbpass="$vps_pass"
  bucardo add db "$private_db" dbhost="$private_host" dbport="$private_port" dbname="$dbname" dbuser="$private_user" dbpass="$private_pass"
  bucardo add dbgroup "$dbgroup" "${vps_db}:source" "${private_db}:source"
  bucardo add all tables db="$vps_db" relgroup="$relgroup"
  bucardo add all sequences db="$vps_db" relgroup="$relgroup"
  bucardo add sync "$sync_name" relgroup="$relgroup" dbs="$dbgroup" conflict_strategy=bucardo_latest
  bucardo validate sync "$sync_name"
  echo "$sync_name"
}

sync_registered_objects() {
  local dbname="$1"
  local sync_name="$2"
  local slug relgroup vps_db
  slug="$(name_slug "$dbname")"
  relgroup="ipos5_2way_rel_${slug}"
  vps_db="vps_${slug}"
  bucardo add all tables db="$vps_db" relgroup="$relgroup" >/dev/null 2>&1 || true
  bucardo add all sequences db="$vps_db" relgroup="$relgroup" >/dev/null 2>&1 || true
  bucardo update sync "$sync_name" onetimecopy=2 >/dev/null 2>&1 || true
  bucardo validate sync "$sync_name"
}

finalize_aggregate_state() {
  local state_file="$1"
  python3 - "$state_file" <<'PY'
import json
import pathlib
import sys
from datetime import UTC, datetime

path = pathlib.Path(sys.argv[1])
try:
    data = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {}
except Exception:
    data = {}
sync = data.get("db_sync_mode")
if not isinstance(sync, dict):
    sync = {}
records = sync.get("databases")
if not isinstance(records, list):
    records = []
active = [row for row in records if isinstance(row, dict) and row.get("status") != "dropped"]
errors = [row for row in active if row.get("status") == "error"]
pending = [row for row in active if row.get("status") in {"pending_clone", "pending_register"}]
sync["initial_clone_done"] = bool(active) and not pending and not errors
sync["bucardo_configured"] = bool(active) and not pending and not errors
sync["waiting_for_client"] = False
sync["last_discovery_at"] = datetime.now(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
sync["last_error"] = errors[0].get("last_error", "") if errors else ""
data["db_sync_mode"] = sync
data["updated_at"] = sync["last_discovery_at"]
path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
PY
  chmod 0600 "$state_file"
}

main() {
  require_root
  ensure_ubuntu_22_plus
  ensure_command python3

  local easy_root="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
  local state_file="${EASY_RATHOLE_STATE_FILE:-${easy_root}/state/install-state.json}"
  local backup_dir="${EASY_RATHOLE_DB_SYNC_BACKUP_DIR:-${easy_root}/backups}"
  [[ -f "$state_file" ]] || fail "State file tidak ditemukan: ${state_file}"

  local sync_enabled
  sync_enabled="$(json_get "$state_file" "db_sync_mode.enabled" "false")"
  [[ "$sync_enabled" == "true" ]] || fail "db_sync_mode.enabled belum true di ${state_file}"

  local vps_addr private_addr
  vps_addr="${EASY_RATHOLE_VPS_DB_ADDR:-$(json_get "$state_file" "db_sync_mode.vps_db_addr" "0.0.0.0:5444")}"
  private_addr="${EASY_RATHOLE_SYNC_PRIVATE_ADDR:-$(json_get "$state_file" "db_sync_mode.private_db_tunnel_addr" "127.0.0.1:5445")}"

  mapfile -t vps_parts < <(split_host_port "$vps_addr" "0.0.0.0" "5444")
  mapfile -t private_parts < <(split_host_port "$private_addr" "127.0.0.1" "5445")
  local vps_bind_host="${vps_parts[0]}"
  local vps_port="${vps_parts[1]}"
  local private_host="${private_parts[0]}"
  local private_port="${private_parts[1]}"
  local vps_host
  vps_host="$(connect_host_for_bind "$vps_bind_host")"

  local vps_user="${EASY_RATHOLE_SYNC_VPS_USER:-$(json_get "$state_file" "db_sync_mode.vps_db_user" "sysi5adm")}"
  local vps_pass="${EASY_RATHOLE_SYNC_VPS_PASSWORD:-$(json_get "$state_file" "db_sync_mode.vps_db_password" "u&aV23cc.o82dtr1x89c")}"
  local private_user="${EASY_RATHOLE_SYNC_PRIVATE_USER:-$(json_get "$state_file" "db_sync_mode.private_db_user" "$vps_user")}"
  local private_pass="${EASY_RATHOLE_SYNC_PRIVATE_PASSWORD:-$(json_get "$state_file" "db_sync_mode.private_db_password" "$vps_pass")}"
  local exclude_csv="${EASY_RATHOLE_DB_SYNC_EXCLUDE_DATABASES:-$(json_get "$state_file" "db_sync_mode.exclude_databases" "postgres,template0,template1,bucardo")}"
  local conflict_policy="${EASY_RATHOLE_DB_SYNC_CONFLICT_POLICY:-$(json_get "$state_file" "db_sync_mode.conflict_policy" "client_wins")}"
  local drop_policy="${EASY_RATHOLE_DB_SYNC_DROP_POLICY:-$(json_get "$state_file" "db_sync_mode.drop_policy" "mirror_drop")}"
  local legacy_db="${EASY_RATHOLE_SYNC_DBNAME:-}"

  update_sync_state "$state_file" "{\"database_scope\":\"user\",\"initial_clone_source\":\"client\",\"new_database_policy\":\"auto\",\"ddl_policy\":\"auto_register\",\"drop_policy\":\"${drop_policy}\",\"conflict_policy\":\"${conflict_policy}\",\"exclude_databases\":\"${exclude_csv}\",\"private_db_tunnel_addr\":\"${private_host}:${private_port}\",\"vps_db_addr\":\"${vps_bind_host}:${vps_port}\"}"

  log INFO "Menginstal Bucardo dan PostgreSQL client..."
  apt-get update -y
  DEBIAN_FRONTEND=noninteractive apt-get install -y bucardo postgresql-client

  ensure_command bucardo
  ensure_command psql
  ensure_command pg_dump
  ensure_command pg_restore
  ensure_command pg_dumpall

  log INFO "Validasi koneksi DB VPS ${vps_host}:${vps_port}/postgres"
  psql_exec "$vps_host" "$vps_port" "postgres" "$vps_user" "$vps_pass" "SELECT 1;"

  log INFO "Validasi koneksi DB Private via tunnel ${private_host}:${private_port}/postgres"
  if ! psql_exec "$private_host" "$private_port" "postgres" "$private_user" "$private_pass" "SELECT 1;"; then
    local now
    now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    update_sync_state "$state_file" "{\"waiting_for_client\": true, \"bucardo_configured\": false, \"last_waiting_for_client_at\": \"${now}\"}"
    log WARN "Private DB belum reachable via ${private_host}:${private_port}. Jalankan client tunnel lalu ulangi finalisasi."
    return 0
  fi
  update_sync_state "$state_file" "{\"waiting_for_client\": false}"

  ensure_dir "$backup_dir" 700
  local role_stamp private_roles_file vps_roles_file
  role_stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  private_roles_file="${backup_dir}/private-roles-${role_stamp}.sql"
  vps_roles_file="${backup_dir}/vps-roles-${role_stamp}.sql"
  dump_roles_best_effort "$private_host" "$private_port" "$private_user" "$private_pass" "$private_roles_file" || true
  restore_roles_best_effort "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$private_roles_file"
  dump_roles_best_effort "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$vps_roles_file" || true
  restore_roles_best_effort "$private_host" "$private_port" "$private_user" "$private_pass" "$vps_roles_file"

  ensure_bucardo_control

  local vps_dbs private_dbs registered db_names
  vps_dbs="$(list_user_databases "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$exclude_csv")"
  private_dbs="$(list_user_databases "$private_host" "$private_port" "$private_user" "$private_pass" "$exclude_csv")"
  registered="$(registered_db_names_json "$state_file")"
  if [[ -n "$legacy_db" ]]; then
    db_names="$legacy_db"
  else
    db_names="$(union_database_names "$vps_dbs" "$private_dbs" "$registered")"
  fi

  if [[ -z "$db_names" ]]; then
    log WARN "Tidak ada database user untuk disinkronkan."
    update_sync_state "$state_file" "{\"initial_clone_done\": true, \"bucardo_configured\": true, \"last_error\":\"\"}"
    return 0
  fi

  local had_error=0
  while IFS= read -r dbname; do
    [[ -n "$dbname" ]] || continue
    local slug sync_name registered_contains
    slug="$(name_slug "$dbname")"
    sync_name="ipos5_2way_${slug}"
    registered_contains="$(json_array_contains "$registered" "$dbname")"

    local exists_vps=0 exists_private=0
    database_exists "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname" && exists_vps=1 || true
    database_exists "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname" && exists_private=1 || true

    if [[ "$registered_contains" == "true" && ( "$exists_vps" == "0" || "$exists_private" == "0" ) ]]; then
      if [[ "$drop_policy" == "mirror_drop" ]]; then
        remove_bucardo_objects "$dbname"
        if [[ "$exists_vps" == "1" ]]; then
          drop_database "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname"
        fi
        if [[ "$exists_private" == "1" ]]; then
          drop_database "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname"
        fi
        update_db_record "$state_file" "$dbname" "dropped" "$sync_name" "Database dihapus di salah satu sisi; mirror_drop diterapkan."
        continue
      fi
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Database hilang di salah satu sisi."
      had_error=1
      continue
    fi

    update_db_record "$state_file" "$dbname" "pending_clone" "$sync_name" ""
    if [[ "$exists_vps" == "0" && "$exists_private" == "1" ]]; then
      create_database "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname"
      clone_database "$backup_dir" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname" || {
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Clone private ke VPS gagal."
        had_error=1
        continue
      }
    elif [[ "$exists_vps" == "1" && "$exists_private" == "0" ]]; then
      create_database "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname"
      clone_database "$backup_dir" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname" || {
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Clone VPS ke private gagal."
        had_error=1
        continue
      }
    elif [[ "$registered_contains" != "true" && "$exists_vps" == "1" && "$exists_private" == "1" ]]; then
      if [[ "$conflict_policy" == "client_wins" ]]; then
        clone_database "$backup_dir" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname" || {
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Initial conflict client_wins gagal overwrite VPS."
          had_error=1
          continue
        }
      else
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Conflict policy tidak didukung: ${conflict_policy}"
        had_error=1
        continue
      fi
    fi

    sync_schema_best_effort "$backup_dir" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname"
    sync_schema_best_effort "$backup_dir" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname"

    update_db_record "$state_file" "$dbname" "pending_register" "$sync_name" ""
    local object_count
    object_count="$(user_object_count "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" || echo "0")"
    if [[ "$object_count" == "0" ]]; then
      remove_bucardo_objects "$dbname"
      update_db_record "$state_file" "$dbname" "pending_register" "" "Database sudah ada di dua sisi, tetapi belum ada tabel/sequence user untuk didaftarkan ke Bucardo."
      continue
    fi

    if ! apply_sequence_policy "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" 1; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Sequence policy VPS gagal."
      had_error=1
      continue
    fi
    if ! apply_sequence_policy "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" 2; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Sequence policy private gagal."
      had_error=1
      continue
    fi

    if [[ "$registered_contains" == "true" ]]; then
      if ! sync_registered_objects "$dbname" "$sync_name"; then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Auto-register tabel/sequence baru atau validate sync gagal."
        had_error=1
        continue
      fi
    else
      if ! register_bucardo_sync "$dbname" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$private_host" "$private_port" "$private_user" "$private_pass"; then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Registrasi Bucardo gagal. Periksa tabel tanpa primary key atau schema mismatch."
        had_error=1
        continue
      fi
    fi

    update_db_record "$state_file" "$dbname" "synced" "$sync_name" ""
  done <<<"$db_names"

  log INFO "Restart Bucardo"
  bucardo restart || true
  finalize_aggregate_state "$state_file"

  if (( had_error != 0 )); then
    fail "Sebagian database gagal disinkronkan. Lihat db_sync_mode.databases di ${state_file}."
  fi

  log INFO "DB sync Bucardo multi-database selesai."
}

main "$@"
