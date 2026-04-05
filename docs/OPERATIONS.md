# Easy Rathole Operations Guide

## Cek status service

```bash
systemctl is-active rathole
systemctl is-active easy-rathole-dashboard
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

1. Login dashboard
2. Set token baru
3. Download ulang installer client
4. Re-deploy client dengan bundle baru

> Catatan: client dengan token lama akan gagal autentikasi setelah rotasi.

## Verifikasi port listening

```bash
sudo ss -ltnp | grep -E ':5444|:5480|:5485|:8088'
```

Untuk control port Rathole, lihat dari state file:

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

Installer bersifat idempotent dasar. Aman dijalankan ulang:

```bash
sudo bash install.sh
```

Namun setelah re-run:

- cek ulang service status
- cek state file
- verifikasi dashboard masih bisa login
