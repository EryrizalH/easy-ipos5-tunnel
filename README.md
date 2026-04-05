# Easy Rathole

Auto installer berbasis Bash untuk **Ubuntu 22+** yang menyiapkan:

- Rathole server
- Expose TCP fixed port: **5444**, **5480**, **5485**
- Dashboard web HTTP dengan Basic Auth
- Setup/rotasi 1 global token via dashboard
- Generator installer client:
  - Windows (ZIP, auto-start service via NSSM)
  - Linux (ZIP, systemd service)

---

## 1) Install Server (Ubuntu 22+)

### Opsi A (paling gampang) — langsung dari repo publik

```bash
curl -fsSL https://raw.githubusercontent.com/pruedence21/easy-ipos5-tunnel/main/public-install.sh | sudo bash
```

Installer akan **menyiapkan hardening server dulu** sebelum install Rathole/dashboard:

- update package index
- install dan aktifkan `ufw` + baseline rule (deny incoming, allow outgoing, allow SSH)
- install & aktifkan `fail2ban` untuk SSH
- aktifkan `unattended-upgrades`
- apply baseline `sysctl` hardening network
- apply baseline `sshd` hardening (aman default, tanpa memaksa disable password)

### Opsi B — clone repo lalu jalankan installer

```bash
git clone https://github.com/pruedence21/easy-ipos5-tunnel.git
cd easy-ipos5-tunnel
sudo bash install.sh
```

### Opsi C — jika sudah berada di folder project lokal

```bash
sudo bash install.sh
```

### Opsi parameter hardening (opsional)

```bash
# skip hardening (tidak direkomendasikan)
sudo EASY_RATHOLE_HARDENING=0 bash install.sh

# tetap hardening, tapi skip apt upgrade (lebih cepat)
sudo EASY_RATHOLE_RUN_UPGRADE=0 bash install.sh

# disable SSH password auth (HANYA jika key-based login sudah siap)
sudo EASY_RATHOLE_DISABLE_SSH_PASSWORD=1 bash install.sh

# batasi akses SSH hanya dari CIDR tertentu
sudo EASY_RATHOLE_SSH_ALLOW_CIDR="1.2.3.4/32" bash install.sh

# batasi akses dashboard hanya dari CIDR tertentu
sudo EASY_RATHOLE_DASHBOARD_ALLOW_CIDR="1.2.3.4/32" bash install.sh
```

> ⚠️ **Penting**: Gunakan `EASY_RATHOLE_DISABLE_SSH_PASSWORD=1` hanya jika Anda sudah bisa login SSH pakai key. Installer akan menolak setting ini jika tidak menemukan `authorized_keys`.

Setelah selesai, installer akan menampilkan:

- URL dashboard
- Username dashboard
- Lokasi file password dashboard
- Control port Rathole (random)
- Port forward yang dibuka

### Lokasi penting

- State file: `/opt/easy-rathole/state/install-state.json`
- Rathole config: `/etc/easy-rathole/server.toml`
- Dashboard credentials file: `/opt/easy-rathole/state/dashboard-credentials.txt`

---

## 2) Dashboard

Dashboard default berjalan di port `8088`.

Fitur:

1. Login dengan Basic Auth
2. Set/rotate global token
3. Lihat status service `rathole` dan `easy-rathole-dashboard`
4. Download installer client:
   - Windows ZIP
   - Linux ZIP

---

## 3) Client Windows

1. Download bundle dari dashboard
2. Extract ZIP
3. Run `install-service.ps1` sebagai Administrator
4. Service akan auto-start saat boot

Uninstall:

- Run `uninstall-service.ps1` sebagai Administrator

---

## 4) Client Linux

1. Download bundle dari dashboard
2. Extract ZIP
3. Jalankan:

```bash
sudo ./install-client.sh
```

Service client akan aktif dan auto-start saat boot.

---

## 5) Service names

- `rathole`
- `easy-rathole-dashboard`
- Linux client default: `easy-rathole-client`
- Windows client default: `EasyRatholeClient`

---

## 6) Security notes

- Dashboard saat ini HTTP-only (tanpa TLS)
- Disarankan membatasi akses dashboard via firewall (IP whitelist)
- Simpan file credentials dengan aman

---

## 7) Troubleshooting cepat

```bash
systemctl status rathole
systemctl status easy-rathole-dashboard
journalctl -u rathole -n 100 --no-pager
journalctl -u easy-rathole-dashboard -n 100 --no-pager
```

Detail tambahan ada di `docs/OPERATIONS.md`.
