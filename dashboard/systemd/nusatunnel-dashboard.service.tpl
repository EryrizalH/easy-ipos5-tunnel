[Unit]
Description=NusaTunnel Dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory={{DASHBOARD_WORKDIR}}
Environment=NUSA_TUNNEL_STATE_FILE={{STATE_FILE}}
Environment=NUSA_TUNNEL_DB_PATH={{DB_PATH}}
Environment=NUSA_TUNNEL_BUNDLES_DIR={{BUNDLES_DIR}}
Environment=NUSA_TUNNEL_CACHE_DIR={{CACHE_DIR}}
Environment=NUSA_TUNNEL_RESOURCES_DIR={{RESOURCES_DIR}}
ExecStart={{DASHBOARD_VENV}}/bin/uvicorn app.main:app --host 0.0.0.0 --port {{DASHBOARD_PORT}}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
