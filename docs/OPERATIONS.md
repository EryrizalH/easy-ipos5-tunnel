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

Catatan alur database default:
- Port VPS tetap `5444`.
- Client Windows tetap menjalankan rathole ke `127.0.0.1:5444`.
- Saat mode PgBouncer aktif, PostgreSQL pindah ke `127.0.0.1:5445` dan PgBouncer listen di `0.0.0.0:5444` agar bisa diakses lokal maupun LAN.
- Installer Windows juga membuat rule firewall inbound TCP `5444` untuk semua sumber.
- Pastikan file PGbouncer (`pgbouncer.ini` dan `userlist.txt`) tersimpan aman dan konsisten dengan kredensial PG lock.

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
