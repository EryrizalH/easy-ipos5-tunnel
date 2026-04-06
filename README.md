# IPOS5TunnelPublik

Installer otomatis berbasis Bash untuk **Ubuntu 22+** guna memudahkan IPOS 5 diakses lewat internet menggunakan rathole tunnel.

Yang disiapkan otomatis:

- Server `rathole`
- Forward TCP tetap: **5444**, **5480**, **5485**
- Dashboard web (HTTP + Basic Auth)
- Setup/rotasi 1 global token lewat dashboard
- Generator installer client:
  - Windows (ZIP + auto-start service via NSSM)
  - Linux (ZIP + systemd service)

---

## Quick Start (untuk admin baru)

1. Install server.
2. Login dashboard.
3. Atur global token.
4. Download installer client (Windows/Linux).
5. Jalankan installer client di mesin tujuan.

---

## 1) Install Server (Ubuntu 22+)

### Opsi A (paling cepat) — langsung dari repo publik

```bash
curl -fsSL https://raw.githubusercontent.com/pruedence21/easy-ipos5-tunnel/main/public-install.sh | sudo bash
```

Installer akan **menjalankan hardening server terlebih dahulu** sebelum install rathole/dashboard:

- update package index
- install dan aktifkan `ufw` + baseline rule (deny incoming, allow outgoing, allow SSH)
- install & aktifkan `fail2ban` untuk proteksi SSH
- aktifkan `unattended-upgrades`
- apply baseline hardening `sysctl` jaringan
- apply baseline hardening `sshd` (aman default, tanpa paksa disable password)

### Opsi B — clone repo lalu jalankan installer

```bash
git clone https://github.com/pruedence21/easy-ipos5-tunnel.git
cd easy-ipos5-tunnel
sudo bash install.sh
```

### Opsi C — jika sudah ada di folder project

```bash
sudo bash install.sh
```

### Opsi parameter hardening (opsional)

```bash
# skip hardening (tidak direkomendasikan)
sudo EASY_RATHOLE_HARDENING=0 bash install.sh

# tetap hardening, tapi skip apt upgrade (lebih cepat)
sudo EASY_RATHOLE_RUN_UPGRADE=0 bash install.sh

# nonaktifkan SSH password auth (HANYA jika key-based login sudah siap)
sudo EASY_RATHOLE_DISABLE_SSH_PASSWORD=1 bash install.sh

# batasi akses SSH hanya dari CIDR tertentu
sudo EASY_RATHOLE_SSH_ALLOW_CIDR="1.2.3.4/32" bash install.sh

# batasi akses dashboard hanya dari CIDR tertentu
sudo EASY_RATHOLE_DASHBOARD_ALLOW_CIDR="1.2.3.4/32" bash install.sh
```

> ⚠️ **Penting**: Gunakan `EASY_RATHOLE_DISABLE_SSH_PASSWORD=1` hanya jika login SSH via key sudah teruji. Installer akan menolak jika `authorized_keys` tidak ditemukan.

Setelah selesai, installer akan menampilkan:

- URL dashboard
- Username dashboard
- Lokasi file password dashboard
- Control port rathole (random)
- Port forward yang dibuka

### Lokasi penting

- State file: `/opt/easy-rathole/state/install-state.json`
- Konfigurasi rathole server: `/etc/easy-rathole/server.toml`
- File kredensial dashboard: `/opt/easy-rathole/state/dashboard-credentials.txt`

---

## 2) Dashboard

Dashboard default berjalan di port `8088`.

Fitur utama:

1. Login dengan Basic Auth
2. Set/rotasi global token
3. Lihat status service `rathole` dan `easy-rathole-dashboard`
4. Unduh installer client:
   - Windows ZIP
   - Linux ZIP

---

## 3) Client Windows

1. Unduh bundle dari dashboard
2. Ekstrak ZIP
3. Jalankan `setup-client.cmd` (otomatis minta Administrator/UAC)
4. Service client auto-start saat boot

> `setup-client.cmd` mendukung binary `ipos5-rathole.exe` maupun `rathole.exe`.
> `nssm.exe` sudah termasuk di bundle, jadi tidak perlu unduh manual.

Uninstall:

- Jalankan `uninstall-service.cmd` sebagai Administrator

---

## 4) Client Linux

1. Unduh bundle dari dashboard
2. Ekstrak ZIP
3. Jalankan:

```bash
sudo ./install-client.sh
```

Service client akan aktif dan auto-start saat boot.

---

## 5) Nama service (kompatibilitas)

> Nama service tetap dipertahankan agar tidak breaking change.

- `rathole`
- `easy-rathole-dashboard`
- Linux client default: `easy-rathole-client`
- Windows client default: `EasyRatholeClient`

---

## 6) Catatan keamanan

- Dashboard saat ini masih HTTP-only (tanpa TLS)
- Sangat disarankan membatasi akses dashboard via firewall (IP whitelist)
- Simpan file kredensial dengan aman

---

## 7) Troubleshooting cepat

```bash
systemctl status rathole
systemctl status easy-rathole-dashboard
journalctl -u rathole -n 100 --no-pager
journalctl -u easy-rathole-dashboard -n 100 --no-pager
```

Detail operasional lanjutan ada di `docs/OPERATIONS.md`.
