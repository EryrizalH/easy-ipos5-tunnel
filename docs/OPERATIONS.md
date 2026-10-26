# Panduan Operasional NusaTunnel

## PostgreSQL pada lokasi kustom

Masukkan folder `bin` instalasi yang digunakan IPOS, misalnya `E:\Projects\ipos\Server System 1.0\pgsql9.5\bin`. Installer mencocokkan folder tersebut dengan executable service Windows, argumen `-D`, `postmaster.pid`, dan `postmaster.opts`; lokasi tidak perlu masuk daftar path default.

PostgreSQL 9.5 dapat menulis executable tanpa tanda kutip di `postmaster.opts`, termasuk pada path dengan spasi. Parser memeriksa path terhadap instalasi terpilih sebelum membaca argumen, sehingga tidak memilih database lain hanya berdasarkan nama service.

Jalankan setup sebagai Administrator. Bila proses PostgreSQL menolak pembacaan identitas, installer mencoba hak `SeDebugPrivilege` yang sudah tersedia pada akun admin hanya pada thread pemeriksaan sementara. Hak akses akun maupun DACL proses tidak diubah; ketidakcocokan identitas tetap menghentikan tuning. Jika Windows tidak memberikan hak itu, pesan error menyebut penyebabnya. Jangan memindahkan folder data atau mengubah password database untuk mengatasi error identitas service.

## Cek status service

```bash
systemctl is-active rathole
systemctl is-active nusatunnel-dashboard
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
with open('/opt/nusatunnel/state/install-state.json') as f:
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
sudo systemctl restart nusatunnel-dashboard
```

## Lokasi file

- State: `/opt/nusatunnel/state/install-state.json`
- DB dashboard: `/opt/nusatunnel/state/nusatunnel.db`
- Rathole server config: `/etc/nusatunnel/server.toml`
- Bundle output: `/opt/nusatunnel/bundles`

## Rotasi token

1. Login ke dashboard
2. Set token baru
3. Unduh ulang installer client
4. Deploy ulang client dengan bundle terbaru

> Catatan: client dengan token lama akan gagal autentikasi setelah rotasi.

## Verifikasi port listening

```bash
sudo ss -ltnp | grep -E ':443|:5444|:5480|:5485|:8088'
```

Catatan alur database default:
- Port VPS tetap `5444`.
- Client Windows tetap menjalankan rathole ke `127.0.0.1:5444`.
- Saat mode Pengoptimal Database aktif, PostgreSQL pindah ke `127.0.0.1:5445` dan Pengoptimal Database (NusaTunnelDB) listen di `0.0.0.0:5444` agar bisa diakses lokal maupun LAN.
- Installer Windows juga membuat rule firewall inbound TCP `5444` untuk semua sumber.
- Pastikan file Pengoptimal Database (`pgbouncer.ini` dan `userlist.txt`) tersimpan aman dan konsisten dengan kredensial PG lock.

Control port rathole default `443`. State file tetap sumber kebenaran setelah override:

```bash
python3 - <<'PY'
import json
with open('/opt/nusatunnel/state/install-state.json') as f:
    print(json.load(f).get('rathole_control_port'))
PY
```

## Log service

```bash
journalctl -u rathole -f
journalctl -u nusatunnel-dashboard -f
```

## Jalankan ulang installer

Installer bersifat idempotent dasar, sehingga aman dijalankan ulang:

```bash
sudo NUSA_TUNNEL_RUN_UPGRADE=1 bash install.sh
```

Namun setelah re-run:

- cek ulang status service
- cek state file
- verifikasi dashboard masih bisa login

## Catatan aman sebelum disable SSH password

Jika ingin menjalankan dengan `NUSA_TUNNEL_DISABLE_SSH_PASSWORD=1`, pastikan:

1. Login SSH pakai private key sudah teruji
2. Minimal ada satu file `authorized_keys` valid
3. Jangan menutup sesi SSH aktif sampai verifikasi login sesi baru berhasil


## Release dan monitor database

Instalasi resmi memakai GitHub Release, dengan Python 3.11+ sebagai prasyarat. Pin versi melalui `NUSA_TUNNEL_VERSION=vX.Y.Z`. Bootstrap memeriksa checksum sebelum hardening dan memakai staging baru tanpa menghapus state atau source checkout lama. State menyimpan `nusatunnel_release`, `release_repository`, dan versi upstream pada `rathole_release`.

Installer meminta kredensial database lewat `/dev/tty`, termasuk pada alur `curl | bash`. Kredensial monitor disimpan sebagai environment systemd di `/etc/nusatunnel/postgres-monitor.env`, milik root dengan mode `0600`. Jangan menyalin file tersebut ke repo atau bundle. Instalasi ulang memakainya kembali; untuk mengganti nilai, berikan kedua variabel `NUSA_TUNNEL_PG_MONITOR_USER` dan `NUSA_TUNNEL_PG_MONITOR_PASSWORD` melalui environment privat saat menjalankan installer. Jangan menulis password langsung ke shell history.

Monitor tanpa konfigurasi kredensial/DSN tidak dijalankan. Token tunnel dan password dashboard tetap dikelola secara terpisah. Rotasi kredensial database dilakukan pada instance PostgreSQL terkait, lalu GitHub Secrets untuk build Windows dan environment monitor VPS diperbarui. Lihat [RELEASE.md](RELEASE.md) untuk batas perlindungan binary publik dan aset lama.
