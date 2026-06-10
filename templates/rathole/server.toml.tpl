[server]
bind_addr = "0.0.0.0:{{RATHOLE_CONTROL_PORT}}"

[server.services.{{DB_SERVICE_KEY}}]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
bind_addr = "{{DB_REMOTE_BIND_ADDR}}:{{DB_REMOTE_BIND_PORT}}"

[server.services.{{POS_HTTP_SERVICE_KEY}}]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
bind_addr = "{{POS_HTTP_REMOTE_BIND_ADDR}}:{{POS_HTTP_REMOTE_BIND_PORT}}"

[server.services.{{POS_WORKER_SERVICE_KEY}}]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
bind_addr = "{{POS_WORKER_REMOTE_BIND_ADDR}}:{{POS_WORKER_REMOTE_BIND_PORT}}"
