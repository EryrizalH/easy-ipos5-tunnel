# IPOS5TunnelPublik

Installer otomatis berbasis Bash untuk **Ubuntu 22+** agar IPOS 5 bisa diakses via internet menggunakan reverse tunnel `rathole`.

## Ringkasan fitur

- Install server `rathole` + systemd service (`rathole`)
- Port forward TCP server default (VPS): **5444**, **5480**, **5485**
- Mode DB sync opsional: semua database user PostgreSQL di client dan VPS di-clone lalu disinkronkan 2-arah. PostgreSQL VPS tersinkron publish di **0.0.0.0:5444**, sedangkan tunnel replikasi private memakai **127.0.0.1:5445** di VPS
- Installer dapat memasang PostgreSQL **9.5** di VPS via Docker agar versi database VPS sama dengan client.
- PgBouncer client tetap opsional:
  - tanpa PgBouncer: PostgreSQL Windows tetap `127.0.0.1:5444`
  - dengan PgBouncer: aplikasi tetap `127.0.0.1:5444`, backend PostgreSQL pindah ke `127.0.0.1:5445`
- Control port rathole dipilih otomatis (random, port kosong)
- Dashboard FastAPI (HTTP + Basic Auth) untuk:
  - melihat status service dan status port forward
  - set/rotasi global token
  - download bundle client Windows/Linux
- Hardening baseline server saat install (UFW, fail2ban, unattended-upgrades, sysctl, baseline sshd)
- Generator bundle client:
  - **Windows**: ZIP berisi service installer (NSSM) + GUI tray auto-start
  - **Linux**: ZIP berisi `client.toml` + installer systemd

---

## Quick Start

1. Install server.
2. Login dashboard.
3. Set/rotasi global token.
4. Download bundle client (Windows/Linux).
5. Install client di mesin tujuan.

---

## 1) Install Server (Ubuntu 22+)

### Opsi A — langsung dari repo publik (paling cepat)

```bash
curl -fsSL https://raw.githubusercontent.com/pruedence21/easy-ipos5-tunnel/main/public-install.sh | sudo bash
```

### Opsi B — clone repo lalu jalankan

```bash
git clone https://github.com/pruedence21/easy-ipos5-tunnel.git
cd easy-ipos5-tunnel
sudo bash install.sh
```

### Opsi C — jika source sudah ada di folder lokal

```bash
sudo bash install.sh
```

### Apa yang dilakukan installer

Urutan default:

1. Hardening baseline server.
2. Install dependency runtime.
3. Jika DB sync aktif, siapkan state DB sync awal.
4. Jika DB sync aktif, install PostgreSQL 9.5 Docker di VPS pada `0.0.0.0:5444`.
5. Install rathole server + service `rathole`.
6. Install dashboard + service `easy-rathole-dashboard`.
7. Buka firewall untuk control port + port publik `5444/5480/5485` + port dashboard. Saat DB sync aktif, rathole DB tunnel `5445` hanya bind ke `127.0.0.1` dan tidak dibuka publik.
8. Jika DB sync aktif, finalisasi clone semua database user dan setup Bucardo dilakukan dari tombol **Finalisasi DB Sync** di dashboard setelah client tunnel reachable. Setelah itu dashboard juga menjalankan auto-discovery periodik untuk database/tabel/sequence baru.

Jika `EASY_RATHOLE_INSTALL_DB_SYNC=1`, installer memasang PostgreSQL 9.5 VPS via Docker dan dashboard langsung bisa dipakai untuk download bundle client. Setelah service client aktif, buka dashboard lalu jalankan **Finalisasi DB Sync**. Jika client belum terkoneksi, state ditandai `waiting_for_client=true`; ulangi tombol finalisasi setelah client online. Database system `postgres`, `template0`, `template1`, dan `bucardo` dikecualikan secara default.

### Opsi environment (opsional)

```bash
# skip hardening (tidak direkomendasikan)
sudo EASY_RATHOLE_HARDENING=0 bash install.sh

# tetap hardening, tapi skip apt upgrade
sudo EASY_RATHOLE_RUN_UPGRADE=0 bash install.sh

# nonaktifkan SSH password auth (WAJIB key-based login siap)
sudo EASY_RATHOLE_DISABLE_SSH_PASSWORD=1 bash install.sh

# batasi akses SSH hanya dari CIDR tertentu
sudo EASY_RATHOLE_SSH_ALLOW_CIDR="1.2.3.4/32" bash install.sh

# batasi akses dashboard hanya dari CIDR tertentu
sudo EASY_RATHOLE_DASHBOARD_ALLOW_CIDR="1.2.3.4/32" bash install.sh

# ganti port dashboard (default 8088)
sudo DASHBOARD_PORT=9090 bash install.sh

# aktifkan DB sync + install PostgreSQL 9.5 VPS via Docker
sudo EASY_RATHOLE_INSTALL_DB_SYNC=1 bash install.sh

# bila hanya ingin install DB VPS tanpa Bucardo
sudo EASY_RATHOLE_INSTALL_VPS_DB=1 bash install.sh

# opsi DB sync multi-database
sudo EASY_RATHOLE_INSTALL_DB_SYNC=1 \
  EASY_RATHOLE_DB_SYNC_DISCOVERY_INTERVAL_SEC=60 \
  EASY_RATHOLE_DB_SYNC_EXCLUDE_DATABASES="postgres,template0,template1,bucardo" \
  EASY_RATHOLE_DB_SYNC_CONFLICT_POLICY=client_wins \
  EASY_RATHOLE_DB_SYNC_DROP_POLICY=mirror_drop \
  bash install.sh
```

> ⚠️ `EASY_RATHOLE_DISABLE_SSH_PASSWORD=1` hanya aman jika login SSH key-based sudah teruji. Script akan menolak jika `authorized_keys` tidak ditemukan.

### Opsi untuk `public-install.sh` (advanced)

```bash
# contoh install dari branch selain main
curl -fsSL https://raw.githubusercontent.com/pruedence21/easy-ipos5-tunnel/main/public-install.sh \
| sudo REPO_BRANCH=feature-branch bash
```

Variabel yang didukung: `REPO_URL`, `REPO_BRANCH`, `REPO_BASE_DIR`.

### Output akhir installer

Setelah install selesai, installer menampilkan:

- URL dashboard
- username dashboard
- lokasi file password dashboard
- control port rathole
- daftar port forward aktif

---

## 2) Lokasi file penting

- State file: `/opt/easy-rathole/state/install-state.json`
  - kunci penting baru: `service_ports` (contoh item DB: `remote_bind_port=5444`, `client_local_port=5444`)
  - mode sync DB opsional:
    ```json
    {
      "db_sync_mode": {
        "enabled": true,
        "vps_db_addr": "0.0.0.0:5444",
        "private_db_tunnel_addr": "127.0.0.1:5445",
        "private_db_backend_mode": "direct",
        "database_scope": "user",
        "initial_clone_source": "client",
        "new_database_policy": "auto",
        "ddl_policy": "auto_register",
        "drop_policy": "mirror_drop",
        "conflict_policy": "client_wins",
        "exclude_databases": "postgres,template0,template1,bucardo",
        "databases": [
          {
            "name": "contoh_db",
            "status": "synced",
            "sync_name": "ipos5_2way_contoh_db_...",
            "last_synced_at": "2026-06-10T00:00:00Z"
          }
        ],
        "initial_clone_done": false,
        "bucardo_configured": false,
        "waiting_for_client": false
      }
    }
    ```
    Gunakan `private_db_backend_mode=pgbouncer_backend` bila PostgreSQL private sudah dipindah ke backend `127.0.0.1:5445` oleh PgBouncer.
- Metadata PostgreSQL VPS Docker disimpan pada `vps_postgres`.
- Config rathole server: `/etc/easy-rathole/server.toml`
- Credential dashboard: `/opt/easy-rathole/state/dashboard-credentials.txt`
- DB dashboard (sqlite): `/opt/easy-rathole/state/easy-rathole.db`
- Output bundle client: `/opt/easy-rathole/bundles`

---

## 3) Dashboard

Default berjalan di port `8088` (atau sesuai `DASHBOARD_PORT`).

Fitur utama:

1. Login Basic Auth.
2. Set/rotasi global token (dengan restart service `rathole`).
3. Monitoring status:
   - `rathole`
   - `easy-rathole-dashboard`
   - status port remote bind tunnel publik
   - performa PostgreSQL VPS lokal via `127.0.0.1:5444` saat DB sync aktif, atau port DB forward default saat sync tidak aktif:
     - `connect_ms`, `query_ms`, `tx_ms`
     - active/waiting connections
     - cache hit ratio
4. Download bundle client:
   - `GET /download/windows`
   - `GET /download/windows7`
   - `GET /download/linux`

Health check endpoint:

- `GET /health` → `{"status":"ok"}`

Endpoint monitor PostgreSQL untuk GUI:

- `GET /api/monitor/postgres/latest`

Environment variable monitor PostgreSQL (opsional):

- `EASY_RATHOLE_PG_MONITOR_ENABLED` (default `1`)
- `EASY_RATHOLE_PG_MONITOR_INTERVAL_SEC` (default `5`)
- `EASY_RATHOLE_PG_MONITOR_DSN` (disarankan, contoh: `host=127.0.0.1 port=5444 dbname=postgres user=monitor password=*** connect_timeout=3`)
- Jika `DSN` tidak diisi, fallback:
  - `EASY_RATHOLE_PG_MONITOR_HOST` (default `127.0.0.1`)
  - `EASY_RATHOLE_PG_MONITOR_PORT` (default `db_sync_mode.vps_db_addr` saat sync aktif; jika tidak, mengikuti `service_ports.db.remote_bind_port`)
  - `EASY_RATHOLE_PG_MONITOR_USER` (default `sysi5adm`)
  - `EASY_RATHOLE_PG_MONITOR_PASSWORD` (default `u&aV23cc.o82dtr1x89c`)
  - `EASY_RATHOLE_PG_MONITOR_DBNAME` (default `postgres`)
  - `EASY_RATHOLE_PG_MONITOR_TIMEOUT_SEC` (default `3`)

---

## 4) Client Windows

### Isi bundle Windows (modern)

- `setup.exe`
- `ipos5-rathole-service.exe`
- `ipos5-rathole.exe`
- `ipos5-rathole-gui.exe`
- `nssm.exe`
- `pgbouncer.exe`
- `libevent-7.dll`
- `libssl-3-x64.dll`
- `libcrypto-3-x64.dll`
- `libwinpthread-1.dll`
- `client.toml`
- `pgbouncer.ini`
- `userlist.sample.txt`
- `README.txt`

### Cara install (disarankan)

1. Download bundle dari dashboard.
2. Extract ZIP.
3. Jalankan `setup.exe` sebagai Administrator.

`setup.exe` akan menyediakan menu untuk:

- install/uninstall service Windows `EasyRatholeClient`
- menjalankan/stop aplikasi GUI client
- aksi lock/unlock pembuatan database baru

Catatan paket terbaru:
- Entry point resmi installer Windows adalah `setup.exe` (menu interaktif).
- `EasyRatholeClient` dijalankan lewat wrapper headless `ipos5-rathole-service.exe`, lalu wrapper itu mengeksekusi `ipos5-rathole.exe client.toml`.
- Script template lama seperti `setup-client.cmd`/`install-service.cmd` bukan alur utama bundle dashboard saat ini.
- Saat install sukses, shortcut desktop `ipos5-rathole` dibuat untuk membuka GUI jendela utama dengan UAC (Run as Administrator).
- PgBouncer tetap opsional. Menu `Install IP Public` tidak memindahkan PostgreSQL ke `5445`; menu `Install PgBouncer` memindahkan backend PostgreSQL ke `127.0.0.1:5445`.
- Saat bundle dibuat untuk DB sync + PgBouncer backend, `client.toml` mempertahankan DB `local_addr=127.0.0.1:5445` agar Bucardo di VPS mencapai backend PostgreSQL, bukan PgBouncer.
- Runtime file `pgbouncer.ini` dan `userlist.txt` dibuat otomatis saat install service.
- Untuk menyiapkan asset Windows:
  - `scripts/build_windows_unified.ps1` membangun `setup.exe`
  - `scripts/build_windows_service_wrapper.ps1` membangun `ipos5-rathole-service.exe`
  - `scripts/build_windows_gui.ps1` membangun `ipos5-rathole-gui.exe`
  - `assets/windows/ipos5-rathole.exe` tetap dipakai sebagai binary tunnel Windows custom milik repo ini
- Build asset PGbouncer Windows dari source repository resmi:
  - repo: `https://github.com/pgbouncer/pgbouncer`
  - helper script: `scripts/build_pgbouncer_windows.ps1`

### Isi bundle Windows 7 (terpisah)

- `setup.exe`
- `ipos5-rathole-service.exe`
- `ipos5-rathole.exe`
- `nssm.exe`
- `pgbouncer.exe`
- `libevent-7.dll`
- `libssl-3-x64.dll`
- `libcrypto-3-x64.dll`
- `libwinpthread-1.dll`
- `client.toml`
- `pgbouncer.ini`
- `userlist.sample.txt`
- `README.txt`

Catatan Windows 7:
- Download melalui route `GET /download/windows7`.
- Asset bundle dibaca dari folder terpisah `assets/windows7` lalu disalin ke `resources/assets/windows7`.
- Varian Windows 7 sengaja **tanpa** `ipos5-rathole-gui.exe`.
- Installer Win7 fokus ke service tunnel/kompatibilitas Win7; GUI desktop tidak termasuk di paket ini.

### Uninstall

- Jalankan `setup.exe` sebagai Administrator lalu pilih menu uninstall/cleanup service.

---

## 5) Client Linux

### Isi bundle Linux

- `client.toml`
- `install-client.sh`

### Cara install

```bash
sudo ./install-client.sh
```

Perilaku installer Linux:

- install binary `rathole` terbaru dari release resmi GitHub (sesuai arsitektur x86_64/aarch64)
- membuat service `easy-rathole-client`
- enable + start service saat install selesai

---

## 6) Nama service/task

- Server rathole: `rathole`
- Dashboard: `easy-rathole-dashboard`
- Linux client: `easy-rathole-client`
- Windows client service: `EasyRatholeClient`
- Windows GUI scheduled task: `EasyRatholeClientGUI`

---

## 7) Catatan keamanan

- Dashboard saat ini **HTTP-only** (tanpa TLS).
- Sangat disarankan batasi akses dashboard via firewall (CIDR whitelist).
- Simpan file credential dashboard dengan aman.
- Saat rotasi token, client lama (token lama) akan gagal autentikasi sampai di-deploy ulang.

---

## 8) Troubleshooting cepat

```bash
systemctl status rathole
systemctl status easy-rathole-dashboard
journalctl -u rathole -n 100 --no-pager
journalctl -u easy-rathole-dashboard -n 100 --no-pager
sudo ss -ltnp | grep -E ':5444|:5480|:5485|:8088'
```

Untuk menyiapkan DB sync, jalankan installer dengan mode DB sync. Installer memasang PostgreSQL VPS, rathole, dan dashboard; finalisasi Bucardo dilakukan dari dashboard setelah client tunnel aktif:

```bash
sudo EASY_RATHOLE_INSTALL_DB_SYNC=1 bash install.sh
```

Variabel PostgreSQL VPS Docker:

```bash
sudo EASY_RATHOLE_INSTALL_VPS_DB=1 \
  EASY_RATHOLE_VPS_DB_IMAGE=postgres:9.5 \
  EASY_RATHOLE_VPS_DB_BIND_HOST=0.0.0.0 \
  EASY_RATHOLE_VPS_DB_NAME=postgres \
  EASY_RATHOLE_VPS_DB_USER=sysi5adm \
  EASY_RATHOLE_VPS_DB_PASSWORD='u&aV23cc.o82dtr1x89c' \
  bash install.sh
```

Catatan: image default adalah `postgres:9.5`. Karena tag lama bisa tidak tersedia lagi di registry publik, pin `EASY_RATHOLE_VPS_DB_IMAGE` ke image PostgreSQL 9.5 yang tersedia/teruji bila pull default gagal.

Clone awal melalui dashboard:

- Source clone adalah DB private/client via `127.0.0.1:5445`.
- DB VPS tidak akan dioverwrite bila sudah berisi object user, kecuali `EASY_RATHOLE_FORCE_INITIAL_CLONE=1`.
- Dump sementara disimpan di `/opt/easy-rathole/backups`.
- Role/auth di-dump best-effort sebelum restore. Password hash role bisa 1:1 hanya bila user dump punya permission cukup.

Sequence ganjil/genap sengaja tidak diterapkan otomatis. Jalankan hanya setelah backup dan snapshot awal kedua database sama:

```bash
sudo EASY_RATHOLE_APPLY_SEQUENCE_POLICY=1 /opt/easy-rathole/src/easy-ipos5-tunnel/scripts/install_db_sync_bucardo.sh
```

Lihat panduan operasional lanjutan di: `docs/OPERATIONS.md`.
