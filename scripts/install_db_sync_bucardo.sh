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
  local phase="${6:-$status}"
  local detail="${7:-}"
  local unsupported_tables="${8:-[]}"
  python3 - "$state_file" "$dbname" "$status" "$sync_name" "$message" "$phase" "$detail" "$unsupported_tables" <<'PY'
import json
import pathlib
import sys
from datetime import UTC, datetime

path = pathlib.Path(sys.argv[1])
dbname, status, sync_name, message, phase, detail, unsupported_raw = sys.argv[2:9]
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
        "phase": phase or status,
        "last_checked_at": now,
    }
)
if status == "synced":
    row["last_synced_at"] = now
    row["last_error"] = ""
    row["last_error_detail"] = ""
    row["unsupported_tables"] = []
    row["message"] = ""
elif status == "dropped":
    row["dropped_at"] = now
    row["last_error"] = ""
    row["last_error_detail"] = ""
    row["unsupported_tables"] = []
    row["message"] = ""
elif status.startswith("pending"):
    row["last_error"] = message
    row["last_error_detail"] = detail
    row["unsupported_tables"] = []
    row["message"] = message
else:
    row["last_error"] = message
    row["last_error_detail"] = detail
    row["message"] = message

try:
    unsupported = json.loads(unsupported_raw)
except Exception:
    unsupported = []
if not isinstance(unsupported, list):
    unsupported = []
if unsupported:
    row["unsupported_tables"] = [str(item) for item in unsupported]
elif status not in {"synced", "dropped"} and not status.startswith("pending"):
    row.setdefault("unsupported_tables", [])

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

psql_exec_detail() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local sql="$6"
  local output
  if ! output="$(PGPASSWORD="$password" psql -v ON_ERROR_STOP=1 \
    "host=${host} port=${port} dbname=${dbname} user=${user}" \
    -c "$sql" 2>&1 >/dev/null)"; then
    printf '%s\n' "$output"
    return 1
  fi
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
    "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema','bucardo') AND n.nspname NOT LIKE 'pg_toast%' AND c.relkind IN ('r','p','S');"
}

unsupported_tables_json() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local rows
  rows="$(psql_scalar "$host" "$port" "$dbname" "$user" "$password" \
    "SELECT quote_ident(n.nspname) || '.' || quote_ident(c.relname)
       FROM pg_class c
       JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname NOT IN ('pg_catalog','information_schema','bucardo')
        AND n.nspname NOT LIKE 'pg_toast%'
        AND c.relkind IN ('r','p')
        AND NOT EXISTS (
          SELECT 1
            FROM pg_index i
           WHERE i.indrelid = c.oid
             AND i.indisvalid
             AND (i.indisprimary OR i.indisunique)
        )
      ORDER BY 1;")" || return 1
  python3 - "$rows" <<'PY'
import json
import sys

items = [line.strip() for line in sys.argv[1].splitlines() if line.strip()]
print(json.dumps(items, separators=(",", ":")))
PY
}

supported_tables_json() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local rows
  rows="$(psql_scalar "$host" "$port" "$dbname" "$user" "$password" \
    "SELECT quote_ident(n.nspname) || '.' || quote_ident(c.relname)
       FROM pg_class c
       JOIN pg_namespace n ON n.oid = c.relnamespace
      WHERE n.nspname NOT IN ('pg_catalog','information_schema','bucardo')
        AND n.nspname NOT LIKE 'pg_toast%'
        AND c.relkind IN ('r','p')
        AND EXISTS (
          SELECT 1
            FROM pg_index i
           WHERE i.indrelid = c.oid
             AND i.indisvalid
             AND (i.indisprimary OR i.indisunique)
        )
      ORDER BY 1;")" || return 1
  python3 - "$rows" <<'PY'
import json
import sys

items = [line.strip() for line in sys.argv[1].splitlines() if line.strip()]
print(json.dumps(items, separators=(",", ":")))
PY
}

repair_unsupported_tables_with_uuid() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"

  PGPASSWORD="$password" psql -v ON_ERROR_STOP=1 \
    "host=${host} port=${port} dbname=${dbname} user=${user}" <<'SQL'
CREATE OR REPLACE FUNCTION public.easy_rathole_sync_uuid()
RETURNS uuid
LANGUAGE sql
VOLATILE
AS $$
  SELECT (
    substr(md5(random()::text || clock_timestamp()::text || pg_backend_pid()::text), 1, 8) || '-' ||
    substr(md5(random()::text || clock_timestamp()::text || pg_backend_pid()::text), 1, 4) || '-' ||
    substr(md5(random()::text || clock_timestamp()::text || pg_backend_pid()::text), 1, 4) || '-' ||
    substr(md5(random()::text || clock_timestamp()::text || pg_backend_pid()::text), 1, 4) || '-' ||
    substr(md5(random()::text || clock_timestamp()::text || pg_backend_pid()::text), 1, 12)
  )::uuid;
$$;

DO $$
DECLARE
  tbl record;
  col_type text;
  idx_name text;
BEGIN
  FOR tbl IN
    SELECT n.nspname AS schema_name, c.relname AS table_name
      FROM pg_class c
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname NOT IN ('pg_catalog','information_schema','bucardo')
       AND n.nspname NOT LIKE 'pg_toast%'
       AND c.relkind IN ('r','p')
       AND NOT EXISTS (
         SELECT 1
           FROM pg_index i
          WHERE i.indrelid = c.oid
            AND i.indisvalid
            AND (i.indisprimary OR i.indisunique)
       )
     ORDER BY n.nspname, c.relname
  LOOP
    SELECT data_type
      INTO col_type
      FROM information_schema.columns
     WHERE table_schema = tbl.schema_name
       AND table_name = tbl.table_name
       AND column_name = 'easy_sync_uuid';

    IF col_type IS NULL THEN
      EXECUTE format('ALTER TABLE %I.%I ADD COLUMN easy_sync_uuid uuid', tbl.schema_name, tbl.table_name);
    ELSIF col_type <> 'uuid' THEN
      RAISE EXCEPTION 'Column %.%.easy_sync_uuid exists but is %, expected uuid', tbl.schema_name, tbl.table_name, col_type;
    END IF;

    EXECUTE format(
      'UPDATE %I.%I SET easy_sync_uuid = public.easy_rathole_sync_uuid() WHERE easy_sync_uuid IS NULL',
      tbl.schema_name,
      tbl.table_name
    );
    EXECUTE format(
      'ALTER TABLE %I.%I ALTER COLUMN easy_sync_uuid SET DEFAULT public.easy_rathole_sync_uuid()',
      tbl.schema_name,
      tbl.table_name
    );
    EXECUTE format(
      'ALTER TABLE %I.%I ALTER COLUMN easy_sync_uuid SET NOT NULL',
      tbl.schema_name,
      tbl.table_name
    );

    idx_name := 'er_uuid_' || substr(md5(tbl.schema_name || '.' || tbl.table_name), 1, 20);
    IF NOT EXISTS (
      SELECT 1
        FROM pg_class i
        JOIN pg_namespace n ON n.oid = i.relnamespace
       WHERE n.nspname = tbl.schema_name
         AND i.relname = idx_name
    ) THEN
      EXECUTE format(
        'CREATE UNIQUE INDEX %I ON %I.%I (easy_sync_uuid)',
        idx_name,
        tbl.schema_name,
        tbl.table_name
      );
    END IF;
  END LOOP;
END
$$;
SQL
}

preflight_database() {
  local label="$1"
  local host="$2"
  local port="$3"
  local dbname="$4"
  local user="$5"
  local password="$6"
  local auto_grant="${7:-1}"

  log INFO "Preflight DB sync ${label}/${dbname}: koneksi dan privilege Bucardo"
  psql_exec_detail "$host" "$port" "$dbname" "$user" "$password" "SELECT 1;" || return 1

  local output
  if ! output="$(PGPASSWORD="$password" psql -v ON_ERROR_STOP=1 \
    "host=${host} port=${port} dbname=${dbname} user=${user}" <<SQL 2>&1
CREATE SCHEMA IF NOT EXISTS bucardo;

CREATE TABLE IF NOT EXISTS bucardo.bucardo_preflight_capability (
  id integer PRIMARY KEY
);

CREATE OR REPLACE FUNCTION bucardo.bucardo_preflight_trigger_fn()
RETURNS trigger
LANGUAGE plpgsql
AS \$\$
BEGIN
  RETURN NEW;
END;
\$\$;

DROP TRIGGER IF EXISTS bucardo_preflight_trigger ON bucardo.bucardo_preflight_capability;
CREATE TRIGGER bucardo_preflight_trigger
  BEFORE INSERT OR UPDATE OR DELETE ON bucardo.bucardo_preflight_capability
  FOR EACH ROW EXECUTE PROCEDURE bucardo.bucardo_preflight_trigger_fn();

DROP TRIGGER IF EXISTS bucardo_preflight_trigger ON bucardo.bucardo_preflight_capability;
DROP FUNCTION IF EXISTS bucardo.bucardo_preflight_trigger_fn();
DROP TABLE IF EXISTS bucardo.bucardo_preflight_capability;

DO \$\$
BEGIN
  IF ${auto_grant} = 1 THEN
    EXECUTE format('GRANT USAGE, CREATE ON SCHEMA bucardo TO %I', current_user);
  END IF;
END
\$\$;
SQL
)"; then
    printf '%s\n' "$output"
    return 1
  fi
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
  if ! PGPASSWORD="$source_pass" pg_dump -Fc \
    --exclude-schema=bucardo \
    -h "$source_host" \
    -p "$source_port" \
    -U "$source_user" \
    -d "$dbname" \
    -f "$dump_file"; then
    log ERROR "pg_dump ${dbname} dari ${source_label} gagal."
    return 1
  fi

  local restore_status=0
  PGPASSWORD="$target_pass" pg_restore --clean --if-exists \
    -h "$target_host" \
    -p "$target_port" \
    -U "$target_user" \
    -d "$dbname" \
    "$dump_file" || restore_status=$?

  if (( restore_status > 1 )); then
    log ERROR "pg_restore ${dbname} gagal total dengan exit code ${restore_status}."
    return "$restore_status"
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
    --exclude-schema=bucardo \
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

ensure_bucardo_remote_truncate_tables() {
  local host="$1"
  local port="$2"
  local dbname="$3"
  local user="$4"
  local password="$5"
  local label="$6"

  log INFO "Memastikan metadata truncate Bucardo tersedia di ${label}/${dbname}"
  local output
  if ! output="$(PGPASSWORD="$password" psql -v ON_ERROR_STOP=1 \
    "host=${host} port=${port} dbname=${dbname} user=${user}" <<'SQL' 2>&1
CREATE SCHEMA IF NOT EXISTS bucardo;

CREATE TABLE IF NOT EXISTS bucardo.bucardo_truncate_trigger (
  tablename   OID         NOT NULL,
  sname       TEXT        NOT NULL,
  tname       TEXT        NOT NULL,
  sync        TEXT        NOT NULL,
  replicated  TIMESTAMPTZ     NULL,
  cdate       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS bucardo_truncate_trigger_index
  ON bucardo.bucardo_truncate_trigger (sync, tablename)
  WHERE replicated IS NULL;

CREATE TABLE IF NOT EXISTS bucardo.bucardo_truncate_trigger_log (
  tablename   OID         NOT NULL,
  sname       TEXT        NOT NULL,
  tname       TEXT        NOT NULL,
  sync        TEXT        NOT NULL,
  target      TEXT        NOT NULL,
  replicated  TIMESTAMPTZ NOT NULL,
  cdate       TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL
)"; then
    printf '%s\n' "$output"
    return 1
  fi
}

ensure_bucardo_sync_metadata() {
  local dbname="$1"
  local sync_name="$2"
  local vps_host="$3"
  local vps_port="$4"
  local vps_user="$5"
  local vps_pass="$6"
  local private_host="$7"
  local private_port="$8"
  local private_user="$9"
  local private_pass="${10}"

  ensure_bucardo_remote_truncate_tables "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" "vps" || return 1
  ensure_bucardo_remote_truncate_tables "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" "private" || return 1
  bucardo validate sync "$sync_name" || return 1
  ensure_bucardo_remote_truncate_tables "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" "vps" || return 1
  ensure_bucardo_remote_truncate_tables "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" "private" || return 1
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
    sudo -u postgres bucardo upgrade --batch --quiet --dbuser=postgres || true
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

ensure_bucardo_non_superuser_fallback() {
  local module_path="/usr/share/perl5/Bucardo.pm"
  [[ -f "$module_path" ]] || return 0

  python3 - "$module_path" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()
marker = "easy-rathole fallback to ALTER TABLE DISABLE TRIGGER USER"
if marker in text:
    raise SystemExit(0)

backup = path.with_suffix(path.suffix + ".easy-rathole-bak")
if not backup.exists():
    backup.write_text(text)

old_disable = """    ## Can we do this the easy way? Thanks to Jan for srr!
    my $dbname = $db->{name};
    if ($dbh->{pg_server_version} >= 80300) {
        $self->glog("Setting session_replication_role to replica for database $dbname", LOG_VERBOSE);
        $dbh->do(q{SET session_replication_role = 'replica'});

        $db->{triggers_enabled} = 0;
        return undef;
    }
"""
new_disable = """    ## Can we do this the easy way? Thanks to Jan for srr!
    my $dbname = $db->{name};
    if ($dbh->{pg_server_version} >= 80300) {
        $self->glog("Setting session_replication_role to replica for database $dbname", LOG_VERBOSE);
        my $srr_ok = eval { $dbh->do(q{SET session_replication_role = 'replica'}); 1 };
        if ($srr_ok) {
            $db->{triggers_enabled} = 0;
            return undef;
        }

        ## easy-rathole fallback to ALTER TABLE DISABLE TRIGGER USER for non-superuser table owners.
        my $srr_error = $@ || $dbh->errstr || 'unknown error';
        eval { $dbh->rollback() };
        $self->glog("Could not set session_replication_role on database $dbname ($srr_error); falling back to DISABLE TRIGGER USER", LOG_WARN);
        for my $goat (grep { $_->{reltype} eq 'table' } @{ $sync->{goatlist} }) {
            $dbh->do("ALTER TABLE $goat->{safeschema}.$goat->{safetable} DISABLE TRIGGER USER");
        }
        $db->{easy_rathole_user_triggers_disabled} = 1;
        $db->{triggers_enabled} = 0;
        return undef;
    }
"""
old_enable = """        ## If we are using srr, just flip it back to the default
        if ($db->{dbh}{pg_server_version} >= 80300) {
            $self->glog("Setting session_replication_role to default for database $dbname", LOG_VERBOSE);
            $dbh->do(q{SET session_replication_role = default}); ## Assumes a sane default!
            $dbh->commit();
            $db->{triggers_enabled} = time;
            next;
        }
"""
new_enable = """        ## If the easy-rathole non-superuser fallback disabled user triggers, re-enable them per table.
        if ($db->{easy_rathole_user_triggers_disabled}) {
            $self->glog("Enabling user triggers on database $dbname", LOG_VERBOSE);
            for my $goat (grep { $_->{reltype} eq 'table' } @{ $sync->{goatlist} }) {
                $dbh->do("ALTER TABLE $goat->{safeschema}.$goat->{safetable} ENABLE TRIGGER USER");
            }
            $dbh->commit();
            delete $db->{easy_rathole_user_triggers_disabled};
            $db->{triggers_enabled} = time;
            next;
        }

        ## If we are using srr, just flip it back to the default
        if ($db->{dbh}{pg_server_version} >= 80300) {
            $self->glog("Setting session_replication_role to default for database $dbname", LOG_VERBOSE);
            $dbh->do(q{SET session_replication_role = default}); ## Assumes a sane default!
            $dbh->commit();
            $db->{triggers_enabled} = time;
            next;
        }
"""
if old_disable not in text:
    raise SystemExit("Bucardo disable_triggers block tidak ditemukan")
if old_enable not in text:
    raise SystemExit("Bucardo enable_triggers block tidak ditemukan")
path.write_text(text.replace(old_disable, new_disable).replace(old_enable, new_enable))
PY

  perl -c "$module_path" >/dev/null
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

cleanup_legacy_bucardo_objects() {
  [[ "${EASY_RATHOLE_DB_SYNC_LEGACY_CLEANUP:-1}" == "1" ]] || return 0

  if ! bucardo list sync 2>/dev/null | grep -Eq '(^|[[:space:]])Sync "ipos5_2way"($|[[:space:]])|^[[:space:]]*ipos5_2way[[:space:]]'; then
    return 0
  fi

  log WARN "Membersihkan sync Bucardo legacy ipos5_2way yang tidak dipakai registry multi-database."
  bucardo stop >/dev/null 2>&1 || true
  bucardo remove sync "ipos5_2way" --force >/dev/null 2>&1 || true
  bucardo remove dbgroup "ipos5_2way_dbs" --force >/dev/null 2>&1 || true
  bucardo remove relgroup "ipos5_2way_82" --force >/dev/null 2>&1 || true
  bucardo remove relgroup "ipos5_2way_rel" --force >/dev/null 2>&1 || true
  bucardo remove db "private_remote" --force >/dev/null 2>&1 || true
  bucardo remove db "vps_local" --force >/dev/null 2>&1 || true
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
  bucardo add db "$vps_db" dbhost="$vps_host" dbport="$vps_port" dbname="$dbname" dbuser="$vps_user" dbpass="$vps_pass" || return 1
  bucardo add db "$private_db" dbhost="$private_host" dbport="$private_port" dbname="$dbname" dbuser="$private_user" dbpass="$private_pass" || return 1
  bucardo add dbgroup "$dbgroup" "${vps_db}:source" "${private_db}:source" || return 1
  local supported_tables
  supported_tables="$(supported_tables_json "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass")" || return 1
  add_supported_tables_to_relgroup "$supported_tables" "$vps_db" "$relgroup" || return 1
  bucardo add all sequences db="$vps_db" relgroup="$relgroup" || return 1
  bucardo add sync "$sync_name" relgroup="$relgroup" dbs="$dbgroup" conflict_strategy=bucardo_latest || return 1
  echo "$sync_name"
}

add_supported_tables_to_relgroup() {
  local supported_tables="$1"
  local vps_db="$2"
  local relgroup="$3"
  local count=0
  local table_name
  while IFS= read -r table_name; do
    [[ -n "$table_name" ]] || continue
    bucardo add table "$table_name" db="$vps_db" relgroup="$relgroup" || return 1
    count=$((count + 1))
  done < <(python3 - "$supported_tables" <<'PY'
import json
import sys

try:
    items = json.loads(sys.argv[1])
except Exception:
    items = []
for item in items:
    print(str(item))
PY
)
  if (( count == 0 )); then
    log ERROR "Tidak ada tabel dengan primary/unique key yang bisa didaftarkan ke Bucardo."
    return 1
  fi
}

sync_registered_objects() {
  local dbname="$1"
  local sync_name="$2"
  local vps_host="$3"
  local vps_port="$4"
  local vps_user="$5"
  local vps_pass="$6"
  local slug relgroup vps_db
  slug="$(name_slug "$dbname")"
  relgroup="ipos5_2way_rel_${slug}"
  vps_db="vps_${slug}"
  local supported_tables
  supported_tables="$(supported_tables_json "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass")" || return 1
  add_supported_tables_to_relgroup "$supported_tables" "$vps_db" "$relgroup" >/dev/null 2>&1 || true
  bucardo add all sequences db="$vps_db" relgroup="$relgroup" >/dev/null 2>&1 || true
  bucardo update sync "$sync_name" onetimecopy=2 || return 1
}

bucardo_sync_exists() {
  local sync_name="$1"
  bucardo list sync 2>/dev/null | grep -Fq "$sync_name"
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
  local unsupported_policy="${EASY_RATHOLE_DB_SYNC_UNSUPPORTED_TABLE_POLICY:-$(json_get "$state_file" "db_sync_mode.unsupported_table_policy" "uuid")}"
  case "$unsupported_policy" in
    uuid|skip) ;;
    *) unsupported_policy="uuid" ;;
  esac
  local auto_grant="${EASY_RATHOLE_DB_SYNC_AUTO_GRANT:-1}"
  [[ "$auto_grant" == "1" ]] || auto_grant="0"
  local legacy_db="${EASY_RATHOLE_SYNC_DBNAME:-}"

  update_sync_state "$state_file" "{\"database_scope\":\"user\",\"initial_clone_source\":\"client\",\"new_database_policy\":\"auto\",\"ddl_policy\":\"auto_register\",\"drop_policy\":\"${drop_policy}\",\"conflict_policy\":\"${conflict_policy}\",\"unsupported_table_policy\":\"${unsupported_policy}\",\"exclude_databases\":\"${exclude_csv}\",\"private_db_tunnel_addr\":\"${private_host}:${private_port}\",\"vps_db_addr\":\"${vps_bind_host}:${vps_port}\"}"

  if ! command -v bucardo >/dev/null 2>&1 \
    || ! command -v psql >/dev/null 2>&1 \
    || ! command -v pg_dump >/dev/null 2>&1 \
    || ! command -v pg_restore >/dev/null 2>&1 \
    || ! command -v pg_dumpall >/dev/null 2>&1; then
    log INFO "Menginstal Bucardo dan PostgreSQL client..."
    apt-get update -y
    DEBIAN_FRONTEND=noninteractive apt-get install -y bucardo postgresql-client
  else
    log INFO "Bucardo dan PostgreSQL client sudah tersedia; melewati apt-get."
  fi

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
  ensure_bucardo_non_superuser_fallback
  cleanup_legacy_bucardo_objects

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
        update_db_record "$state_file" "$dbname" "dropped" "$sync_name" "Database dihapus di salah satu sisi; mirror_drop diterapkan." "drop_database"
        continue
      fi
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Database hilang di salah satu sisi." "drop_database"
      had_error=1
      continue
    fi

    update_db_record "$state_file" "$dbname" "pending_clone" "$sync_name" "" "clone_database"
    local detail
    if [[ "$exists_vps" == "0" && "$exists_private" == "1" ]]; then
      if ! detail="$(create_database "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname" 2>&1)"; then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Create database VPS gagal." "create_database" "$detail"
        had_error=1
        continue
      fi
      if ! detail="$(clone_database "$backup_dir" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname" 2>&1)"; then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Clone private ke VPS gagal." "clone_database" "$detail"
        had_error=1
        continue
      fi
    elif [[ "$exists_vps" == "1" && "$exists_private" == "0" ]]; then
      if ! detail="$(create_database "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname" 2>&1)"; then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Create database private gagal." "create_database" "$detail"
        had_error=1
        continue
      fi
      if ! detail="$(clone_database "$backup_dir" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname" 2>&1)"; then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Clone VPS ke private gagal." "clone_database" "$detail"
        had_error=1
        continue
      fi
    elif [[ "$registered_contains" != "true" && "$exists_vps" == "1" && "$exists_private" == "1" ]]; then
      if [[ "$conflict_policy" == "client_wins" ]]; then
        if ! detail="$(clone_database "$backup_dir" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname" 2>&1)"; then
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Initial conflict client_wins gagal overwrite VPS." "clone_database" "$detail"
          had_error=1
          continue
        fi
      else
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Conflict policy tidak didukung: ${conflict_policy}" "clone_database"
        had_error=1
        continue
      fi
    fi

    sync_schema_best_effort "$backup_dir" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname"
    sync_schema_best_effort "$backup_dir" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "$dbname"

    update_db_record "$state_file" "$dbname" "pending_register" "$sync_name" "" "preflight_database"
    if ! detail="$(preflight_database "vps" "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" "$auto_grant" 2>&1)"; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Preflight Bucardo VPS gagal. Pastikan user DB punya hak CREATE schema/table/function/trigger." "preflight_database" "$detail"
      had_error=1
      continue
    fi
    if ! detail="$(preflight_database "private" "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" "$auto_grant" 2>&1)"; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Preflight Bucardo private gagal. Pastikan user DB punya hak CREATE schema/table/function/trigger." "preflight_database" "$detail"
      had_error=1
      continue
    fi

    local object_count
    if ! object_count="$(user_object_count "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" 2>&1)"; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Hitung objek user gagal." "preflight_database" "$object_count"
      had_error=1
      continue
    fi
    if [[ "$object_count" == "0" ]]; then
      remove_bucardo_objects "$dbname"
      update_db_record "$state_file" "$dbname" "error" "" "Database tidak punya tabel/sequence user yang layak didaftarkan ke Bucardo." "preflight_database"
      had_error=1
      continue
    fi

    local unsupported_vps unsupported_private unsupported_tables
    if ! unsupported_vps="$(unsupported_tables_json "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" 2>&1)"; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Cek tabel unsupported di VPS gagal." "preflight_database" "$unsupported_vps"
      had_error=1
      continue
    fi
    if ! unsupported_private="$(unsupported_tables_json "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" 2>&1)"; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Cek tabel unsupported di private gagal." "preflight_database" "$unsupported_private"
      had_error=1
      continue
    fi
    unsupported_tables="$(python3 - "$unsupported_vps" "$unsupported_private" <<'PY'
import json
import sys

out = []
for label, raw in (("vps", sys.argv[1]), ("private", sys.argv[2])):
    try:
        rows = json.loads(raw)
    except Exception:
        rows = []
    for row in rows:
        out.append(f"{label}:{row}")
print(json.dumps(sorted(set(out)), separators=(",", ":")))
PY
)"
    if [[ "$unsupported_tables" != "[]" ]]; then
      if [[ "$unsupported_policy" == "uuid" ]]; then
        if [[ "$conflict_policy" != "client_wins" ]]; then
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Policy UUID untuk tabel tanpa key membutuhkan conflict_policy=client_wins." "preflight_database" "" "$unsupported_tables"
          had_error=1
          continue
        fi

        log WARN "Beberapa tabel di ${dbname} tidak memiliki primary/unique key. Menambahkan easy_sync_uuid di private lalu clone ulang ke VPS."
        update_db_record "$state_file" "$dbname" "pending_register" "$sync_name" "Menambahkan easy_sync_uuid untuk tabel tanpa key." "repair_unsupported_tables" "" "$unsupported_tables"

        bucardo stop >/dev/null 2>&1 || true
        remove_bucardo_objects "$dbname"
        registered_contains="false"

        if ! detail="$(repair_unsupported_tables_with_uuid "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" 2>&1)"; then
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Repair UUID tabel tanpa key di private gagal." "repair_unsupported_tables" "$detail" "$unsupported_tables"
          had_error=1
          continue
        fi
        if ! detail="$(clone_database "$backup_dir" "private" "$private_host" "$private_port" "$private_user" "$private_pass" "vps" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$dbname" 2>&1)"; then
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Clone ulang private ke VPS setelah repair UUID gagal." "repair_unsupported_tables" "$detail" "$unsupported_tables"
          had_error=1
          continue
        fi

        if ! unsupported_vps="$(unsupported_tables_json "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" 2>&1)"; then
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Cek ulang tabel unsupported di VPS gagal." "repair_unsupported_tables" "$unsupported_vps"
          had_error=1
          continue
        fi
        if ! unsupported_private="$(unsupported_tables_json "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" 2>&1)"; then
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Cek ulang tabel unsupported di private gagal." "repair_unsupported_tables" "$unsupported_private"
          had_error=1
          continue
        fi
        unsupported_tables="$(python3 - "$unsupported_vps" "$unsupported_private" <<'PY'
import json
import sys

out = []
for label, raw in (("vps", sys.argv[1]), ("private", sys.argv[2])):
    try:
        rows = json.loads(raw)
    except Exception:
        rows = []
    for row in rows:
        out.append(f"{label}:{row}")
print(json.dumps(sorted(set(out)), separators=(",", ":")))
PY
)"
        if [[ "$unsupported_tables" != "[]" ]]; then
          update_db_record "$state_file" "$dbname" "error" "$sync_name" "Masih ada tabel tanpa primary/unique key setelah repair UUID." "repair_unsupported_tables" "" "$unsupported_tables"
          had_error=1
          continue
        fi
        update_db_record "$state_file" "$dbname" "pending_register" "$sync_name" "Tabel tanpa key sudah diberi easy_sync_uuid." "repair_unsupported_tables" "" "$unsupported_tables"
      else
        log WARN "Beberapa tabel di ${dbname} tidak memiliki primary/unique key dan dilewati oleh Bucardo."
        update_db_record "$state_file" "$dbname" "pending_register" "$sync_name" "Beberapa tabel dilewati (tidak ada primary/unique key)." "preflight_database" "" "$unsupported_tables"
      fi
    fi

    if ! apply_sequence_policy "$vps_host" "$vps_port" "$dbname" "$vps_user" "$vps_pass" 1; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Sequence policy VPS gagal." "sequence_policy" "" "$unsupported_tables"
      had_error=1
      continue
    fi
    if ! apply_sequence_policy "$private_host" "$private_port" "$dbname" "$private_user" "$private_pass" 2; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Sequence policy private gagal." "sequence_policy" "" "$unsupported_tables"
      had_error=1
      continue
    fi

    update_db_record "$state_file" "$dbname" "pending_register" "$sync_name" "" "register_bucardo" "" "$unsupported_tables"
    local reg_exit_code=0
    local reg_detail_file
    reg_detail_file="$(mktemp)"
    if [[ "$registered_contains" == "true" ]] && bucardo_sync_exists "$sync_name"; then
      (
        set -euo pipefail
        sync_registered_objects "$dbname" "$sync_name" "$vps_host" "$vps_port" "$vps_user" "$vps_pass"
      ) >"$reg_detail_file" 2>&1 || reg_exit_code=$?
      
      local detail
      detail="$(cat "$reg_detail_file")"
      rm -f "$reg_detail_file"
      
      if (( reg_exit_code != 0 )); then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Auto-register tabel/sequence baru gagal." "register_bucardo" "$detail" "$unsupported_tables"
        had_error=1
        continue
      fi
    else
      (
        set -euo pipefail
        register_bucardo_sync "$dbname" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$private_host" "$private_port" "$private_user" "$private_pass"
      ) >"$reg_detail_file" 2>&1 || reg_exit_code=$?
      
      local detail
      detail="$(cat "$reg_detail_file")"
      rm -f "$reg_detail_file"
      
      if (( reg_exit_code != 0 )); then
        update_db_record "$state_file" "$dbname" "error" "$sync_name" "Registrasi Bucardo gagal. Periksa schema mismatch atau kredensial DB." "register_bucardo" "$detail" "$unsupported_tables"
        had_error=1
        continue
      fi
    fi

    update_db_record "$state_file" "$dbname" "pending_register" "$sync_name" "" "repair_metadata" "" "$unsupported_tables"
    if ! detail="$(ensure_bucardo_sync_metadata "$dbname" "$sync_name" "$vps_host" "$vps_port" "$vps_user" "$vps_pass" "$private_host" "$private_port" "$private_user" "$private_pass" 2>&1)"; then
      update_db_record "$state_file" "$dbname" "error" "$sync_name" "Repair metadata Bucardo remote atau validate sync gagal." "repair_metadata" "$detail" "$unsupported_tables"
      had_error=1
      continue
    fi

    update_db_record "$state_file" "$dbname" "synced" "$sync_name" "" "synced" "" "$unsupported_tables"
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
