[Unit]
Description=IPOS5TunnelPublik Dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory={{DASHBOARD_WORKDIR}}
Environment=EASY_RATHOLE_STATE_FILE={{STATE_FILE}}
Environment=EASY_RATHOLE_DB_PATH={{DB_PATH}}
Environment=EASY_RATHOLE_BUNDLES_DIR={{BUNDLES_DIR}}
Environment=EASY_RATHOLE_CACHE_DIR={{CACHE_DIR}}
Environment=EASY_RATHOLE_RESOURCES_DIR={{RESOURCES_DIR}}
ExecStart={{DASHBOARD_VENV}}/bin/uvicorn app.main:app --host 0.0.0.0 --port {{DASHBOARD_PORT}}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
