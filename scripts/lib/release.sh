#!/usr/bin/env bash

require_release_python() {
  command -v python3 >/dev/null 2>&1 || fail "Python 3.11+ wajib tersedia sebelum instalasi."
  python3 -c 'import sys; sys.exit(sys.version_info < (3,11))' || fail "Python 3.11+ wajib tersedia; Python bawaan Ubuntu 22 perlu disiapkan terlebih dahulu."
}

load_release_metadata() {
  local project_root="$1" info
  [[ -f "$project_root/release.json" ]] || fail "Metadata release tidak tersedia. Gunakan public-install.sh dari GitHub Release."
  info="$(python3 - "$project_root" <<'PY'
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / 'scripts'))
from release import metadata
data = metadata(Path(sys.argv[1]))
print(data['version'])
print(data['repository'])
print(data['rathole_version'])
PY
  )" || fail "Gagal membaca metadata release"
  local values
  mapfile -t values <<< "$info"
  [[ -z "${NUSA_TUNNEL_VERSION:-}" || "${NUSA_TUNNEL_VERSION}" == "${values[0]}" ]] || fail "Versi paket berbeda dari NUSA_TUNNEL_VERSION"
  export NUSA_TUNNEL_VERSION="${values[0]}"
  export NUSA_TUNNEL_RELEASE_REPOSITORY="${values[1]}"
  export NUSA_TUNNEL_RATHOLE_VERSION="${values[2]}"
}
