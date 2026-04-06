# Easy Rathole Client GUI (Windows)

Desktop GUI berbasis Go + Wails v2 untuk monitor dan kontrol service `EasyRatholeClient`.

## Fitur

- Monitor status koneksi (`Connected` / `Disconnected`)
- Deteksi kegagalan autentikasi token (`Auth Failed`) dari log/event Windows
- Monitor status service client
- Monitor status server (`Dashboard Up/Down`, `Control Port Up/Down`)
- Tampilkan IP publik server dari resolve `remote_addr` di `client.toml`
- Kontrol service: Start / Stop / Restart
- Auto-start via Task Scheduler (Run at logon)
- System tray: open dashboard, refresh, start/stop/restart, exit
- Close window (`X`) => hide ke tray

## Lokasi config GUI

Config disimpan di:

`%AppData%\easy-rathole-client-gui\config.json`

Field penting:

- `configPath`: lokasi `client.toml`
- `autoStartEnabled`: status preferensi auto-start

## Development

```powershell
cd client-gui
go mod tidy
go test ./...
```

> Untuk menjalankan sebagai Wails app, gunakan workflow build/run Wails di mesin yang sudah terpasang Wails CLI.

## Catatan Log Service (Windows)

Installer service menulis log ke:

`%ProgramData%\easy-rathole-client\logs\EasyRatholeClient.stderr.log`

GUI menggunakan log tersebut (dan fallback event log Windows) untuk mendeteksi indikasi token mismatch/auth failure.
