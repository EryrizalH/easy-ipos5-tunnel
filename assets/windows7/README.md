Folder ini disediakan untuk asset bundle client Windows 7 yang dipisah dari `assets/windows`.

Route `GET /download/windows7` hanya memakai `setup.exe` mandiri; ZIP hasil download berisi `setup.exe` dan `client.toml`.

Saat membangun `setup.exe`, file berikut digabungkan sebagai payload internal:

- `nusatunnel-service.exe`
- `nusatunnel.exe`
- `nssm.exe`
- `pgbouncer.exe`
- `libevent-7.dll`
- `libssl-3-x64.dll`
- `libcrypto-3-x64.dll`
- `libwinpthread-1.dll`

Varian ini sengaja tanpa `nusatunnel-gui.exe`.

`setup.exe` dan wrapper adalah output build yang di-ignore. GitHub Actions membangunnya dengan Go 1.20.14; runtime pihak ketiga tetap dilacak dan fingerprint-nya dikunci dalam `scripts/release-dependencies.json`. Distribusi resmi diambil dari GitHub Release. Kredensial build berasal dari GitHub Secrets dan tetap dapat diekstrak dari executable publik; lihat `docs/RELEASE.md`.
