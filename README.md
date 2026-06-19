# Nusa IPOS 5 Tunnel (NusaTunnel)

Installer otomatis berbasis Bash untuk **Ubuntu 22+** agar IPOS 5 bisa diakses via internet menggunakan reverse sambungan (`rathole`).

## Ringkasan fitur

- Install server sambungan + layanan (`rathole`)
- Jalur forward TCP server (VPS): **5444**, **5480**, **5485**
- Mapping database default: **VPS 5444 -> Client 127.0.0.1:5444 (NusaTunnelClient) -> Pengoptimal Database (NusaTunnelDB) 0.0.0.0:5444 -> Database 127.0.0.1:5445**
- Jalur kontrol sambungan dipilih otomatis (random, port kosong)
- Dashboard NusaTunnel (HTTP + Basic Auth) untuk:
  - melihat status layanan dan status jalur forward
  - set/rotasi global token
  - download bundle client Windows/Linux
- Hardening baseline server saat install (UFW, fail2ban, unattended-upgrades, sysctl, baseline sshd)
- Generator bundle client:
  - **Windows**: ZIP berisi service installer + GUI tray auto-start
  - **Linux**: ZIP berisi File Pengaturan (`client.toml`) + installer layanan

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
3. Install rathole server + layanan `rathole`.
4. Install dashboard + layanan `nusatunnel-dashboard`.
5. Buka firewall untuk jalur kontrol + remote bind ports sambungan (default 5444/5480/5485) + jalur dashboard.

### Opsi environment (opsional)

```bash
# skip hardening (tidak direkomendasikan)
sudo NUSA_TUNNEL_HARDENING=0 bash install.sh

# tetap hardening, tapi skip apt upgrade
sudo NUSA_TUNNEL_RUN_UPGRADE=0 bash install.sh

# nonaktifkan SSH password auth (WAJIB key-based login siap)
sudo NUSA_TUNNEL_DISABLE_SSH_PASSWORD=1 bash install.sh

# batasi akses SSH hanya dari CIDR tertentu
sudo NUSA_TUNNEL_SSH_ALLOW_CIDR="1.2.3.4/32" bash install.sh

# batasi akses dashboard hanya dari CIDR tertentu
sudo NUSA_TUNNEL_DASHBOARD_ALLOW_CIDR="1.2.3.4/32" bash install.sh

# ganti port dashboard (default 8088)
sudo NUSA_TUNNEL_PORT=9090 bash install.sh
```

> ⚠️ `NUSA_TUNNEL_DISABLE_SSH_PASSWORD=1` hanya aman jika login SSH key-based sudah teruji. Script akan menolak jika `authorized_keys` tidak ditemukan.

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

- State file: `/opt/nusatunnel/state/install-state.json`
  - kunci penting baru: `service_ports` (contoh item DB: `remote_bind_port=5444`, `client_local_port=5444`)
- Config rathole server: `/etc/nusatunnel/server.toml`
- Credential dashboard: `/opt/nusatunnel/state/dashboard-credentials.txt`
- DB dashboard (sqlite): `/opt/nusatunnel/state/nusatunnel.db`
- Output bundle client: `/opt/nusatunnel/bundles`

---

## 3) Dashboard

Default berjalan di port `8088` (atau sesuai `NUSA_TUNNEL_PORT`).

Fitur utama:

1. Login Basic Auth.
2. Set/rotasi global token (dengan restart layanan `rathole`).
3. Monitoring status:
   - `rathole`
   - `nusatunnel-dashboard`
   - status jalur remote bind sambungan (default 5444/5480/5485)
   - performa Database PostgreSQL client via sambungan `127.0.0.1:5444` (NusaTunnelDB listen `0.0.0.0:5444`, backend PostgreSQL `127.0.0.1:5445`):
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

- `NUSA_TUNNEL_PG_MONITOR_ENABLED` (default `1`)
- `NUSA_TUNNEL_PG_MONITOR_INTERVAL_SEC` (default `5`)
- `NUSA_TUNNEL_PG_MONITOR_DSN` (disarankan, contoh: `host=127.0.0.1 port=5444 dbname=postgres user=monitor password=*** connect_timeout=3`)
- Jika `DSN` tidak diisi, fallback:
  - `NUSA_TUNNEL_PG_MONITOR_HOST` (default `127.0.0.1`)
  - `NUSA_TUNNEL_PG_MONITOR_PORT` (default mengikuti `service_ports.db.remote_bind_port`, biasanya `5444`)
  - `NUSA_TUNNEL_PG_MONITOR_USER` (default `sysi5adm`)
  - `NUSA_TUNNEL_PG_MONITOR_PASSWORD` (default `u&aV23cc.o82dtr1x89c`)
  - `NUSA_TUNNEL_PG_MONITOR_DBNAME` (default `postgres`)
  - `NUSA_TUNNEL_PG_MONITOR_TIMEOUT_SEC` (default `3`)

---

## 4) Client Windows

### Isi bundle Windows (modern)

- `setup.exe`
- `nusatunnel-service.exe`
- `nusatunnel.exe`
- `nusatunnel-gui.exe`
- `client.toml` (File Pengaturan)
- file pendukung database

### Cara install (disarankan)

1. Download bundle dari dashboard.
2. Extract ZIP.
3. Jalankan `setup.exe` sebagai Administrator (Izin Administrator).

`setup.exe` akan menyediakan menu untuk:

- install/uninstall service Windows `NusaTunnelClient`
- menjalankan/stop aplikasi GUI client
- aksi lock/unlock pembuatan database baru

Catatan paket terbaru:
- Entry point resmi installer Windows adalah `setup.exe` (menu interaktif).
- `NusaTunnelClient` dijalankan lewat wrapper headless `nusatunnel-service.exe`, lalu wrapper itu mengeksekusi `nusatunnel.exe client.toml`.
- Script template lama seperti `setup-client.cmd`/`install-service.cmd` bukan alur utama bundle dashboard saat ini.
- Saat install sukses, shortcut desktop `nusatunnel` dibuat untuk membuka GUI jendela utama dengan Izin Administrator.
- `setup.exe` akan auto-install service `NusaTunnelDB` dulu (fail-fast jika gagal), lalu install `NusaTunnelClient`.
- Runtime file `pgbouncer.ini` dan `userlist.txt` dibuat otomatis saat install service.
- Untuk menyiapkan asset Windows:
  - `scripts/build_windows_unified.ps1` membangun `setup.exe`
  - `scripts/build_windows_service_wrapper.ps1` membangun `nusatunnel-service.exe`
  - `scripts/build_windows_gui.ps1` membangun `nusatunnel-gui.exe`
  - `assets/windows/nusatunnel.exe` tetap dipakai sebagai binary sambungan Windows custom milik repo ini

### Isi bundle Windows 7 (terpisah)

- `setup.exe`
- `nusatunnel-service.exe`
- `nusatunnel.exe`
- `client.toml` (File Pengaturan)
- file pendukung database

Catatan Windows 7:
- Download melalui route `GET /download/windows7`.
- Asset bundle dibaca dari folder terpisah `assets/windows7` lalu disalin ke `resources/assets/windows7`.
- Varian Windows 7 sengaja **tanpa** `nusatunnel-gui.exe`.
- Installer Win7 fokus ke service tunnel/kompatibilitas Win7; GUI desktop tidak termasuk di paket ini.

### Uninstall

- Jalankan `setup.exe` sebagai Administrator (Izin Administrator) lalu pilih menu uninstall/cleanup service.

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
- membuat service `nusatunnel-client`
- enable + start service saat install selesai

---

## 6) Nama service/task

- Server rathole: `rathole`
- Dashboard: `nusatunnel-dashboard`
- Linux client: `nusatunnel-client`
- Windows client service: `NusaTunnelClient`
- Windows GUI scheduled task: `NusaTunnelClientGUI`

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
systemctl status nusatunnel-dashboard
journalctl -u rathole -n 100 --no-pager
journalctl -u nusatunnel-dashboard -n 100 --no-pager
sudo ss -ltnp | grep -E ':5444|:5480|:5485|:8088'
```

Lihat panduan operasional lanjutan di: `docs/OPERATIONS.md`.
