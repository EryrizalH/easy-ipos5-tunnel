#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

main() {
  require_root
  ensure_ubuntu_22_plus
  ensure_command python3
  ensure_command systemctl

  local easy_root="${NUSA_TUNNEL_ROOT:-/opt/nusatunnel}"
  local state_file="${NUSA_TUNNEL_STATE_FILE:-${easy_root}/state/install-state.json}"
  local deploy_dir="${easy_root}/dashboard"
  local resources_dir="${easy_root}/resources"
  local db_path="${easy_root}/state/nusatunnel.db"
  local bundles_dir="${easy_root}/bundles"
  local cache_dir="${easy_root}/cache"
  local dashboard_port
  local venv_dir
  local admin_username
  local admin_password
  local token
  local credentials_file
  local service_file
  local now
  local public_ip

  dashboard_port="$(state_get "$state_file" "dashboard_port" "8088")"

  ensure_dir "$deploy_dir" 755
  ensure_dir "$easy_root/state" 750
  ensure_dir "$bundles_dir" 750
  ensure_dir "$cache_dir" 750
  ensure_dir "$resources_dir/assets/windows" 755
  ensure_dir "$resources_dir/assets/windows7" 755
  ensure_dir "$resources_dir/assets/linux" 755
  ensure_dir "$resources_dir/templates/rathole" 755

  [[ -f "${PROJECT_ROOT}/assets/windows/pgbouncer.exe" ]] || fail "Asset wajib belum tersedia: ${PROJECT_ROOT}/assets/windows/pgbouncer.exe"
  [[ -f "${PROJECT_ROOT}/assets/windows/libevent-7.dll" ]] || fail "Asset wajib belum tersedia: ${PROJECT_ROOT}/assets/windows/libevent-7.dll"
  [[ -f "${PROJECT_ROOT}/assets/windows/libssl-3-x64.dll" ]] || fail "Asset wajib belum tersedia: ${PROJECT_ROOT}/assets/windows/libssl-3-x64.dll"
  [[ -f "${PROJECT_ROOT}/assets/windows/libcrypto-3-x64.dll" ]] || fail "Asset wajib belum tersedia: ${PROJECT_ROOT}/assets/windows/libcrypto-3-x64.dll"
  [[ -f "${PROJECT_ROOT}/assets/windows/libwinpthread-1.dll" ]] || fail "Asset wajib belum tersedia: ${PROJECT_ROOT}/assets/windows/libwinpthread-1.dll"
  [[ -f "${PROJECT_ROOT}/assets/windows/nusatunnel-service.exe" ]] || fail "Asset wajib belum tersedia: ${PROJECT_ROOT}/assets/windows/nusatunnel-service.exe"
  [[ -f "${PROJECT_ROOT}/assets/windows/nusatunnel.exe" ]] || fail "Asset wajib belum tersedia: ${PROJECT_ROOT}/assets/windows/nusatunnel.exe"

  rm -rf "${deploy_dir}/app"
  cp -R "${PROJECT_ROOT}/dashboard/app" "${deploy_dir}/app"
  cp "${PROJECT_ROOT}/dashboard/requirements.txt" "${deploy_dir}/requirements.txt"

  install -m 0644 "${PROJECT_ROOT}/assets/windows/install-service.ps1.tpl" "${resources_dir}/assets/windows/install-service.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/uninstall-service.ps1.tpl" "${resources_dir}/assets/windows/uninstall-service.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/install-service.cmd.tpl" "${resources_dir}/assets/windows/install-service.cmd.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/uninstall-service.cmd.tpl" "${resources_dir}/assets/windows/uninstall-service.cmd.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/setup-client.cmd.tpl" "${resources_dir}/assets/windows/setup-client.cmd.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/setup.exe" "${resources_dir}/assets/windows/setup.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/nusatunnel-service.exe" "${resources_dir}/assets/windows/nusatunnel-service.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/nusatunnel.exe" "${resources_dir}/assets/windows/nusatunnel.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/nusatunnel-gui.exe" "${resources_dir}/assets/windows/nusatunnel-gui.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/nssm.exe" "${resources_dir}/assets/windows/nssm.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/pgbouncer.exe" "${resources_dir}/assets/windows/pgbouncer.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/libevent-7.dll" "${resources_dir}/assets/windows/libevent-7.dll"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/libssl-3-x64.dll" "${resources_dir}/assets/windows/libssl-3-x64.dll"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/libcrypto-3-x64.dll" "${resources_dir}/assets/windows/libcrypto-3-x64.dll"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/libwinpthread-1.dll" "${resources_dir}/assets/windows/libwinpthread-1.dll"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/install-gui-autostart.ps1.tpl" "${resources_dir}/assets/windows/install-gui-autostart.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/uninstall-gui-autostart.ps1.tpl" "${resources_dir}/assets/windows/uninstall-gui-autostart.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/pgbouncer.ini.tpl" "${resources_dir}/assets/windows/pgbouncer.ini.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/userlist.sample.txt" "${resources_dir}/assets/windows/userlist.sample.txt"
  if [[ -d "${PROJECT_ROOT}/assets/windows7" ]]; then
    cp -R "${PROJECT_ROOT}/assets/windows7/." "${resources_dir}/assets/windows7/"
  fi
  install -m 0644 "${PROJECT_ROOT}/assets/linux/install-client.sh.tpl" "${resources_dir}/assets/linux/install-client.sh.tpl"
  install -m 0644 "${PROJECT_ROOT}/templates/rathole/client.toml.tpl" "${resources_dir}/templates/rathole/client.toml.tpl"

  venv_dir="${deploy_dir}/.venv"
  if [[ ! -x "${venv_dir}/bin/python" ]]; then
    python3 -m venv "$venv_dir"
  fi
  "${venv_dir}/bin/pip" install --upgrade pip >/dev/null
  "${venv_dir}/bin/pip" install -r "${deploy_dir}/requirements.txt" >/dev/null

  admin_username="$(state_get "$state_file" "admin_username" "admin")"
  admin_password="$(state_get "$state_file" "admin_password" "")"
  token="$(state_get "$state_file" "token" "")"
  public_ip="$(state_get "$state_file" "public_ip" "127.0.0.1")"

  [[ -n "$admin_password" ]] || admin_password="$(random_string 24)"
  [[ -n "$token" ]] || token="$(random_string 40)"

  NUSA_TUNNEL_DB_PATH="$db_path" \
  NUSA_TUNNEL_STATE_FILE="$state_file" \
  NUSA_TUNNEL_INITIAL_TOKEN="$token" \
  NUSA_TUNNEL_ADMIN_USERNAME="$admin_username" \
  NUSA_TUNNEL_ADMIN_PASSWORD="$admin_password" \
  PYTHONPATH="$deploy_dir" \
  "${venv_dir}/bin/python" -m app.bootstrap

  credentials_file="${easy_root}/state/dashboard-credentials.txt"
  {
    printf 'Dashboard URL      : http://%s:%s\n' "$public_ip" "$dashboard_port"
    printf 'Username           : %s\n' "$admin_username"
    printf 'Password           : %s\n' "$admin_password"
    printf 'Generated at (UTC) : %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  } > "$credentials_file"
  chmod 0600 "$credentials_file"

  service_file="/etc/systemd/system/nusatunnel-dashboard.service"
  render_template "${PROJECT_ROOT}/dashboard/systemd/nusatunnel-dashboard.service.tpl" "$service_file" \
    DASHBOARD_WORKDIR "$deploy_dir" \
    DASHBOARD_VENV "$venv_dir" \
    DASHBOARD_PORT "$dashboard_port" \
    STATE_FILE "$state_file" \
    DB_PATH "$db_path" \
    BUNDLES_DIR "$bundles_dir" \
    CACHE_DIR "$cache_dir" \
    RESOURCES_DIR "$resources_dir"

  systemctl daemon-reload
  systemctl enable nusatunnel-dashboard >/dev/null
  systemctl restart nusatunnel-dashboard
  systemctl is-active --quiet nusatunnel-dashboard || fail "Gagal menjalankan service: nusatunnel-dashboard"

  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  state_merge_json "$state_file" "{\
    \"admin_username\": \"${admin_username}\", \
    \"admin_password\": \"${admin_password}\", \
    \"credentials_file\": \"${credentials_file}\", \
    \"dashboard_service_name\": \"nusatunnel-dashboard\", \
    \"dashboard_port\": ${dashboard_port}, \
    \"db_path\": \"${db_path}\", \
    \"bundles_dir\": \"${bundles_dir}\", \
    \"resources_dir\": \"${resources_dir}\", \
    \"updated_at\": \"${now}\"\
  }"

  chmod 0600 "$state_file"
  log INFO "Instalasi dashboard Nusa IPOS 5 Tunnel selesai."
}

main "$@"
