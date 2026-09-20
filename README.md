# 📚 Komikindo Unofficial API

REST API tidak resmi untuk mengambil data komik dari [Komikindo](https://komikindo.ch/) menggunakan teknik web scraping. Dibangun dengan **Go**, **Gin**, **Colly**, dan **GORM**.

---

## ✨ Fitur

- 🔍 **Cari Komik** — Cari komik berdasarkan judul
- 📖 **Daftar Semua Komik** — Ambil seluruh data komik yang sudah di-scrape dan tersimpan di database
- 🔥 **Komik Populer** — Ambil daftar komik populer secara real-time dari halaman utama Komikindo
- 📑 **Daftar Chapter** — Ambil semua chapter dari suatu komik beserta detail info-nya
- 🖼️ **Panel Komik** — Ambil semua gambar panel dari chapter tertentu
- ⏱️ **Auto-Scraping** — Background routine yang otomatis update chapter baru setiap 12 jam
- 🛡️ **Rate Limiting** — Proteksi API dengan rate limiter per IP
- 🔑 **API Key Auth** — Autentikasi aplikasi via header `X-API-Key`
- 👤 **Akun User (JWT)** — Register, login, refresh token dengan rotasi, dan manajemen sesi → [dokumentasi](AUTHENTICATION.md)
- 🔖 **Bookmark** — Simpan komik favorit per user
- 📖 **Riwayat Baca** — Lanjutkan membaca dari chapter & panel terakhir
- 👮 **Role Admin** — Kelola akun user dan status aktifnya
- 🩺 **Health Check** — Endpoint `/health` publik untuk uptime monitoring

---

## 🏗️ Arsitektur & Cara Kerja

```
┌─────────────┐       ┌──────────────┐       ┌──────────────┐
│   Client     │──────▶│   Gin API    │──────▶│    MySQL     │
│  (HTTP Req)  │◀──────│   Server     │◀──────│   Database   │
└─────────────┘       └──────┬───────┘       └──────────────┘
                             │                       ▲
                             │  cache miss?           │ simpan hasil
                             ▼                       │
                      ┌──────────────┐               │
                      │  Colly       │───────────────┘
                      │  Scraper     │
                      │  (komikindo) │
                      └──────────────┘
```

### Alur Kerja

1. **Request masuk** → melewati middleware **Rate Limiter** (max 2 req/s, burst 5) dan **API Key** validation.
2. **Cek database** — Controller pertama kali cek apakah data sudah ada di database (MySQL via GORM).
3. **Cache miss** — Jika data belum ada, Colly akan melakukan scraping langsung ke website Komikindo.
4. **Simpan ke DB** — Hasil scraping disimpan ke database untuk request selanjutnya (caching layer).
5. **Background routine** — Goroutine berjalan setiap **12 jam** untuk update chapter terbaru dari komik yang statusnya masih "Berjalan".
6. **IP Cleanup** — Goroutine terpisah membersihkan data IP dari rate limiter setiap 1 menit untuk IP yang tidak aktif selama 3 menit.

---

## 📁 Struktur Project

```
komikindo-scraper/
├── main.go                          # Entry point
├── bootstrap/
│   └── index.bootstrap.go           # Inisialisasi app, DB, scraper, dan routes
├── config/
│   ├── database.go                  # Koneksi dan migrasi MySQL (GORM)
│   └── loadenv.go                   # Load environment variables
├── controllers/
│   ├── komikindo.controller.go      # Handler untuk endpoint komik
│   ├── auth.controller.go           # Handler register, login, refresh, profil
│   ├── user.controller.go           # Handler bookmark, riwayat baca, admin
│   ├── health.controller.go         # Handler health check
│   └── pagination.go                # Parsing query paginasi & meta
├── helpers/
│   ├── response.go                  # Standar format response JSON
│   ├── auth.go                      # bcrypt, token acak, hash
│   ├── jwt.go                       # Pembuatan & validasi access token
│   └── utils.go                     # Utility (cek koneksi provider)
├── middleware/
│   ├── apikey.go                    # Middleware autentikasi API Key
│   ├── auth.go                      # Middleware JWT: RequireAuth, RequireRole
│   └── ratelimiter.go               # Middleware rate limiter per IP
├── model/
│   ├── komik/
│   │   └── komik.model.go           # Model: Komik, KomikChapter, KomikPanel
│   └── user/
│       └── user.model.go            # Model: User, RefreshToken, Bookmark, ReadingHistory
├── routes/
│   └── index.route.go               # Definisi semua route API
├── scraper/
│   └── komikindo_scraper.go         # Background scraper untuk update chapter
├── routine/
│   ├── komikindo.routine.go         # Routine update chapter berkala
│   └── auth.routine.go              # Routine pembersihan refresh token
├── AUTHENTICATION.md                # Dokumentasi autentikasi & fitur user
├── .env.example                     # Template environment variables
├── go.mod                           # Go module dependencies
└── start.sh                         # Script untuk menjalankan app
```

---

## 🚀 Instalasi & Setup

### Prasyarat

- [Go](https://go.dev/dl/) >= 1.21
- [MySQL](https://dev.mysql.com/downloads/) server yang sudah berjalan

### Langkah-langkah

1. **Clone repository**

   ```bash
   git clone https://github.com/hilmanXcode/komikindo-unofficial-api.git
   cd komikindo-unofficial-api
   ```

2. **Copy dan konfigurasi file environment**

   ```bash
   cp .env.example .env
   ```

   Edit file `.env` dan sesuaikan:

   ```env
   DATABASE_DSN="username:password@tcp(127.0.0.1:3306)/db_komik?parseTime=true"
   SECRET_SELF_API_KEY="your-secret-api-key"
   JWT_SECRET="hasil-dari-openssl-rand-hex-32"
   ```

   | Variable              | Wajib | Default | Deskripsi                                     |
   | --------------------- | ----- | ------- | --------------------------------------------- |
   | `DATABASE_DSN`        | ✅    | —       | Connection string MySQL (format GORM)         |
   | `SECRET_SELF_API_KEY` | ✅    | —       | API key aplikasi, dikirim lewat `X-API-Key`   |
   | `JWT_SECRET`          | ✅    | —       | Kunci tanda tangan JWT, **minimal 32 karakter** |
   | `ACCESS_TOKEN_TTL`    | ❌    | `15m`   | Umur access token                             |
   | `REFRESH_TOKEN_TTL`   | ❌    | `720h`  | Umur refresh token (30 hari)                  |
   | `MAX_LOGIN_ATTEMPTS`  | ❌    | `5`     | Gagal login sebelum akun dikunci              |
   | `LOGIN_LOCK_DURATION` | ❌    | `15m`   | Lama akun dikunci                             |
   | `PORT`                | ❌    | `8000`  | Port HTTP server                              |
   | `CORS_ORIGINS`        | ❌    | localhost 5173/4173 + domain produksi | Origin browser yang diizinkan, dipisah koma |
   | `RATE_LIMIT_RPS`      | ❌    | `10`    | Request per detik per IP untuk `/v1`          |
   | `RATE_LIMIT_BURST`    | ❌    | `30`    | Burst rate limit `/v1`                        |
   | `TRUSTED_PROXIES`     | ❌    | semua   | Proxy yang `X-Forwarded-For`-nya dipercaya, dipisah koma |

   Generate `JWT_SECRET` dengan:

   ```bash
   openssl rand -hex 32
   ```

   > ⚠️ Aplikasi **menolak start** kalau `JWT_SECRET` kosong/kurang dari 32 karakter,
   > atau kalau `SECRET_SELF_API_KEY` kosong.

   File `.env` hanya untuk development. Di production cukup set environment
   variable lewat platform — ketiadaan file `.env` bukan error.

   **Rate limit dan IP pengunjung.** Limit dihitung per IP. Kalau ada proxy di
   depan aplikasi (frontend SSR, CDN, nginx), IP yang dipakai diambil dari header
   `X-Forwarded-For`. Isi `TRUSTED_PROXIES` dengan IP proxy tersebut di production
   supaya pengunjung tidak bisa memalsukan IP untuk menghindari rate limit.

3. **Install dependencies**

   ```bash
   go mod tidy
   ```

4. **Buat database MySQL**

   ```sql
   CREATE DATABASE db_komik;
   ```

   > Tabel akan otomatis dibuat oleh GORM AutoMigrate saat app pertama kali dijalankan.

5. **Jalankan server**

   ```bash
   go run main.go
   ```

   Atau build terlebih dahulu:

   ```bash
   go build -o komikindo-api .
   ./komikindo-api
   ```

   Server akan berjalan di `http://localhost:8000`

---

## 🔐 Autentikasi

API ini punya dua lapis keamanan dengan tugas berbeda:

| Lapis | Header | Menjawab | Berlaku di |
|-------|--------|----------|------------|
| **API Key** | `X-API-Key` | *Aplikasi mana yang memanggil?* | Semua endpoint `/v1/**` |
| **Access Token (JWT)** | `Authorization: Bearer <token>` | *User mana yang sedang login?* | Endpoint bookmark, riwayat, & admin |

Endpoint komik (search, chapter, panel) cukup dengan API key:

```
X-API-Key: your-secret-api-key
```

> Klien lama yang mengirim API key lewat header `Authorization` **tetap berfungsi**, selama isinya bukan token `Bearer`.

Jika API key tidak valid atau tidak disertakan:

```json
{
  "success": false,
  "message": "Invalid API KEY",
  "code": 401,
  "data": null
}
```

📖 **Autentikasi user (register, login, JWT, bookmark, riwayat baca, admin) didokumentasikan terpisah di [AUTHENTICATION.md](AUTHENTICATION.md).**

---

## 📡 API Endpoints

Base URL: `http://localhost:8000`

### Format Response

Semua endpoint mengembalikan response dengan format standar:

```json
{
  "success": true,
  "message": "Deskripsi hasil",
  "code": 200,
  "data": [ ... ]
}
```

---

### 1. Get All Komik

Mengambil data komik yang sudah tersimpan di database.

```
GET /v1/get_all_komik
```

**Query Parameters** (semuanya opsional)

| Parameter | Deskripsi                                        |
| --------- | ------------------------------------------------ |
| `q`       | Cari berdasarkan judul                           |
| `status`  | Filter `Berjalan` atau `Tamat`                   |
| `page`    | Nomor halaman, mulai dari 1                      |
| `limit`   | Jumlah per halaman, default 20, maksimal 100     |

> Tanpa `page` dan `limit`, endpoint ini mengembalikan **seluruh data** seperti sebelumnya. Kalau salah satunya dikirim, response akan membawa blok `meta` berisi info paginasi.

**Response** `200 OK`

```json
{
  "success": true,
  "message": "Berhasil Mengambil Data",
  "code": 200,
  "data": [
    {
      "title": "One Piece",
      "imgurl": "https://example.com/cover.jpg",
      "slug": "one-piece",
      "description": "Kisah petualangan Monkey D. Luffy...",
      "status": "Berjalan"
    }
  ]
}
```

---

### 2. Komik Populer

Mengambil daftar komik populer secara **real-time** dari slider halaman utama Komikindo (tidak dari database).

```
GET /v1/populer_komik
```

**Response** `200 OK`

```json
{
  "success": true,
  "message": "Berhasil mengambil data komik",
  "code": 200,
  "data": [
    {
      "title": "Solo Leveling",
      "imgurl": "https://example.com/cover.jpg",
      "slug": "solo-leveling"
    }
  ]
}
```

**Response** `408 Request Timeout` — Jika gagal scraping dari website.

---

### 3. Search Komik

Mencari komik berdasarkan judul secara **real-time** dari website Komikindo.

```
GET /v1/search_komik?komik={keyword}
```

| Parameter | Tipe   | Wajib | Deskripsi                   |
| --------- | ------ | ----- | --------------------------- |
| `komik`   | string | Ya    | Keyword pencarian judul     |

**Contoh Request**

```
GET /v1/search_komik?komik=naruto
```

**Response** `200 OK`

```json
{
  "success": true,
  "message": "Komik ditemukan",
  "code": 200,
  "data": [
    {
      "title": "Naruto",
      "imgurl": "https://example.com/cover.jpg",
      "slug": "naruto"
    }
  ]
}
```

**Response** `404 Not Found` — Jika komik tidak ditemukan.

---

### 4. Get All Chapters

Mengambil semua chapter dari suatu komik. Pertama cek database, jika belum ada maka scraping dari website lalu disimpan ke database.

```
GET /v1/get_all_chapter/:slug
```

| Parameter | Tipe   | Wajib | Deskripsi              |
| --------- | ------ | ----- | ---------------------- |
| `slug`    | path   | Ya    | Slug unik dari komik   |

**Contoh Request**

```
GET /v1/get_all_chapter/one-piece
```

**Response** `200 OK`

```json
{
  "success": true,
  "message": "Berhasil mengambil data chapter",
  "code": 200,
  "data": {
    "title": "One Piece",
    "imgurl": "https://example.com/cover.jpg",
    "slug": "one-piece",
    "description": "Kisah petualangan...",
    "status": "Berjalan",
    "KomikChapter": [
      {
        "title": "One Piece Chapter 1100",
        "slugchapter": "one-piece-chapter-1100"
      }
    ]
  }
}
```

**Response** `204 No Content` — Jika chapter tidak ditemukan.

---

### 5. Get Panel Komik

Mengambil semua gambar panel dari suatu chapter. Pertama cek database, jika belum ada maka scraping dari website lalu disimpan ke database.

```
GET /v1/get_panel_komik/:chapter
```

| Parameter | Tipe   | Wajib | Deskripsi                      |
| --------- | ------ | ----- | ------------------------------ |
| `chapter` | path   | Ya    | Slug chapter (dari endpoint 4) |

**Contoh Request**

```
GET /v1/get_panel_komik/one-piece-chapter-1100
```

**Response** `200 OK`

```json
{
  "success": true,
  "message": "Berhasil mengambil data panel",
  "code": 200,
  "data": [
    {
      "panelnumber": 1,
      "imgurl": "https://example.com/panel-1.jpg"
    },
    {
      "panelnumber": 2,
      "imgurl": "https://example.com/panel-2.jpg"
    }
  ]
}
```

---

## ⚠️ Rate Limiting

API menggunakan rate limiter per IP address, dengan batas terpisah untuk endpoint autentikasi:

| Setting            | Endpoint `/v1/**`    | Endpoint `/v1/auth/**` |
| ------------------ | -------------------- | ---------------------- |
| Rate               | 2 request/detik      | 10 request/menit       |
| Burst              | 5 request            | 5 request              |
| IP Cleanup         | Setiap 1 menit       | Setiap 1 menit         |
| IP Expiry          | 3 menit tidak aktif  | 3 menit tidak aktif    |

Selain itu ada penguncian akun setelah 5x gagal login — lihat [AUTHENTICATION.md](AUTHENTICATION.md).

Jika melebihi limit:

```json
{
  "success": false,
  "message": "Too many requests.",
  "code": 429,
  "data": null
}
```

**HTTP Status:** `429 Too Many Requests`

---

## 🛠️ Tech Stack

| Teknologi                                                  | Kegunaan                     |
| ---------------------------------------------------------- | ---------------------------- |
| [Go](https://go.dev/)                                      | Bahasa pemrograman utama     |
| [Gin](https://github.com/gin-gonic/gin)                    | HTTP web framework           |
| [Colly](https://github.com/gocolly/colly)                  | Web scraping framework       |
| [GORM](https://gorm.io/)                                   | ORM untuk database           |
| [MySQL](https://www.mysql.com/)                             | Database                     |
| [godotenv](https://github.com/joho/godotenv)               | Load .env file               |
| [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) | Token bucket rate limiter |

---

## 📊 Database Schema

GORM akan otomatis membuat tabel berikut:

### Tabel `komiks`

| Kolom         | Tipe     | Deskripsi                  |
| ------------- | -------- | -------------------------- |
| id            | uint     | Primary key (auto)         |
| created_at    | datetime | Waktu dibuat               |
| updated_at    | datetime | Waktu diupdate             |
| deleted_at    | datetime | Soft delete                |
| title         | string   | Judul komik                |
| img_url       | string   | URL gambar cover           |
| slug          | string   | Slug unik komik            |
| description   | string   | Sinopsis komik             |
| status        | string   | "Berjalan" atau "Tamat"    |

### Tabel `komik_chapters`

| Kolom         | Tipe     | Deskripsi                          |
| ------------- | -------- | ---------------------------------- |
| id            | uint     | Primary key (auto)                 |
| created_at    | datetime | Waktu dibuat                       |
| updated_at    | datetime | Waktu diupdate                     |
| deleted_at    | datetime | Soft delete                        |
| title         | string   | Judul chapter                      |
| slug_chapter  | string   | Slug unik chapter (unique index)   |
| komik_id      | string   | Foreign key ke tabel komiks        |

### Tabel `komik_panels`

| Kolom         | Tipe     | Deskripsi                  |
| ------------- | -------- | -------------------------- |
| id            | uint     | Primary key (auto)         |
| created_at    | datetime | Waktu dibuat               |
| updated_at    | datetime | Waktu diupdate             |
| deleted_at    | datetime | Soft delete                |
| slug_chapter  | string   | Referensi ke chapter       |
| panel_number  | int      | Urutan panel               |
| img_url       | string   | URL gambar panel           |

---

## 📜 Contoh Penggunaan (cURL)

```bash
KEY="your-api-key"

# 1. Ambil semua komik
curl -H "X-API-Key: $KEY" http://localhost:8000/v1/get_all_komik

# 1b. Dengan filter dan paginasi
curl -H "X-API-Key: $KEY" "http://localhost:8000/v1/get_all_komik?q=piece&status=Berjalan&page=1&limit=10"

# 2. Ambil komik populer
curl -H "X-API-Key: $KEY" http://localhost:8000/v1/populer_komik

# 3. Cari komik
curl -H "X-API-Key: $KEY" "http://localhost:8000/v1/search_komik?komik=one+piece"

# 4. Ambil semua chapter
curl -H "X-API-Key: $KEY" http://localhost:8000/v1/get_all_chapter/one-piece

# 5. Ambil panel chapter
curl -H "X-API-Key: $KEY" http://localhost:8000/v1/get_panel_komik/one-piece-chapter-1100

# 6. Health check (tanpa API key)
curl http://localhost:8000/health
```

Contoh alur user (selengkapnya di [AUTHENTICATION.md](AUTHENTICATION.md)):

```bash
KEY="your-api-key"

# Login, ambil access token
TOKEN=$(curl -s -X POST http://localhost:8000/v1/auth/login \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"identifier":"bilal","password":"rahasia123"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['access_token'])")

# Simpan komik ke bookmark
curl -X POST http://localhost:8000/v1/bookmarks \
  -H "X-API-Key: $KEY" -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"komik_slug":"one-piece"}'

# Lihat bookmark
curl -H "X-API-Key: $KEY" -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/v1/bookmarks
```

---

## ⚖️ Disclaimer

> Project ini dibuat **hanya untuk tujuan edukasi**. Semua konten komik adalah milik pencipta dan penerbit aslinya. Gunakan API ini secara bertanggung jawab dan sesuai dengan hukum yang berlaku.

---

## 📄 Lisensi

Project ini bersifat open-source. Silakan gunakan dan modifikasi sesuai kebutuhan.