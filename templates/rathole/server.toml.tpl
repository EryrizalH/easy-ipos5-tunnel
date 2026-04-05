[server]
bind_addr = "0.0.0.0:{{RATHOLE_CONTROL_PORT}}"

[server.services.port_5444]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
bind_addr = "0.0.0.0:5444"

[server.services.port_5480]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
bind_addr = "0.0.0.0:5480"

[server.services.port_5485]
type = "tcp"
token = "{{GLOBAL_TOKEN}}"
bind_addr = "0.0.0.0:5485"
