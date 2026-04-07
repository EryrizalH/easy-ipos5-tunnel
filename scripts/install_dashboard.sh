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

  local easy_root="${EASY_RATHOLE_ROOT:-/opt/easy-rathole}"
  local state_file="${EASY_RATHOLE_STATE_FILE:-${easy_root}/state/install-state.json}"
  local deploy_dir="${easy_root}/dashboard"
  local resources_dir="${easy_root}/resources"
  local db_path="${easy_root}/state/easy-rathole.db"
  local bundles_dir="${easy_root}/bundles"
  local cache_dir="${easy_root}/cache"
  local dashboard_port
  dashboard_port="$(state_get "$state_file" "dashboard_port" "8088")"

  ensure_dir "$deploy_dir" 755
  ensure_dir "$easy_root/state" 750
  ensure_dir "$bundles_dir" 750
  ensure_dir "$cache_dir" 750
  ensure_dir "$resources_dir/assets/windows" 755
  ensure_dir "$resources_dir/assets/linux" 755
  ensure_dir "$resources_dir/templates/rathole" 755

  rm -rf "${deploy_dir}/app"
  cp -R "${PROJECT_ROOT}/dashboard/app" "${deploy_dir}/app"
  cp "${PROJECT_ROOT}/dashboard/requirements.txt" "${deploy_dir}/requirements.txt"

  install -m 0644 "${PROJECT_ROOT}/assets/windows/install-service.ps1.tpl" "${resources_dir}/assets/windows/install-service.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/uninstall-service.ps1.tpl" "${resources_dir}/assets/windows/uninstall-service.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/install-service.cmd.tpl" "${resources_dir}/assets/windows/install-service.cmd.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/uninstall-service.cmd.tpl" "${resources_dir}/assets/windows/uninstall-service.cmd.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/setup-client.cmd.tpl" "${resources_dir}/assets/windows/setup-client.cmd.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/setup.exe" "${resources_dir}/assets/windows/setup.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/ipos5-rathole.exe" "${resources_dir}/assets/windows/ipos5-rathole.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/ipos5-rathole-gui.exe" "${resources_dir}/assets/windows/ipos5-rathole-gui.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/nssm.exe" "${resources_dir}/assets/windows/nssm.exe"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/install-gui-autostart.ps1.tpl" "${resources_dir}/assets/windows/install-gui-autostart.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/windows/uninstall-gui-autostart.ps1.tpl" "${resources_dir}/assets/windows/uninstall-gui-autostart.ps1.tpl"
  install -m 0644 "${PROJECT_ROOT}/assets/linux/install-client.sh.tpl" "${resources_dir}/assets/linux/install-client.sh.tpl"
  install -m 0644 "${PROJECT_ROOT}/templates/rathole/client.toml.tpl" "${resources_dir}/templates/rathole/client.toml.tpl"

  local venv_dir="${deploy_dir}/.venv"
  if [[ ! -x "${venv_dir}/bin/python" ]]; then
    python3 -m venv "$venv_dir"
  fi
  "${venv_dir}/bin/pip" install --upgrade pip >/dev/null
  "${venv_dir}/bin/pip" install -r "${deploy_dir}/requirements.txt" >/dev/null

  local admin_username
  local admin_password
  local token
  admin_username="$(state_get "$state_file" "admin_username" "admin")"
  admin_password="$(state_get "$state_file" "admin_password" "")"
  token="$(state_get "$state_file" "token" "")"

  [[ -n "$admin_password" ]] || admin_password="$(random_string 24)"
  [[ -n "$token" ]] || token="$(random_string 40)"

  EASY_RATHOLE_DB_PATH="$db_path" \
  EASY_RATHOLE_STATE_FILE="$state_file" \
  EASY_RATHOLE_INITIAL_TOKEN="$token" \
  EASY_RATHOLE_ADMIN_USERNAME="$admin_username" \
  EASY_RATHOLE_ADMIN_PASSWORD="$admin_password" \
  PYTHONPATH="$deploy_dir" \
  "${venv_dir}/bin/python" -m app.bootstrap

  local credentials_file="${easy_root}/state/dashboard-credentials.txt"
  cat > "$credentials_file" <<EOF
Dashboard URL      : http://$(state_get "$state_file" "public_ip" "127.0.0.1"):${dashboard_port}
Username           : ${admin_username}
Password           : ${admin_password}
Generated at (UTC) : $(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
  chmod 0600 "$credentials_file"

  local service_file="/etc/systemd/system/easy-rathole-dashboard.service"
  render_template "${PROJECT_ROOT}/dashboard/systemd/easy-rathole-dashboard.service.tpl" "$service_file" \
    DASHBOARD_WORKDIR "$deploy_dir" \
    DASHBOARD_VENV "$venv_dir" \
    DASHBOARD_PORT "$dashboard_port" \
    STATE_FILE "$state_file" \
    DB_PATH "$db_path" \
    BUNDLES_DIR "$bundles_dir" \
    CACHE_DIR "$cache_dir" \
    RESOURCES_DIR "$resources_dir"

  systemctl daemon-reload
  systemctl enable easy-rathole-dashboard >/dev/null
  systemctl restart easy-rathole-dashboard
  systemctl is-active --quiet easy-rathole-dashboard || fail "Gagal menjalankan service: easy-rathole-dashboard"

  local now
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  state_merge_json "$state_file" "{\
    \"admin_username\": \"${admin_username}\", \
    \"admin_password\": \"${admin_password}\", \
    \"credentials_file\": \"${credentials_file}\", \
    \"dashboard_service_name\": \"easy-rathole-dashboard\", \
    \"dashboard_port\": ${dashboard_port}, \
    \"db_path\": \"${db_path}\", \
    \"bundles_dir\": \"${bundles_dir}\", \
    \"resources_dir\": \"${resources_dir}\", \
    \"updated_at\": \"${now}\"\
  }"

  chmod 0600 "$state_file"
  log INFO "Instalasi dashboard IPOS5TunnelPublik selesai."
}

main "$@"
