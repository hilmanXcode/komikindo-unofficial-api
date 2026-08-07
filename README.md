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
- 🔑 **API Key Auth** — Autentikasi via header `Authorization`

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
│   └── komikindo.controller.go      # Handler untuk semua endpoint API
├── helpers/
│   ├── response.go                  # Standar format response JSON
│   └── utils.go                     # Utility (cek koneksi provider)
├── middleware/
│   ├── apikey.go                    # Middleware autentikasi API Key
│   └── ratelimiter.go               # Middleware rate limiter per IP
├── model/
│   └── komik/
│       └── komik.model.go           # Model: Komik, KomikChapter, KomikPanel
├── routes/
│   └── index.route.go               # Definisi semua route API
├── scraper/
│   └── komikindo_scraper.go         # Background scraper untuk update chapter
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
   ```

   | Variable              | Deskripsi                                |
   | --------------------- | ---------------------------------------- |
   | `DATABASE_DSN`        | Connection string MySQL (format GORM)    |
   | `SECRET_SELF_API_KEY` | API key untuk autentikasi setiap request |

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

Semua endpoint memerlukan API Key yang dikirim melalui header `Authorization`.

```
Authorization: your-secret-api-key
```

Jika API key tidak valid atau tidak disertakan, API akan mengembalikan:

```json
{
  "success": false,
  "message": "Invalid API KEY",
  "code": 400,
  "data": null
}
```

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

Mengambil semua data komik yang sudah tersimpan di database.

```
GET /v1/get_all_komik
```

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

API menggunakan rate limiter per IP address:

| Setting            | Nilai                |
| ------------------ | -------------------- |
| Rate               | 2 request/detik      |
| Burst              | 5 request             |
| IP Cleanup         | Setiap 1 menit       |
| IP Expiry          | 3 menit tidak aktif   |

Jika melebihi limit:

```json
{
  "error": "Too many requests."
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
# 1. Ambil semua komik
curl -H "Authorization: your-api-key" http://localhost:8000/v1/get_all_komik

# 2. Ambil komik populer
curl -H "Authorization: your-api-key" http://localhost:8000/v1/populer_komik

# 3. Cari komik
curl -H "Authorization: your-api-key" "http://localhost:8000/v1/search_komik?komik=one+piece"

# 4. Ambil semua chapter
curl -H "Authorization: your-api-key" http://localhost:8000/v1/get_all_chapter/one-piece

# 5. Ambil panel chapter
curl -H "Authorization: your-api-key" http://localhost:8000/v1/get_panel_komik/one-piece-chapter-1100
```

---

## ⚖️ Disclaimer

> Project ini dibuat **hanya untuk tujuan edukasi**. Semua konten komik adalah milik pencipta dan penerbit aslinya. Gunakan API ini secara bertanggung jawab dan sesuai dengan hukum yang berlaku.

---

## 📄 Lisensi

Project ini bersifat open-source. Silakan gunakan dan modifikasi sesuai kebutuhan.