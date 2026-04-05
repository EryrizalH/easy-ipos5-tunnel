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

```bash
sudo bash install.sh
```

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
