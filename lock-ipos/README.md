# PostgreSQL Self-Permission Manager

Tools kecil berbasis TUI (Terminal User Interface) dengan Golang untuk Windows. Tools ini akan mengatur permission **diri sendiri** pada PostgreSQL - apakah user yang sedang login boleh atau tidak boleh membuat database baru (CREATEDB/NO CREATEDB privilege).

## Fitur

- ✅ Auto-detect PostgreSQL bin location (2 default paths)
- ✅ Manual input path jika auto-detect gagal
- ✅ 2 opsi menu: Lock atau Allow buat database
- ✅ Current permission status display
- ✅ Preview SQL command sebelum eksekusi
- ✅ Confirmation dialog
- ✅ Success/error feedback
- ✅ Self-permission change (ALTER USER on self only)

## Requirements

- Windows 10/11
- Go 1.23+ (untuk build)
- PostgreSQL 9.5+ dengan akses ke psql.exe

## Connection Data (Hardcoded)

- **Host**: `localhost`
- **Port**: `5444`
- **User**: `sysi5adm`
- **Password**: `u&aV23cc.o82dtr1x89c`
- **Database**: `postgres`

## PostgreSQL Binary Locations (Auto-Detect)

- `C:\Program Files (x86)\Inspirasibiz\Server System 1.0\pgsql9.5\bin`
- `D:\Server System 1.0\pgsql9.5\bin`

## Installation & Build

### 1. Install Dependencies

```bash
go mod download
```

### 2. Build Windows Binary

Menggunakan PowerShell build script:

```powershell
.\build.ps1
```

Atau manual:

```bash
go build -o lock-ipos.exe -ldflags "-s -w" .
```

### 3. Run Application

```bash
.\lock-ipos.exe
```

## Usage

### Keyboard Shortcuts

- `1` - Pilih opsi Lock (Tidak Boleh Buat DB)
- `2` - Pilih opsi Allow (Boleh Buat DB)
- `Enter` - Confirm/Lanjut
- `Esc` - Back/Batal
- `q` - Quit application

### User Flow

1. User jalankan `lock-ipos.exe`
2. Aplikasi deteksi PostgreSQL bin di 2 lokasi default
3. Jika ditemukan → tampilkan menu, jika tidak → minta input path
4. User pilih opsi: Lock atau Allow
5. Preview & konfirmasi perubahan
6. Eksekusi PSQL command
7. Tampilkan result & opsi kembali/exit

## Project Structure

```
lock-ipos/
├── main.go                 # Entry point
├── go.mod                  # Go module dependencies
├── go.sum                  # Dependency checksums
├── internal/
│   ├── pgpath/
│   │   └── detector.go     # PostgreSQL binary path detection
│   ├── db/
│   │   └── conn.go         # PostgreSQL connection & permission queries via psql
│   ├── tui/
│   │   ├── styles.go       # Lipgloss style definitions
│   │   ├── components.go   # Reusable TUI components
│   │   └── views.go        # View screens (menu, path input, options, result)
│   └── models/
│       └── config.go       # Config data models
├── README.md               # Build & usage instructions
└── build.ps1               # PowerShell script untuk Windows build
```

## Verification

### Testing Steps

1. **Install Dependencies**
   ```bash
   go mod download
   ```

2. **Run Application**
   ```bash
   go run main.go
   ```

3. **Test Path Detection**
   - Pastikan salah satu path PostgreSQL ada di sistem
   - Verify aplikasi mendeteksi path otomatis
   - Atau test manual path input

4. **Test Current Permission Check**
   - Lihat status CREATEDB saat ini di menu
   - Verify sesuai dengan actual permission

5. **Test Lock Permission**
   - Pilih opsi 1 (Lock)
   - Confirm preview SQL shows `ALTER USER sysi5adm NOCREATEDB`
   - Verify success message
   - Verify status berubah ke "TIDAK BOLEH Buat DB"

6. **Test Allow Permission**
   - Pilih opsi 2 (Allow)
   - Confirm preview SQL shows `ALTER USER sysi5adm CREATEDB`
   - Verify success message
   - Verify status berubah ke "BOLEH Buat DB"

7. **Verify Database Permission**
   - Setelah Lock, test: `CREATE DATABASE testdb;` harus gagal
   - Setelah Allow, test: `CREATE DATABASE testdb;` harus sukses

8. **Build Windows Binary**
   ```powershell
   .\build.ps1
   ```
   - Run `.\lock-ipos.exe` di Windows
   - Verify standalone binary works tanpa Go installation

### Manual Testing Checklist

- [ ] Path detection works di C:\Program Files (x86)\...
- [ ] Path detection works di D:\Server System 1.0\...
- [ ] Manual path input works jika default tidak ada
- [ ] Current permission status displayed correctly
- [ ] Lock option executes NOCREATEDB correctly
- [ ] Allow option executes CREATEDB correctly
- [ ] SQL preview accurate
- [ ] Confirmation dialog works
- [ ] Success/error messages clear
- [ ] Keyboard navigation smooth
- [ ] Windows executable runs standalone
- [ ] PSQL password environment variable handled correctly

## Security Considerations

- **Password tidak hardcoded di source code** → Simpan di struct
- PSQL connection menggunakan environment variable `PGPASSWORD`
- Password tidak ditampilkan di terminal (PSQL handles this)
- Confirmation dialog sebelum ALTER USER
- No SQL injection risk (hardcoded queries)

## Tech Stack

- **Language**: Go (Golang)
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Elm architecture untuk TUI
- **Styling**: [Lipgloss](https://github.com/charmbracelet/lipgloss) - styling TUI
- **PostgreSQL CLI**: Execute `psql.exe` via `os/exec`
- **Windows Build**: Cross-compilation untuk Windows

## License

MIT License
