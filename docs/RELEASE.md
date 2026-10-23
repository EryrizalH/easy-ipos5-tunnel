# Release NusaTunnel

## Membuat release

1. Lengkapi source frontend yang dilacak di `client-gui/frontend/dist`. Folder ini adalah source saat ini, bukan output frontend yang boleh dibuang.
2. Pastikan repository secrets `IPOS5_DB_USERNAME` dan `IPOS5_DB_PASSWORD` tersedia. Pengaturan berada di Settings → Secrets and variables → Actions. Jangan menulis nilai ke command line atau chat; GitHub CLI dapat membaca nilai melalui stdin.
3. Merge perubahan ke `main`, lalu buat dan push tag stabil baru:

```bash
git tag v1.0.0
git push origin v1.0.0
```

CI menguji dashboard, installer release, Go modern, serta kompatibilitas build Go 1.20.14. Release membangun wrapper dan GUI sebelum setup modern; setup Windows 7 dibangun terpisah tanpa GUI. Tag selain `vX.Y.Z` ditolak.

Hanya job release Windows menerima kedua secret, melalui environment generator. File Go hasil generate di-ignore dan dihapus setelah build; cache Go release tidak diunggah. Tes PR tidak mendapat secret. Generator wajib berhasil sebelum kedua setup dibangun.

**Kredensial tetap dapat diekstrak dari setup Windows publik**, termasuk setup di dalam paket server. GitHub Secrets melindungi source dan log CI, bukan isi executable yang dipublikasikan. Jangan menganggap kredensial ini rahasia dari penerima installer.

Workflow membuat draft, mengunggah delapan aset lengkap, memeriksa checksum dan daftar aset, lalu menerbitkannya. Release yang sudah terbit tidak ditimpa; perbaikan menggunakan tag baru. Draft yang gagal dapat dilanjutkan dengan rerun workflow. Pengaturan immutable releases dapat diaktifkan pada repository; workflow sudah memakai urutan draft/upload/publish.

## Kontrak aset

| Nama | Isi |
|---|---|
| `public-install.sh` | Bootstrap installer |
| `nusatunnel-server.tar.gz` | File deployment, manifest, dua setup, dua runtime Linux |
| `nusatunnel-windows-amd64-setup.exe` | Setup Windows modern mandiri |
| `nusatunnel-windows7-amd64-setup.exe` | Setup Windows 7 tanpa GUI |
| `rathole-x86_64-unknown-linux-gnu.zip` | Runtime Linux x64 |
| `rathole-aarch64-unknown-linux-musl.zip` | Runtime Linux ARM64 |
| `release.json` | Tag, repository, commit, versi Rathole dan checksum runtime/setup |
| `SHA256SUMS` | SHA-256 semua aset selain file checksum itu sendiri |

Paket server memakai allowlist deployment; source Go, tes, panduan AI, log, `.env`, dan `client.toml` aktif tidak ikut. Dashboard membuat `client.toml` berisi token hanya di VPS; ZIP client Windows tetap tepat dua file.

`scripts/release-dependencies.json` mengunci Rathole Linux v0.5.0 dan fingerprint runtime Windows yang sudah ada. Source runtime Rathole Windows mengacu ke v0.5.0; asal NSSM, PgBouncer, dan DLL tercatat dalam lock. Versi persis sebagian runtime vendored lama belum terdokumentasi; checksum mempertahankan file yang sama, bukan membuktikan asal build. Perubahan vendor harus memperbarui lock setelah runtime diverifikasi. Jangan mengganti Rathole Windows dengan binary upstream MSVC tanpa memeriksa kebutuhan VCRUNTIME dan kompatibilitas Windows 7.

## Tracking Git

- Dilacak: source, tes, template, dokumentasi, workflow, ikon, `go.mod`/`go.sum`, frontend `dist`, serta runtime pihak ketiga sementara.
- Di-ignore: setup, GUI, wrapper hasil build, `.syso`, runtime Linux hasil download, file kredensial hasil generate, `.env`, konfigurasi aktif, `userlist.txt`, log, database, cache, staging, dan bundle hasil generate.
- Penghapusan dari tracking mempertahankan file lokal. `.gitignore` tidak membersihkan riwayat Git.

## Kredensial lama

Audit menemukan nilai kredensial lama dalam README utama, README lock-ipos, default monitor Python, `internal/db/conn.go`, `internal/models/config.go`, dan kedua setup lama. Log lock-ipos juga memuat identitas pengguna. Nilai langsung dihapus dari source baru, sedangkan setup/log lama dilepas dari tracking.

Kredensial yang sudah pernah dipush harus dianggap terekspos dan perlu dirotasi pada database yang menggunakannya, lalu diperbarui di GitHub Secrets. Pembersihan ini tidak merotasi database, menulis ulang riwayat, menghapus clone orang lain, atau menghapus aset lama yang sudah dipublikasikan. Installer publik baru tetap mengandung kredensial build sesuai kebijakan di atas.

## Verifikasi lokal

```powershell
python -m unittest discover -s tests
Push-Location dashboard
..\.venv\Scripts\python.exe -m unittest discover -s app
..\.venv\Scripts\python.exe -m unittest discover -s app/services
Pop-Location
```

Tes release memakai HTTP lokal dan fixture setup/runtime, tanpa menjalankan instalasi sistem. Tes terminal/permission Unix dijalankan di CI Ubuntu. Pada Windows, arahkan `NUSA_TEST_BASH` ke Git Bash jika `bash` default mengarah ke WSL. Build GUI wajib berhenti bila source frontend tidak lengkap; jangan memulihkan penghapusan lokal otomatis.

Instalasi penuh dan perilaku service Windows 7 tetap perlu diuji pada VM/VPS disposable sebelum rollout produksi.
