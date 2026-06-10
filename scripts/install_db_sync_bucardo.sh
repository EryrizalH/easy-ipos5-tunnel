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

update_sync_state() {
  local state_file="$1"
  local patch_json="$2"
  python3 - "$state_file" "$patch_json" <<'PY'
import json
import pathlib
import sys

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
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
PY
  chmod 0600 "$state_file"
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

user_object_count() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  psql_scalar "$host" "$port" "$dbname" "$user" "$password" \
    "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND c.relkind IN ('r','v','m','S','f');"
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
  log WARN "Dump roles dari private DB gagal; clone data tetap dilanjutkan. Detail: ${output_file}.err"
  return 1
}

restore_roles_best_effort() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local input_file="$6"

  [[ -s "$input_file" ]] || return 0
  if PGPASSWORD="$password" psql "host=${host} port=${port} dbname=${dbname} user=${user}" -f "$input_file" >/dev/null 2>"${input_file}.restore.err"; then
    return 0
  fi
  log WARN "Restore roles ke VPS tidak sepenuhnya berhasil; lanjut restore data. Detail: ${input_file}.restore.err"
  return 0
}

run_initial_clone() {
  local state_file="$1"
  local backup_dir="$2"
  local private_host="$3"
  local private_port="$4"
  local private_db="$5"
  local private_user="$6"
  local private_pass="$7"
  local vps_host="$8"
  local vps_port="$9"
  local vps_db="${10}"
  local vps_user="${11}"
  local vps_pass="${12}"

  ensure_dir "$backup_dir" 700

  local now
  local stamp
  local dump_file
  local roles_file
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  dump_file="${backup_dir}/${private_db}-initial-clone-${stamp}.dump"
  roles_file="${backup_dir}/${private_db}-roles-${stamp}.sql"

  log INFO "Mencoba dump role/auth dari private DB ${private_host}:${private_port}"
  dump_roles_best_effort "$private_host" "$private_port" "$private_user" "$private_pass" "$roles_file" || true
  restore_roles_best_effort "$vps_host" "$vps_port" "$vps_db" "$vps_user" "$vps_pass" "$roles_file"

  log INFO "Clone awal DB private ${private_host}:${private_port}/${private_db} -> VPS ${vps_host}:${vps_port}/${vps_db}"
  PGPASSWORD="$private_pass" pg_dump -Fc \
    -h "$private_host" \
    -p "$private_port" \
    -U "$private_user" \
    -d "$private_db" \
    -f "$dump_file"

  local restore_status=0
  PGPASSWORD="$vps_pass" pg_restore --clean --if-exists \
    -h "$vps_host" \
    -p "$vps_port" \
    -U "$vps_user" \
    -d "$vps_db" \
    "$dump_file" || restore_status=$?

  if (( restore_status > 1 )); then
    fail "pg_restore gagal total dengan exit code ${restore_status}."
  elif (( restore_status == 1 )); then
    log WARN "pg_restore selesai dengan beberapa peringatan/error non-fatal (seperti parameter konfigurasi tidak dikenali)."
  fi

  update_sync_state "$state_file" "{\"initial_clone_done\": true, \"waiting_for_client\": false, \"initial_clone_blocked_nonempty\": false, \"last_clone_at\": \"${now}\", \"last_clone_dump\": \"${dump_file}\"}"
  log INFO "Clone awal selesai: ${dump_file}"
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
    WHERE sequence_schema NOT IN ('pg_catalog', 'information_schema')
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

  local app_db="${EASY_RATHOLE_SYNC_DBNAME:-$(json_get "$state_file" "db_sync_mode.dbname" "postgres")}"
  local vps_user="${EASY_RATHOLE_SYNC_VPS_USER:-$(json_get "$state_file" "db_sync_mode.vps_db_user" "sysi5adm")}"
  local vps_pass="${EASY_RATHOLE_SYNC_VPS_PASSWORD:-$(json_get "$state_file" "db_sync_mode.vps_db_password" "u&aV23cc.o82dtr1x89c")}"
  local private_user="${EASY_RATHOLE_SYNC_PRIVATE_USER:-$(json_get "$state_file" "db_sync_mode.private_db_user" "$vps_user")}"
  local private_pass="${EASY_RATHOLE_SYNC_PRIVATE_PASSWORD:-$(json_get "$state_file" "db_sync_mode.private_db_password" "$vps_pass")}"
  local sync_name="${EASY_RATHOLE_BUCARDO_SYNC_NAME:-ipos5_2way}"
  local dbgroup_name="${EASY_RATHOLE_BUCARDO_DBGROUP:-ipos5_2way_dbs}"

  log INFO "Menginstal Bucardo dan PostgreSQL client..."
  apt-get update -y
  DEBIAN_FRONTEND=noninteractive apt-get install -y bucardo postgresql-client

  ensure_command bucardo
  ensure_command psql
  ensure_command pg_dump
  ensure_command pg_restore
  ensure_command pg_dumpall

  log INFO "Validasi koneksi DB VPS ${vps_host}:${vps_port}/${app_db}"
  psql_exec "$vps_host" "$vps_port" "$app_db" "$vps_user" "$vps_pass" "SELECT 1;"

  log INFO "Validasi koneksi DB Private via tunnel ${private_host}:${private_port}/${app_db}"
  if ! psql_exec "$private_host" "$private_port" "$app_db" "$private_user" "$private_pass" "SELECT 1;"; then
    local now
    now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    update_sync_state "$state_file" "{\"waiting_for_client\": true, \"bucardo_configured\": false, \"last_waiting_for_client_at\": \"${now}\", \"private_db_tunnel_addr\": \"${private_host}:${private_port}\"}"
    log WARN "Private DB belum reachable via ${private_host}:${private_port}. Jalankan client tunnel lalu ulangi installer untuk clone awal dan Bucardo."
    return 0
  fi
  update_sync_state "$state_file" "{\"waiting_for_client\": false, \"private_db_tunnel_addr\": \"${private_host}:${private_port}\", \"vps_db_addr\": \"${vps_bind_host}:${vps_port}\"}"

  local initial_clone_done
  initial_clone_done="$(json_get "$state_file" "db_sync_mode.initial_clone_done" "false")"
  if [[ "${EASY_RATHOLE_SKIP_INITIAL_CLONE:-0}" == "1" ]]; then
    log WARN "Clone awal dilewati karena EASY_RATHOLE_SKIP_INITIAL_CLONE=1."
    update_sync_state "$state_file" "{\"initial_clone_skipped\": true}"
  elif [[ "$initial_clone_done" != "true" || "${EASY_RATHOLE_FORCE_INITIAL_CLONE:-0}" == "1" ]]; then
    local vps_object_count
    vps_object_count="$(user_object_count "$vps_host" "$vps_port" "$app_db" "$vps_user" "$vps_pass")"
    if [[ "$vps_object_count" != "0" && "${EASY_RATHOLE_FORCE_INITIAL_CLONE:-0}" != "1" ]]; then
      update_sync_state "$state_file" "{\"initial_clone_blocked_nonempty\": true, \"bucardo_configured\": false}"
      fail "DB VPS ${vps_host}:${vps_port}/${app_db} tidak kosong (${vps_object_count} object). Set EASY_RATHOLE_FORCE_INITIAL_CLONE=1 untuk overwrite via clone awal."
    fi
    run_initial_clone "$state_file" "$backup_dir" \
      "$private_host" "$private_port" "$app_db" "$private_user" "$private_pass" \
      "$vps_host" "$vps_port" "$app_db" "$vps_user" "$vps_pass"
  else
    log INFO "Clone awal sudah pernah selesai; melewati pg_dump/pg_restore."
  fi

  if [[ "${EASY_RATHOLE_APPLY_SEQUENCE_POLICY:-0}" == "1" ]]; then
    log INFO "Menerapkan sequence ganjil di VPS dan genap di Private"
    apply_sequence_policy "$vps_host" "$vps_port" "$app_db" "$vps_user" "$vps_pass" 1
    apply_sequence_policy "$private_host" "$private_port" "$app_db" "$private_user" "$private_pass" 2
  else
    log WARN "Sequence policy dilewati. Set EASY_RATHOLE_APPLY_SEQUENCE_POLICY=1 hanya saat maintenance window."
  fi

  systemctl start postgresql || true

  # Temporary remove any existing bucardorc to ensure installation uses Unix domain socket peer authentication
  rm -f /etc/bucardorc /root/.bucardorc || true
  local postgres_home
  postgres_home="$(getent passwd postgres | cut -d: -f6)"
  if [[ -n "$postgres_home" ]]; then
    rm -f "${postgres_home}/.bucardorc" || true
  fi

  if ! sudo -u postgres bucardo status >/dev/null 2>&1; then
    log INFO "Inisialisasi database kontrol Bucardo..."
    sudo -u postgres bucardo install --batch --quiet --dbuser=postgres
  fi

  log INFO "Mengatur password user bucardo di host PostgreSQL..."
  sudo -u postgres psql -c "ALTER USER bucardo WITH PASSWORD 'bucardo';" || true

  log INFO "Konfigurasi kredensial bucardorc..."
  cat <<EOF >/etc/bucardorc
dbhost=127.0.0.1
dbport=5432
dbuser=bucardo
dbpass=bucardo
EOF
  cp /etc/bucardorc /root/.bucardorc || true
  local postgres_home
  postgres_home="$(getent passwd postgres | cut -d: -f6)"
  if [[ -n "$postgres_home" && -d "$postgres_home" ]]; then
    cp /etc/bucardorc "${postgres_home}/.bucardorc" || true
    chown postgres:postgres "${postgres_home}/.bucardorc" || true
    chmod 0600 "${postgres_home}/.bucardorc" || true
  fi
  chmod 0600 /etc/bucardorc /root/.bucardorc || true

  log INFO "Mendaftarkan database Bucardo"
  bucardo remove sync "$sync_name" --force >/dev/null 2>&1 || true
  bucardo remove dbgroup "$dbgroup_name" --force >/dev/null 2>&1 || true
  bucardo remove db vps_local --force >/dev/null 2>&1 || true
  bucardo remove db private_remote --force >/dev/null 2>&1 || true

  bucardo add db vps_local dbhost="$vps_host" dbport="$vps_port" dbname="$app_db" dbuser="$vps_user" dbpass="$vps_pass"
  bucardo add db private_remote dbhost="$private_host" dbport="$private_port" dbname="$app_db" dbuser="$private_user" dbpass="$private_pass"
  bucardo add dbgroup "$dbgroup_name" vps_local:source private_remote:source
  bucardo add sync "$sync_name" tables=all dbs="$dbgroup_name" conflict_strategy=bucardo_latest
  bucardo validate sync "$sync_name"

  log INFO "Restart Bucardo"
  bucardo restart
  local configured_at
  configured_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  update_sync_state "$state_file" "{\"bucardo_configured\": true, \"waiting_for_client\": false, \"bucardo_sync_name\": \"${sync_name}\", \"bucardo_configured_at\": \"${configured_at}\"}"
  log INFO "DB sync Bucardo selesai dikonfigurasi untuk sync ${sync_name}."
}

main "$@"
