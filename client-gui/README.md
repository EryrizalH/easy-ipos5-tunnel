# NusaTunnel Client GUI (Windows)

Desktop GUI berbasis Go + Wails v2 untuk monitor dan kontrol service `NusaTunnelClient`.

## Fitur

- Monitor status koneksi (`Connected` / `Disconnected`)
- Deteksi kegagalan autentikasi token (`Auth Failed`) dari log/event Windows
- Monitor status service client
- Monitor status server (`Dashboard Aktif/Tidak Aktif`, `Control Port Aktif/Tidak Aktif`)
- Tampilkan IP publik server dari resolve `remote_addr` di File Pengaturan (`client.toml`)
- Kontrol service: Start / Stop / Restart
- Auto-start via Task Scheduler (Run at logon)
- System tray: buka dashboard, refresh, start/stop/restart, exit
- Close window (`X`) => hide ke tray

## Lokasi config GUI

Config disimpan di:

`%AppData%\nusatunnel-client-gui\config.json`

Field penting:

- `configPath`: lokasi `client.toml`
- `autoStartEnabled`: status preferensi auto-start

Installer Windows menjalankan GUI dengan `--config` ke runtime terkelola agar konfigurasi bundle lama tidak dipakai kembali.

## Development

```powershell
cd client-gui
go mod tidy
go test ./...
```

> Untuk menjalankan sebagai Wails app, gunakan workflow build/run Wails di mesin yang sudah terpasang Wails CLI.

## Catatan Log Service (Windows)

Installer service menulis log ke:

`%ProgramData%\nusatunnel-client\logs\NusaTunnelClient.stderr.log`

GUI menggunakan log tersebut (dan fallback event log Windows) untuk mendeteksi indikasi token mismatch/auth failure.
