#!/usr/bin/env bash
set -euo pipefail

EASY_RATHOLE_RELEASE_API="https://api.github.com/repos/rathole-org/rathole/releases/latest"

log() {
  local level="$1"
  shift
  printf '[%s] [%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$level" "$*"
}

fail() {
  log ERROR "$*"
  exit 1
}

require_root() {
  [[ "${EUID}" -eq 0 ]] || fail "Script ini wajib dijalankan sebagai root (sudo)."
}

ensure_ubuntu_22_plus() {
  [[ -f /etc/os-release ]] || fail "File /etc/os-release tidak ditemukan"
  # shellcheck disable=SC1091
  source /etc/os-release
  [[ "${ID:-}" == "ubuntu" ]] || fail "Hanya Ubuntu yang didukung. Terdeteksi: ${ID:-unknown}"

  local major="${VERSION_ID%%.*}"
  [[ "$major" =~ ^[0-9]+$ ]] || fail "Gagal membaca Ubuntu VERSION_ID=${VERSION_ID:-unknown}"
  (( major >= 22 )) || fail "Wajib Ubuntu 22 ke atas. Versi saat ini: ${VERSION_ID}"
}

ensure_dir() {
  local dir="$1"
  local mode="${2:-755}"
  install -d -m "$mode" "$dir"
}

ensure_command() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || fail "Perintah wajib tidak ditemukan: $cmd"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64) echo "x86_64" ;;
    aarch64|arm64) echo "aarch64" ;;
    *) fail "Arsitektur tidak didukung: $(uname -m)" ;;
  esac
}

asset_name_for_linux_arch() {
  local arch="$1"
  case "$arch" in
    x86_64) echo "rathole-x86_64-unknown-linux-gnu.zip" ;;
    aarch64) echo "rathole-aarch64-unknown-linux-musl.zip" ;;
    *) fail "Tidak ada mapping release asset untuk arsitektur: $arch" ;;
  esac
}

random_string() {
  local length="${1:-32}"
  tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$length"
  echo
}

detect_public_ip() {
  local ip=""
  ip="$(curl -4fsS --max-time 5 https://api.ipify.org || true)"
  if [[ -z "$ip" ]]; then
    ip="$(curl -4fsS --max-time 5 https://ifconfig.me/ip || true)"
  fi
  if [[ -z "$ip" ]]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  [[ -n "$ip" ]] || fail "Gagal mendeteksi public IP. Isi manual di state file."
  echo "$ip"
}

fetch_latest_rathole_release_json() {
  curl -fsSL "$EASY_RATHOLE_RELEASE_API"
}

get_release_tag() {
  python3 -c 'import json,sys; data=json.load(sys.stdin); print(data.get("tag_name", ""))'
}

get_release_asset_url() {
  local asset_name="$1"
  python3 -c 'import json,sys
target=sys.argv[1]
data=json.load(sys.stdin)
for a in data.get("assets", []):
    if a.get("name") == target:
        print(a.get("browser_download_url", ""))
        break' "$asset_name"
}

port_in_use() {
  local port="$1"
  ss -ltnH "( sport = :${port} )" | grep -q .
}

pick_random_free_port() {
  local start="${1:-20000}"
  local end="${2:-45000}"
  local attempts=0

  while (( attempts < 200 )); do
    local candidate
    candidate="$(( RANDOM % (end - start + 1) + start ))"
    if ! port_in_use "$candidate"; then
      echo "$candidate"
      return 0
    fi
    attempts=$((attempts + 1))
  done

  fail "Gagal menemukan port kosong pada rentang ${start}-${end}"
}

render_template() {
  local template_file="$1"
  local output_file="$2"
  shift 2

  python3 - "$template_file" "$output_file" "$@" <<'PY'
import pathlib
import sys

template_path = pathlib.Path(sys.argv[1])
output_path = pathlib.Path(sys.argv[2])
pairs = sys.argv[3:]

if len(pairs) % 2 != 0:
  raise SystemExit("render_template membutuhkan pasangan key/value")

text = template_path.read_text(encoding="utf-8")
for i in range(0, len(pairs), 2):
    key = pairs[i]
    value = pairs[i + 1]
    text = text.replace("{{" + key + "}}", value)

output_path.parent.mkdir(parents=True, exist_ok=True)
output_path.write_text(text, encoding="utf-8")
PY
}

state_get() {
  local state_file="$1"
  local key="$2"
  local default_value="${3:-}"

  python3 - "$state_file" "$key" "$default_value" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
key = sys.argv[2]
default = sys.argv[3]

if not path.exists():
    print(default)
    raise SystemExit(0)

raw = path.read_text(encoding="utf-8").strip()
if not raw:
  print(default)
  raise SystemExit(0)

try:
  data = json.loads(raw)
except json.JSONDecodeError:
  print(default)
  raise SystemExit(0)

value = data.get(key, default)
if isinstance(value, (dict, list)):
    print(json.dumps(value))
else:
    print(value)
PY
}

state_merge_json() {
  local state_file="$1"
  local patch_json="$2"

  python3 - "$state_file" "$patch_json" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
patch = json.loads(sys.argv[2])

if path.exists():
  raw = path.read_text(encoding="utf-8").strip()
  if raw:
    try:
      data = json.loads(raw)
    except json.JSONDecodeError:
      data = {}
  else:
    data = {}
else:
  data = {}

data.update(patch)
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
PY
}
