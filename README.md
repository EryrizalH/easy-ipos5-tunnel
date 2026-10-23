# Nusa IPOS 5 Tunnel (NusaTunnel)

Installer otomatis berbasis Bash untuk **Ubuntu 22+** agar IPOS 5 bisa diakses via internet menggunakan reverse sambungan (`rathole`).

## Ringkasan fitur

- Install server sambungan + layanan (`rathole`)
- Jalur forward TCP server (VPS): **5444**, **5480**, **5485**
- Mapping database default: **VPS 5444 -> Client 127.0.0.1:5444 (NusaTunnelClient) -> Pengoptimal Database (NusaTunnelDB) 0.0.0.0:5444 -> Database 127.0.0.1:5445**
- Jalur kontrol client ke VPS default TCP **443** agar koneksi sambungan terlihat seperti akses HTTPS.
- Dashboard NusaTunnel (HTTP + Basic Auth) untuk:
  - melihat status layanan dan status jalur forward
  - set/rotasi global token
  - download bundle client Windows/Linux
- Hardening baseline server saat install (UFW, fail2ban, unattended-upgrades, sysctl, baseline sshd)
- Generator bundle client:
  - **Windows**: ZIP dua file (`setup.exe` mandiri + `client.toml`)
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

### Install dari GitHub Release

Prasyarat: Ubuntu 22+ dengan **Python 3.11+** dan `curl`. Python bawaan Ubuntu 22 belum memenuhi kebutuhan dashboard; siapkan Python 3.11+ sebelum instalasi. Pemeriksaan dilakukan sebelum hardening.

```bash
curl -fsSL https://github.com/EryrizalH/easy-ipos5-tunnel/releases/latest/download/public-install.sh | sudo bash
```

Bootstrap memilih satu tag stabil, memeriksa checksum dan kelengkapan paket, lalu menjalankan installer. Source branch `main` tidak digunakan sebagai distribusi. Paket server sudah membawa setup Windows/Windows 7 dan runtime Linux versi release tersebut.

Untuk mengunci versi:

```bash
curl -fsSL https://github.com/EryrizalH/easy-ipos5-tunnel/releases/latest/download/public-install.sh | sudo NUSA_TUNNEL_VERSION=v1.0.0 bash
```

Installer meminta username dan password database IPOS 5 melalui terminal; password tidak ditampilkan. Kredensial monitor disimpan di `/etc/nusatunnel/postgres-monitor.env` dengan permission `0600`, lalu dipakai ulang saat instalasi berikutnya. File tersebut tidak ikut bundle client atau paket Python publik. Instalasi tanpa terminal wajib menyediakan `NUSA_TUNNEL_PG_MONITOR_USER` dan `NUSA_TUNNEL_PG_MONITOR_PASSWORD` melalui environment privat.

Checkout Git tetap untuk pengembangan. `install.sh` memerlukan paket deployment lengkap beserta `release.json`, bukan checkout source biasa.

### Apa yang dilakukan installer

Urutan default:

1. Hardening baseline server.
2. Install dependency runtime.
3. Install rathole server + layanan `rathole`.
4. Install dashboard + layanan `nusatunnel-dashboard`.
5. Buka firewall untuk jalur kontrol **443** + remote bind ports sambungan (default 5444/5480/5485) + jalur dashboard.

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
sudo DASHBOARD_PORT=9090 bash install.sh

# ganti port kontrol rathole (default 443; selain 443 tidak lagi terlihat seperti HTTPS)
sudo NUSA_TUNNEL_RATHOLE_CONTROL_PORT=8443 bash install.sh
```

> ⚠️ `NUSA_TUNNEL_DISABLE_SSH_PASSWORD=1` hanya aman jika login SSH key-based sudah teruji. Script akan menolak jika `authorized_keys` tidak ditemukan.

### Opsi untuk `public-install.sh`

- `NUSA_TUNNEL_VERSION`: tag stabil `vX.Y.Z`; default release stabil terbaru.
- `REPO_URL`: default `https://github.com/EryrizalH/easy-ipos5-tunnel`.
- `REPO_BASE_DIR`: direktori staging/source, default `/opt/nusatunnel`. Root runtime tetap mengikuti `NUSA_TUNNEL_ROOT` (default `/opt/nusatunnel`).
- `REPO_BRANCH`: sudah dihentikan; gunakan tag release.

Setiap instalasi memakai staging baru di `releases/<tag>/install-*`. State dan checkout Git lama tetap dipertahankan. Runtime Linux client dipin ke release server saat bundle dibuat.

### Output akhir installer

Setelah install selesai, installer menampilkan:

- URL dashboard
- username dashboard
- password dashboard langsung di terminal
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

Default berjalan di port `8088` (atau sesuai `DASHBOARD_PORT` saat instalasi, lalu tersimpan sebagai `dashboard_port` pada state).

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
- `NUSA_TUNNEL_PG_MONITOR_USER` (diisi saat instalasi VPS; tanpa default)
- `NUSA_TUNNEL_PG_MONITOR_PASSWORD` (diisi saat instalasi VPS; tanpa default)
  - `NUSA_TUNNEL_PG_MONITOR_DBNAME` (default `postgres`)
  - `NUSA_TUNNEL_PG_MONITOR_TIMEOUT_SEC` (default `3`)

---

## 4) Client Windows

### Isi bundle Windows (modern)

- `setup.exe`
- `client.toml` (File Pengaturan)

### Cara install (disarankan)

1. Download bundle dari dashboard.
2. Extract ZIP.
3. Jalankan `setup.exe` sebagai Administrator (Izin Administrator).

`setup.exe` memuat runtime offline dan akan menyalin File Pengaturan ke `%ProgramData%\nusatunnel-client\runtime`. Setelah instalasi berhasil, folder hasil extract boleh dihapus.

`setup.exe` akan menyediakan menu untuk:

- install/uninstall service Windows `NusaTunnelClient`
- menjalankan/stop aplikasi GUI client
- aksi lock/unlock pembuatan database baru

Catatan paket terbaru:
- Entry point resmi installer Windows adalah `setup.exe` (menu interaktif).
- Saat menjalankan installer, `client.toml` wajib berada di folder yang sama dengan `setup.exe`.
- `NusaTunnelClient` tetap dijalankan lewat wrapper headless dengan config runtime yang tersalin.
- Saat install sukses, shortcut desktop `nusatunnel` dibuat untuk membuka GUI jendela utama dengan Izin Administrator.
- Menu **Install IP Public** menyesuaikan PostgreSQL sebelum memasang service tunnel: `max_connections=200`, keepalive `60/10/0` (count `0` untuk Windows/PostgreSQL 9.5), `work_mem=8MB`, serta logging koneksi/diskoneksi dengan waktu, PID, user, database, dan alamat client. `shared_buffers` dihitung sebesar 25% RAM Windows, minimum 128 MB dan maksimum 512 MB.
- Installer mencocokkan instance aktif dengan service dan instalasi PostgreSQL yang dipilih, lalu membackup konfigurasi sebelum tuning. Override parameter tuning di `postgresql.auto.conf` dibersihkan tanpa mengubah pengaturan lain. Jika deteksi ambigu, proses dihentikan; jika penerapan, restart, atau verifikasi gagal, konfigurasi dipulihkan dan pemasangan tunnel dibatalkan.
- PostgreSQL direstart bila diperlukan dan koneksi database akan terputus sementara. Restart dilewati jika seluruh nilai efektif sudah sesuai. Instalasi lama mendapat tuning dengan menjalankan ulang **Install IP Public**; menu Pengoptimal Database tetap memakai alurnya sendiri.
- Menu Pengoptimal Database memasang service `NusaTunnelDB` secara fail-fast bila preflight gagal.
- Runtime file `pgbouncer.ini` dan `userlist.txt` dibuat otomatis saat install service.
- Untuk menyiapkan asset Windows:
  - `scripts/build_windows_service_wrapper.ps1` membangun `nusatunnel-service.exe`
  - `scripts/build_windows_gui.ps1` membangun `nusatunnel-gui.exe`
  - `scripts/build_windows_unified.ps1` membangun ulang GUI terlebih dahulu, lalu membangun dan menggabungkan runtime ke `setup.exe`; payload modern diverifikasi mendukung argumen `--config` yang dipakai shortcut
  - `scripts/build_windows7_assets.ps1` membangun installer dan wrapper Windows 7 dengan Go 1.20.14, tanpa GUI desktop
  - `assets/windows/nusatunnel.exe` dan `assets/windows7/nusatunnel.exe` dibangun dari source resmi Rathole v0.5.0; build ini tidak mengimpor `VCRUNTIME140.dll`, dependensi yang memicu kegagalan service sebelumnya.

### Isi bundle Windows 7 (terpisah)

- `setup.exe`
- `client.toml` (File Pengaturan)

Catatan Windows 7:
- Download melalui route `GET /download/windows7`.
- Varian Windows 7 sengaja tanpa GUI desktop; runtime tunnel dan Pengoptimal Database sudah ada di dalam `setup.exe`.
- `client.toml` wajib berada di folder yang sama dengan `setup.exe` saat instalasi, lalu folder hasil extract boleh dihapus.

### Uninstall

- Jalankan `setup.exe` sebagai Administrator (Izin Administrator) lalu pilih menu uninstall/cleanup service.
- Tuning PostgreSQL dari menu **Install IP Public** tetap tersimpan setelah uninstall tunnel.

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

## Workflow release dan kredensial Windows

Push tag `vX.Y.Z` menjalankan CI/build dan menerbitkan GitHub Release. Detail aset, tracking Git, verifikasi, dan prosedur rotasi ada di [docs/RELEASE.md](docs/RELEASE.md).

Build Windows mengambil `IPOS5_DB_USERNAME` dan `IPOS5_DB_PASSWORD` dari GitHub Secrets. Nilainya tidak disimpan di source baru, tetapi **tetap dapat diekstrak dari installer Windows publik**. Kredensial yang pernah masuk riwayat Git perlu dianggap terekspos dan dirotasi pada database terkait.
