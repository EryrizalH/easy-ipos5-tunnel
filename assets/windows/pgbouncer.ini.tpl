; PGbouncer sample config for IPOS5TunnelPublik (PostgreSQL 9.5 compatible)
; Keep PostgreSQL local listener on 127.0.0.1:5444
; PGbouncer listens on 127.0.0.1:6432

[databases]
* = host=127.0.0.1 port=5444 dbname=postgres

[pgbouncer]
listen_addr = 127.0.0.1
listen_port = 6432
auth_type = md5
auth_file = userlist.txt
pool_mode = transaction
max_client_conn = 300
default_pool_size = 30
reserve_pool_size = 10
reserve_pool_timeout = 3
server_reset_query = DISCARD ALL
server_check_delay = 30
server_idle_timeout = 600
ignore_startup_parameters = extra_float_digits
admin_users = postgres
stats_users = postgres
log_connections = 1
log_disconnections = 1
