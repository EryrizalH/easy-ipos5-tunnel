# IPOS5TunnelPublik

Installer otomatis berbasis Bash untuk **Ubuntu 22+** agar IPOS 5 bisa diakses via internet menggunakan reverse tunnel `rathole`.

## Ringkasan fitur

- Install server `rathole` + systemd service (`rathole`)
- Port forward TCP tetap: **5444**, **5480**, **5485**
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
3. Install rathole server + service `rathole`.
4. Install dashboard + service `easy-rathole-dashboard`.
5. Buka firewall untuk control port + port 5444/5480/5485 + port dashboard.

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
   - status port 5444/5480/5485
4. Download bundle client:
   - `GET /download/windows`
   - `GET /download/linux`

Health check endpoint:

- `GET /health` → `{"status":"ok"}`

---

## 4) Client Windows

### Isi bundle Windows

- `setup.exe`
- `ipos5-rathole.exe`
- `ipos5-rathole-gui.exe`
- `nssm.exe`
- `client.toml`
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
- Script template lama seperti `setup-client.cmd`/`install-service.cmd` bukan alur utama bundle dashboard saat ini.
- Saat install sukses, shortcut desktop `ipos5-rathole` dibuat untuk membuka GUI jendela utama dengan UAC (Run as Administrator).

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

Lihat panduan operasional lanjutan di: `docs/OPERATIONS.md`.
