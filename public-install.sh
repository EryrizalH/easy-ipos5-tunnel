#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${REPO_URL:-https://github.com/EryrizalH/easy-ipos5-tunnel}"
REPO_BASE_DIR="${REPO_BASE_DIR:-/opt/nusatunnel}"

log() { printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }
require_root() { [[ "$EUID" -eq 0 ]] || fail "Jalankan dengan sudo/root."; }

ensure_prerequisites() {
  command -v curl >/dev/null 2>&1 || fail "curl wajib tersedia."
  command -v python3 >/dev/null 2>&1 || fail "Python 3.11+ wajib tersedia sebelum instalasi."
  python3 -c 'import sys; sys.exit(sys.version_info < (3,11))' || fail "Python 3.11+ wajib tersedia; siapkan runtime sebelum menjalankan installer pada Ubuntu 22."
}

main() {
  require_root
  [[ -z "${REPO_BRANCH:-}" ]] || fail "REPO_BRANCH tidak didukung lagi. Gunakan NUSA_TUNNEL_VERSION=vX.Y.Z."
  ensure_prerequisites
  local repository="${REPO_URL%/}"
  repository="${repository%.git}"
  [[ "$repository" =~ ^https://github.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)$ ]] || fail "REPO_URL harus https://github.com/owner/repo"
  repository="${BASH_REMATCH[1]}"
  local version="${NUSA_TUNNEL_VERSION:-}" info
  if [[ -z "$version" ]]; then
    info="$(curl -fsSL --retry 3 "${NUSA_TUNNEL_RELEASE_API_URL:-https://api.github.com/repos/$repository/releases/latest}")" || fail "Tidak dapat mengambil release stabil terbaru."
    version="$(printf '%s' "$info" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert not d.get("draft") and not d.get("prerelease"); print(d["tag_name"])')" || fail "Metadata latest tidak valid."
  fi
  [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "Versi harus berformat vX.Y.Z."
  case "$(uname -m)" in x86_64|aarch64|arm64) ;; *) fail "Arsitektur tidak didukung: $(uname -m)" ;; esac
  local release_parent="$REPO_BASE_DIR/releases/$version" staging url name
  mkdir -p "$release_parent"
  staging="$(mktemp -d "$release_parent/install-XXXXXXXX")"
  url="${NUSA_TUNNEL_DOWNLOAD_BASE:-https://github.com/$repository/releases/download}/$version"
  for name in SHA256SUMS release.json nusatunnel-server.tar.gz; do
    curl -fsSL --retry 3 "$url/$name" -o "$staging/$name" || fail "Gagal mengunduh $name ($version). Staging: $staging"
  done
  python3 - "$staging" "$version" "$repository" <<'PY'
import hashlib, json, pathlib, re, shutil, sys, tarfile
root = pathlib.Path(sys.argv[1])
checksums = {}
for line in (root / 'SHA256SUMS').read_text(encoding='utf-8').splitlines():
    match = re.fullmatch(r'([0-9a-f]{64})  ([A-Za-z0-9_.-]+)', line)
    if not match or match[2] in checksums:
        raise SystemExit('Daftar checksum tidak valid.')
    checksums[match[2]] = match[1]
for name in ('release.json', 'nusatunnel-server.tar.gz'):
    with (root / name).open('rb') as stream:
        actual = hashlib.file_digest(stream, 'sha256').hexdigest()
    if actual != checksums.get(name):
        raise SystemExit('Checksum tidak cocok: ' + name)
data = json.loads((root / 'release.json').read_text(encoding='utf-8'))
if data.get('version') != sys.argv[2] or data.get('repository') != sys.argv[3]:
    raise SystemExit('Metadata berbeda dari release terpilih.')
destination = root / 'source'
destination.mkdir()
with tarfile.open(root / 'nusatunnel-server.tar.gz', 'r:gz') as archive:
    seen = set()
    members = archive.getmembers()
    for member in members:
        path = pathlib.PurePosixPath(member.name)
        if (not member.isfile() or path.is_absolute() or '..' in path.parts
            or '\\' in member.name or ':' in member.name or member.name in seen):
            raise SystemExit('Isi arsip tidak aman.')
        seen.add(member.name)
    for member in members:
        target = destination.joinpath(*pathlib.PurePosixPath(member.name).parts)
        target.parent.mkdir(parents=True, exist_ok=True)
        with archive.extractfile(member) as source, target.open('xb') as output:
            shutil.copyfileobj(source, output)
        target.chmod(member.mode & 0o755)
if json.loads((destination / 'release.json').read_text(encoding='utf-8')) != data:
    raise SystemExit('Manifest dalam paket berbeda dari release.')
PY
  local work_tree="$staging/source"
  python3 "$work_tree/scripts/release.py" preflight --root "$work_tree" || fail "Paket deployment tidak lengkap."
  export NUSA_TUNNEL_VERSION="$version"
  export NUSA_TUNNEL_ROOT="${NUSA_TUNNEL_ROOT:-/opt/nusatunnel}"
  log "Menggunakan release $version dari $repository"
  bash "$work_tree/install.sh"
}

if [[ "${BASH_SOURCE[0]:-$0}" == "$0" ]]; then main "$@"; fi
