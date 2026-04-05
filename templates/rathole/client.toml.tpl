[client]
remote_addr = "{{SERVER_ADDR}}:{{RATHOLE_CONTROL_PORT}}"

[client.services.port_5444]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
local_addr = "127.0.0.1:5444"

[client.services.port_5480]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
local_addr = "127.0.0.1:5480"

[client.services.port_5485]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
local_addr = "127.0.0.1:5485"
