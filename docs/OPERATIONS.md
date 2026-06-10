# Panduan Operasional IPOS5TunnelPublik

## Cek status service

```bash
systemctl is-active rathole
systemctl is-active easy-rathole-dashboard
systemctl is-active fail2ban
```

## Verifikasi hardening baseline

```bash
sudo ufw status verbose
sudo fail2ban-client status sshd
sysctl net.ipv4.tcp_syncookies
```

Lihat metadata hardening pada state file:

```bash
python3 - <<'PY'
import json
with open('/opt/easy-rathole/state/install-state.json') as f:
    d = json.load(f)
print('hardening_applied=', d.get('hardening_applied'))
print('hardening_ssh_port=', d.get('hardening_ssh_port'))
print('hardening_disable_ssh_password=', d.get('hardening_disable_ssh_password'))
print('hardening_ssh_allow_cidr=', d.get('hardening_ssh_allow_cidr'))
PY
```

## Restart service

```bash
sudo systemctl restart rathole
sudo systemctl restart easy-rathole-dashboard
```

## Lokasi file

- State: `/opt/easy-rathole/state/install-state.json`
- DB dashboard: `/opt/easy-rathole/state/easy-rathole.db`
- Rathole server config: `/etc/easy-rathole/server.toml`
- Bundle output: `/opt/easy-rathole/bundles`

## Rotasi token

1. Login ke dashboard
2. Set token baru
3. Unduh ulang installer client
4. Deploy ulang client dengan bundle terbaru

> Catatan: client dengan token lama akan gagal autentikasi setelah rotasi.

## Verifikasi port listening

```bash
sudo ss -ltnp | grep -E ':5444|:5480|:5485|:8088'
```

Catatan alur database:
- Tanpa DB sync, DB masih bisa berjalan sebagai port forward lama `VPS 5444 -> client 127.0.0.1:5444`.
- Dengan DB sync, PostgreSQL VPS publish di `0.0.0.0:5444` dan dapat dicek lokal via `127.0.0.1:5444`, sementara Bucardo mengakses private DB via tunnel internal VPS `127.0.0.1:5445`.
- Dengan DB sync tanpa PgBouncer, rathole client DB tetap mengarah ke PostgreSQL Windows `127.0.0.1:5444`.
- Dengan DB sync + PgBouncer, aplikasi Windows tetap ke PgBouncer `0.0.0.0:5444`, PostgreSQL backend pindah ke `127.0.0.1:5445`, dan rathole client DB mengarah ke backend `127.0.0.1:5445`.
- Tunnel DB sync harus bind ke `127.0.0.1:5445` di VPS, bukan `0.0.0.0`.
- Installer Windows hanya membuat firewall inbound TCP `5444` saat PgBouncer dipasang.

Untuk control port rathole, lihat dari state file:

```bash
python3 - <<'PY'
import json
with open('/opt/easy-rathole/state/install-state.json') as f:
    print(json.load(f).get('rathole_control_port'))
PY
```

## Log service

```bash
journalctl -u rathole -f
journalctl -u easy-rathole-dashboard -f
```

## Setup DB sync Bucardo

Installer dapat memasang PostgreSQL 9.5 di VPS via Docker dan menyiapkan dashboard sebelum finalisasi Bucardo:

```bash
sudo EASY_RATHOLE_INSTALL_DB_SYNC=1 bash install.sh
```

Default container:
- image: `postgres:9.5`
- container: `postgres95`
- bind: `0.0.0.0:5444`
- volume data: `/opt/easy-rathole/postgres95/data`

Jika registry tidak menyediakan `postgres:9.5`, gunakan image PostgreSQL 9.5 yang sudah dipin:

```bash
sudo EASY_RATHOLE_INSTALL_VPS_DB=1 \
  EASY_RATHOLE_VPS_DB_IMAGE=registry.example.com/postgres:9.5 \
  EASY_RATHOLE_VPS_DB_BIND_HOST=0.0.0.0 \
  bash install.sh
```

Isi state sebelum menjalankan setup:

```json
{
  "db_sync_mode": {
    "enabled": true,
    "vps_db_addr": "0.0.0.0:5444",
    "private_db_tunnel_addr": "127.0.0.1:5445",
    "private_db_backend_mode": "direct",
    "dbname": "postgres",
    "vps_db_user": "sysi5adm",
    "private_db_user": "sysi5adm",
    "initial_clone_done": false,
    "bucardo_configured": false,
    "waiting_for_client": false
  }
}
```

Gunakan `private_db_backend_mode=pgbouncer_backend` bila private PostgreSQL sudah dipindah ke `127.0.0.1:5445`.

Finalisasi Bucardo:

1. Login dashboard.
2. Download dan install bundle client.
3. Pastikan service client aktif dan tunnel private DB reachable.
4. Tekan tombol **Finalisasi DB Sync** di dashboard.

Installer otomatis memasang prerequisite berikut sebelum dashboard aktif:
- Docker + PostgreSQL 9.5 VPS container, kecuali `EASY_RATHOLE_INSTALL_VPS_DB=0`
- Rathole server/client config
- Dashboard dan bundle generator

Saat tombol finalisasi ditekan, dashboard menjalankan setup Bucardo dan PostgreSQL client di VPS. Jika client belum terkoneksi ke rathole, setup Bucardo tidak dianggap gagal total. Dashboard menyimpan `db_sync_mode.waiting_for_client=true`; tekan tombol finalisasi lagi setelah client online agar clone awal dan Bucardo dipasang.

Clone awal melalui dashboard:
- Source clone adalah DB private/client via `127.0.0.1:5445`.
- Dump disimpan di `/opt/easy-rathole/backups`.
- DB VPS non-empty tidak dioverwrite kecuali `EASY_RATHOLE_FORCE_INITIAL_CLONE=1`.
- Role/auth di-dump best-effort. Password hash role hanya bisa 1:1 bila user dump punya permission cukup.

Terapkan sequence ganjil/genap hanya setelah backup, restore snapshot awal, dan aplikasi di kedua sisi sedang berhenti:

```bash
sudo EASY_RATHOLE_APPLY_SEQUENCE_POLICY=1 /opt/easy-rathole/src/easy-ipos5-tunnel/scripts/install_db_sync_bucardo.sh
```

Verifikasi koneksi Bucardo:

```bash
psql "host=127.0.0.1 port=5444 dbname=postgres user=DB_USER" -c "select 1"
psql "host=127.0.0.1 port=5445 dbname=postgres user=DB_USER" -c "select 1"
bucardo status
bucardo list sync
```

Jika `bucardo status` menunjukkan sync `Bad` dan log berisi:

```text
ERROR: relation "bucardo.bucardo_truncate_trigger" does not exist
```

jalankan finalisasi DB sync ulang dari dashboard, atau dari VPS:

```bash
sudo /opt/easy-rathole/src/easy-ipos5-tunnel/scripts/install_db_sync_bucardo.sh
sudo bucardo restart
bucardo status
```

Script finalisasi akan memastikan tabel metadata truncate Bucardo tersedia di DB VPS dan DB private, lalu menjalankan `bucardo validate sync` ulang. Jika tetap gagal, pastikan user DB sync punya hak `CREATE` pada database aplikasi di kedua sisi.

## Jalankan ulang installer

Installer bersifat idempotent dasar, sehingga aman dijalankan ulang:

```bash
sudo bash install.sh
```

Namun setelah re-run:

- cek ulang status service
- cek state file
- verifikasi dashboard masih bisa login

## Catatan aman sebelum disable SSH password

Jika ingin menjalankan dengan `EASY_RATHOLE_DISABLE_SSH_PASSWORD=1`, pastikan:

1. Login SSH pakai private key sudah teruji
2. Minimal ada satu file `authorized_keys` valid
3. Jangan menutup sesi SSH aktif sampai verifikasi login sesi baru berhasil
