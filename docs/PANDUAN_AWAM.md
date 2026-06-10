# 🏪 IPOS5TunnelPublik — Akses IPOS 5 dari Mana Saja

## Apa Ini?

Punya **IPOS 5** di komputer toko tapi ingin buka dari rumah atau HP? 

Alat ini membuat IPOS 5 kamu bisa diakses **dari mana saja** — cukup modal VPS murah (sekitar 50-100rb/bulan). Tanpa perlu setting jaringan rumit, tanpa perlu IP publik, tanpa perlu teknisi.

Singkatnya: **sebuah "terowongan rahasia"** yang menghubungkan komputer toko ke internet, aman dan otomatis.

---

## Gambaran Besar

```mermaid
flowchart TB
    PEMILIK["👤 Pemilik Toko<br/>(di rumah / HP)"]

    subgraph VPS["☁️ VPS Murah (Server Cloud)"]
        DASHBOARD["📊 Halaman Web<br/>Pantau Status"]
    end

    subgraph TOKO["🏪 Komputer Toko"]
        IPOS["📦 IPOS 5"]
        DB["🗄️ Database"]
    end

    PEMILIK -->|"Akses lewat internet"| VPS
    VPS -->|"Terowongan aman"| TOKO
    IPOS --> DB

    style VPS fill:#e1f5fe,stroke:#0277bd
    style TOKO fill:#fff3e0,stroke:#e65100
```

> Komputer toko cukup colok internet biasa. Tidak perlu atur-atur modem atau minta IP publik ke ISP.

---

## Cara Kerja (Versi Sederhana)

```mermaid
sequenceDiagram
    participant KAMU as 👤 Kamu
    participant VPS as ☁️ Server Cloud
    participant TOKO as 💻 Komputer Toko

    KAMU->>VPS: 1. Install sekali saja<br/>(cukup copy-paste 1 perintah)
    VPS-->>KAMU: 2. Jadi! Muncul alamat<br/>web + password login

    KAMU->>VPS: 3. Login & download<br/>aplikasi untuk toko
    KAMU->>TOKO: 4. Install di komputer toko<br/>(klik next-next-finish)
    
    TOKO->>VPS: 5. Komputer toko otomatis<br/>nyambung ke server
    
    Note over KAMU,TOKO: ✅ Selesai! IPOS 5 sekarang<br/>bisa dibuka dari mana saja
```

---

## Yang Kamu Butuhkan

| Kebutuhan | Keterangan |
|---|---|
| **VPS** | Server cloud murah (Ubuntu 22.04), bisa pakai DigitalOcean, Linode, atau lokal seperti IdCloudHost |
| **Komputer Toko** | Windows 10/11 tempat IPOS 5 terpasang |
| **Internet** | Di toko dan di tempat kamu mengakses (rumah/HP) |

---

## Langkah 1: Pasang di Server Cloud

Cukup jalankan **satu perintah** ini di server:

```
curl -fsSL https://raw.githubusercontent.com/.../public-install.sh | bash
```

Tunggu beberapa menit. Setelah selesai, kamu dapat:

- **Alamat dashboard** (misal: `http://123.456.789.0:8088`)
- **Username & password** untuk login

```mermaid
graph LR
    PERINTAH["1 Perintah<br/>curl... | bash"] --> PASANG["Otomatis Pasang<br/>Semua Komponen"]
    PASANG --> AMAN["🔒 Amankan Server<br/>(Firewall + Anti-brute force)"]
    PASANG --> TEROWONGAN["🔒 Pasang Terowongan<br/>(Rathole Server)"]
    PASANG --> WEB["📊 Pasang Dashboard<br/>(Halaman Web)"]
    
    style PERINTAH fill:#4caf50,color:#fff
```

---

## Langkah 2: Pasang di Komputer Toko

1. Buka dashboard (pakai username & password yang tadi)
2. Klik **"Download Client Windows"** — dapat file ZIP
3. Ekstrak ZIP, jalankan **setup.exe** (klik kanan → Run as Administrator)
4. Selesai. Muncul ikon kecil di pojok kanan bawah layar:

```mermaid
graph TB
    subgraph TRAY["🖥️ System Tray (Pojok Kanan Bawah)"]
        IKON["🟢 Ikon IPOS5 Tunnel"]
        STATUS["Status: Connected ✅"]
    end

    IKON -->|"Klik kanan"| MENU["📋 Menu:<br/>• Lihat Status<br/>• Mulai/Ulangi/Berhenti<br/>• Keluar"]
    IKON -->|"Klik kiri"| DASH["Buka Dashboard"]
```

Warna ikon:
- **🟢 Hijau** = Terhubung, normal
- **🟡 Kuning** = Sedang menyambung
- **🔴 Merah** = Gagal (cek internet atau token)

---

## Langkah 3: Akses IPOS 5 dari Mana Saja

Setelah terpasang, buka IPOS 5 dengan alamat:

```
Alamat Server Cloud, port 5480
Contoh: 123.456.789.0:5480
```

```mermaid
graph LR
    HP["📱 HP / Laptop<br/>di Rumah"] -->|"123.456.789.0:5480"| VPS["☁️ Server"]
    VPS -->|"Terowongan"| TOKO["💻 IPOS 5<br/>di Toko"]
```

Layanan yang bisa diakses:

| Port | Layanan |
|---|---|
| **5480** | IPOS 5 HTTP (aplikasi kasir web) |
| **5485** | IPOS 5 Worker (pemroses data) |
| **5444** | Database PostgreSQL (untuk teknisi) |

---

## Fitur Keamanan

Semua sudah otomatis terpasang:

| Fitur | Gunanya |
|---|---|
| **Token Rahasia (40 karakter)** | Hanya komputer yang punya token yang bisa nyambung |
| **Firewall (UFW)** | Blokir akses tidak dikenal ke server |
| **Anti-Brute Force (fail2ban)** | Blokir percobaan hack yang mencoba password terus-menerus |
| **SSH Hardening** | Amankan akses remote server |
| **Update Otomatis** | Server selalu pakai patch keamanan terbaru |

```mermaid
graph TB
    HACKER["👾 Pencoba Hack"] -->|"Coba bobol"| FIREWALL["🛡️ Firewall"]
    FIREWALL -->|"❌ Ditolak"| HACKER
    HACKER2["👾 Tebak Password"] -->|"Coba terus"| FAIL2BAN["🚫 fail2ban"]
    FAIL2BAN -->|"⛔ Auto-blokir"| HACKER2
    HACKER3["👾 Tanpa Token"] -->|"Mau nyambung"| TOKEN["🔑 Cek Token"]
    TOKEN -->|"❌ Token salah"| HACKER3
    
    style FIREWALL fill:#4caf50,color:#fff
    style FAIL2BAN fill:#ff9800,color:#fff
    style TOKEN fill:#2196f3,color:#fff
```

---

## Fitur Dashboard

Bisa kamu akses lewat browser (HP atau PC):

```mermaid
graph TB
    LOGIN["🔐 Halaman Login"] --> MENU_UTAMA["📊 Menu Utama"]

    MENU_UTAMA --> STATUS["📡 Status Koneksi<br/>Lihat apakah toko online"]
    MENU_UTAMA --> TOKEN_PAGE["🔑 Kelola Token<br/>Ganti kunci rahasia"]
    MENU_UTAMA --> DOWNLOAD["📦 Download Client<br/>Buat aplikasi untuk toko baru"]
    MENU_UTAMA --> DB_MONITOR["🗄️ Monitor Database<br/>Cek kesehatan data"]

    style MENU_UTAMA fill:#4caf50,color:#fff
```

---

## Opsi Lanjutan: Sinkronisasi Database

Kalau punya lebih dari satu komputer kasir, bisa aktifkan sinkronisasi:

```mermaid
flowchart TB
    subgraph PUSAT["☁️ Server"]
        DB_PUSAT["Database Salinan<br/>(Backup Cloud)"]
    end

    subgraph TOKO1["🏪 Toko 1"]
        DB1["Database Toko 1"]
    end

    subgraph TOKO2["🏪 Toko 2"]
        DB2["Database Toko 2"]
    end

    DB1 <-->|"Sinkron otomatis"| DB_PUSAT
    DB2 <-->|"Sinkron otomatis"| DB_PUSAT
```

Keuntungan:
- Data aman (ada backup di cloud)
- Semua toko punya data yang sama
- Kalau satu komputer rusak, data tidak hilang

---

## Istilah-istilah

| Istilah | Artinya |
|---|---|
| **VPS** | Komputer sewaan di internet yang nyala 24 jam (sekitar 50-100rb/bln) |
| **Terowongan / Tunnel** | Jalur rahasia yang menghubungkan komputer toko ke server |
| **Token** | Kata sandi panjang (40 karakter) sebagai kunci penghubung |
| **Dashboard** | Halaman web untuk memantau dan mengatur semuanya |
| **Port** | "Pintu" di server untuk tiap layanan (5480 = IPOS, 5444 = database) |
| **Firewall** | Tembok pengaman server dari serangan luar |
| **fail2ban** | Penjaga yang otomatis memblokir orang yang coba-coba masuk |

---

## Butuh Bantuan?

Kalau ada masalah:

1. **Cek dashboard** — lihat status koneksi (hijau/kuning/merah)
2. **Pastikan internet toko nyala** — rathole client butuh koneksi
3. **Coba restart** — klik kanan ikon tray → Restart
4. **Hubungi teknisi kamu** kalau masih bermasalah

---

> Dibuat untuk pemilik usaha yang pakai IPOS 5 dan ingin akses dari mana saja, tanpa pusing urusan jaringan.
