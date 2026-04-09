[client]
remote_addr = "{{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}"

[client.services.{{DB_SERVICE_KEY}}]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
local_addr = "{{DB_CLIENT_LOCAL_ADDR}}"

[client.services.{{POS_HTTP_SERVICE_KEY}}]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
local_addr = "{{POS_HTTP_CLIENT_LOCAL_ADDR}}"

[client.services.{{POS_WORKER_SERVICE_KEY}}]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
local_addr = "{{POS_WORKER_CLIENT_LOCAL_ADDR}}"
