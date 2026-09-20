# 🔐 Autentikasi & Fitur User

Dokumentasi fitur autentikasi user (JWT), bookmark, riwayat baca, manajemen admin, dan health check yang ditambahkan ke Komikindo Unofficial API.

> Untuk dokumentasi endpoint komik (search, chapter, panel), lihat [README.md](README.md).

---

## 📑 Daftar Isi

- [Konsep Dua Lapis Keamanan](#-konsep-dua-lapis-keamanan)
- [Setup](#-setup)
- [Alur Autentikasi](#-alur-autentikasi)
- [Endpoint Autentikasi](#-endpoint-autentikasi)
- [Endpoint Bookmark](#-endpoint-bookmark)
- [Endpoint Riwayat Baca](#-endpoint-riwayat-baca)
- [Endpoint Admin](#-endpoint-admin)
- [Health Check](#-health-check)
- [Paginasi & Filter](#-paginasi--filter)
- [Keputusan Keamanan](#-keputusan-keamanan)
- [Skema Database Baru](#-skema-database-baru)
- [Daftar Kode Error](#-daftar-kode-error)
- [Catatan Migrasi](#-catatan-migrasi)

---

## 🧱 Konsep Dua Lapis Keamanan

API ini sekarang punya **dua lapis** yang fungsinya berbeda dan tidak saling menggantikan:

| Lapis | Header | Menjawab pertanyaan | Berlaku di |
|-------|--------|---------------------|------------|
| **API Key** | `X-API-Key` | *Aplikasi mana yang memanggil?* | Semua endpoint `/v1/**` |
| **Access Token (JWT)** | `Authorization: Bearer <token>` | *User mana yang sedang login?* | Endpoint milik user & admin |

Artinya endpoint seperti `/v1/bookmarks` butuh **dua header sekaligus**:

```http
GET /v1/bookmarks HTTP/1.1
X-API-Key: apikey-aplikasi-kamu
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### Kenapa API key pindah ke `X-API-Key`?

Sebelumnya API key dikirim lewat header `Authorization`. Karena sekarang `Authorization` dipakai untuk access token user, API key dipindah ke header sendiri.

**Klien lama tetap jalan.** Middleware masih menerima API key di header `Authorization` selama isinya *bukan* diawali `Bearer `. Jadi kode lama seperti ini tidak perlu langsung diubah:

```js
// masih berfungsi (cara lama, deprecated)
fetch(url, { headers: { Authorization: API_KEY } })

// cara baru yang disarankan
fetch(url, { headers: { 'X-API-Key': API_KEY } })
```

---

## ⚙️ Setup

### 1. Tambahkan variabel environment baru

Salin dari `.env.example` ke `.env`:

```env
# ----- Database -----
DATABASE_DSN="username:password@tcp(127.0.0.1:3306)/db_komik?parseTime=true"

# ----- API key aplikasi -----
SECRET_SELF_API_KEY="isibebassih"

# ----- Autentikasi user (JWT) -----
JWT_SECRET=""                # WAJIB, minimal 32 karakter
ACCESS_TOKEN_TTL="15m"       # default 15m
REFRESH_TOKEN_TTL="720h"     # default 720h (30 hari)
MAX_LOGIN_ATTEMPTS="5"       # default 5
LOGIN_LOCK_DURATION="15m"    # default 15m

# ----- Server -----
PORT="8000"                  # default 8000
```

### 2. Generate `JWT_SECRET`

```bash
openssl rand -hex 32
```

Aplikasi **menolak start** kalau `JWT_SECRET` kosong atau kurang dari 32 karakter — ini disengaja, supaya tidak ada deployment yang tanpa sadar jalan dengan secret lemah.

### 3. Jalankan

```bash
./start.sh
```

Tabel baru (`users`, `refresh_tokens`, `bookmarks`, `reading_histories`) dibuat otomatis lewat GORM AutoMigrate. **Tidak ada perubahan pada tabel komik yang sudah ada.**

### 4. Membuat akun admin pertama

Belum ada endpoint khusus untuk ini (disengaja — endpoint yang bisa membuat admin adalah lubang keamanan). Daftar seperti user biasa lalu promosikan lewat SQL:

```sql
UPDATE users SET role = 'admin' WHERE username = 'namakamu';
```

---

## 🔄 Alur Autentikasi

```
┌──────────┐                                      ┌──────────┐
│  Klien   │                                      │   API    │
└────┬─────┘                                      └────┬─────┘
     │  POST /v1/auth/register atau /login             │
     │  { identifier, password }                       │
     ├────────────────────────────────────────────────▶│
     │                                                 │ cek bcrypt
     │  { access_token (15m), refresh_token (30h) }    │ simpan hash
     │◀────────────────────────────────────────────────┤ refresh token
     │                                                 │
     │  Request biasa                                  │
     │  Authorization: Bearer <access_token>           │
     ├────────────────────────────────────────────────▶│
     │                                                 │
     │  ... 15 menit kemudian: 401 token kedaluwarsa   │
     │                                                 │
     │  POST /v1/auth/refresh                          │
     │  { refresh_token }                              │
     ├────────────────────────────────────────────────▶│ token lama
     │                                                 │ DICABUT
     │  { access_token baru, refresh_token BARU }      │ (rotasi)
     │◀────────────────────────────────────────────────┤
```

**Dua jenis token, dua tugas berbeda:**

- **Access token** — JWT bertanda tangan HS256, umur pendek (15 menit). Dikirim di setiap request. Tidak bisa dicabut, makanya umurnya pendek.
- **Refresh token** — string acak 256-bit yang tidak membawa informasi apa pun. Umur panjang (30 hari), disimpan di database sebagai hash SHA-256, dan **bisa dicabut kapan saja**.

---

## 🔑 Endpoint Autentikasi

Base path: `/v1/auth`
Rate limit khusus: **10 request/menit per IP** (burst 5) — lebih ketat dari endpoint lain.

### 1. Register

`POST /v1/auth/register`

**Header:** `X-API-Key`, `Content-Type: application/json`

**Body:**

| Field | Tipe | Aturan |
|-------|------|--------|
| `username` | string | wajib, 3–30 karakter, hanya huruf & angka |
| `email` | string | wajib, format email valid, maks 120 karakter |
| `password` | string | wajib, 8–72 karakter |

```bash
curl -X POST http://localhost:8000/v1/auth/register \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"username":"bilal","email":"bilal@example.com","password":"rahasia123"}'
```

**Response `201`:**

```json
{
  "success": true,
  "message": "Registrasi berhasil",
  "code": 201,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "58aa6f4fed5ef14ae5250d9faf10d8ec...",
    "token_type": "Bearer",
    "expires_in": 900,
    "expires_at": "2026-09-18T23:45:14+07:00",
    "user": {
      "ID": 1,
      "username": "bilal",
      "email": "bilal@example.com",
      "role": "user",
      "is_active": true
    }
  }
}
```

> Username dan email otomatis di-*lowercase* dan di-*trim*, jadi `Bilal@Example.com` dan `bilal@example.com` dianggap akun yang sama.

**Error:** `400` input tidak valid · `409` username/email sudah terdaftar

---

### 2. Login

`POST /v1/auth/login`

**Body:**

| Field | Tipe | Keterangan |
|-------|------|------------|
| `identifier` | string | wajib — bisa diisi **username atau email** |
| `password` | string | wajib |

```bash
curl -X POST http://localhost:8000/v1/auth/login \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"identifier":"bilal","password":"rahasia123"}'
```

**Response `200`:** sama persis dengan register.

**Error:**

| Kode | Arti |
|------|------|
| `401` | Username/email atau password salah |
| `403` | Akun dinonaktifkan admin |
| `429` | Akun dikunci sementara (5x gagal login) |

---

### 3. Refresh Token

`POST /v1/auth/refresh`

Menukar refresh token dengan pasangan token baru. **Tidak butuh** header `Authorization`.

```bash
curl -X POST http://localhost:8000/v1/auth/refresh \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"58aa6f4fed5ef14ae5250d9faf10d8ec..."}'
```

**Response `200`:** pasangan token baru — perhatikan bahwa **`refresh_token` juga ikut berganti**.

> ⚠️ **Rotasi + deteksi kebocoran.** Satu refresh token hanya bisa dipakai **sekali**. Setelah dipakai, token itu langsung dicabut. Kalau ada yang mencoba memakainya lagi, sistem menganggap token sudah bocor dan **mencabut seluruh sesi user tersebut** — termasuk token hasil rotasi yang masih dipegang user asli. Efeknya: kalau ada penyerang yang mencuri refresh token, salah satu pihak pasti kena tendang dan user tahu ada yang tidak beres.
>
> Simpan selalu refresh token terbaru dari response. Jangan pernah memakai ulang yang lama.

**Error:** `401` token tidak valid / kedaluwarsa / sudah dicabut

---

### 4. Logout

`POST /v1/auth/logout`

Mencabut **satu** refresh token — hanya perangkat ini yang keluar.

```bash
curl -X POST http://localhost:8000/v1/auth/logout \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"58aa6f4f..."}'
```

**Response `200`** selalu sukses, bahkan kalau token tidak ditemukan — klien tidak perlu tahu bedanya, dan hasil akhirnya sama.

---

### 5. Profil Saya

`GET /v1/auth/me` — **butuh access token**

```bash
curl http://localhost:8000/v1/auth/me \
  -H "X-API-Key: $API_KEY" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

```json
{
  "success": true,
  "message": "Berhasil mengambil profil",
  "code": 200,
  "data": {
    "id": 1,
    "username": "bilal",
    "email": "bilal@example.com",
    "role": "user",
    "is_active": true,
    "last_login_at": "2026-09-18T16:30:29Z",
    "created_at": "2026-09-18T16:30:14Z",
    "total_bookmark": 12,
    "total_riwayat": 34
  }
}
```

---

### 6. Daftar Sesi Aktif

`GET /v1/auth/sessions` — **butuh access token**

Menampilkan perangkat yang masih login (refresh token aktif), lengkap dengan User-Agent dan IP saat login.

```json
{
  "success": true,
  "message": "Berhasil mengambil daftar sesi aktif",
  "code": 200,
  "data": [
    {
      "ID": 7,
      "user_id": 1,
      "expires_at": "2026-10-18T16:30:29Z",
      "revoked_at": null,
      "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)...",
      "ip": "103.12.44.8"
    }
  ]
}
```

---

### 7. Logout Semua Perangkat

`POST /v1/auth/logout-all` — **butuh access token**

Mencabut semua refresh token milik user. Berguna kalau user merasa akunnya dipakai orang lain.

> Access token yang sudah terlanjur diterbitkan tetap berlaku sampai kedaluwarsa (maksimal 15 menit). Untuk menendang seseorang **seketika**, nonaktifkan akunnya lewat endpoint admin.

---

### 8. Ganti Password

`PATCH /v1/auth/change-password` — **butuh access token**

| Field | Aturan |
|-------|--------|
| `old_password` | wajib |
| `new_password` | wajib, 8–72 karakter, harus berbeda dari yang lama |

```bash
curl -X PATCH http://localhost:8000/v1/auth/change-password \
  -H "X-API-Key: $API_KEY" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"old_password":"rahasia123","new_password":"rahasiabaru456"}'
```

Setelah berhasil, **semua sesi dicabut** dan user harus login ulang di semua perangkat.

**Error:** `401` password lama salah · `400` password baru sama dengan yang lama

---

## 🔖 Endpoint Bookmark

Komik yang disimpan user. Semua endpoint **butuh access token**.

### Daftar Bookmark

`GET /v1/bookmarks?page=1&limit=20`

Diurutkan dari yang paling baru disimpan.

```json
{
  "success": true,
  "message": "Berhasil mengambil data bookmark",
  "code": 200,
  "data": [
    {
      "ID": 1,
      "user_id": 1,
      "komik_slug": "one-piece",
      "title": "One Piece",
      "imgurl": "https://komikindo.ch/img/op.jpg",
      "CreatedAt": "2026-09-18T16:30:44Z"
    }
  ],
  "meta": { "page": 1, "limit": 20, "total": 1, "total_pages": 1 }
}
```

### Tambah Bookmark

`POST /v1/bookmarks`

| Field | Aturan |
|-------|--------|
| `komik_slug` | wajib, maks 200 karakter |
| `title` | opsional |
| `imgurl` | opsional |

```bash
curl -X POST http://localhost:8000/v1/bookmarks \
  -H "X-API-Key: $API_KEY" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"komik_slug":"one-piece"}'
```

> `title` dan `imgurl` diisi **otomatis** dari tabel komik kalau slug-nya sudah pernah di-scrape. Keduanya sengaja di-denormalisasi supaya daftar bookmark bisa ditampilkan tanpa join.

Bookmark komik yang sama dua kali **bukan error** — operasinya idempoten, baris lama tetap dipakai.

### Hapus Bookmark

`DELETE /v1/bookmarks/:slug`

```bash
curl -X DELETE http://localhost:8000/v1/bookmarks/one-piece \
  -H "X-API-Key: $API_KEY" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Error:** `404` bookmark tidak ditemukan

---

## 📖 Endpoint Riwayat Baca

Menyimpan chapter terakhir yang dibaca per komik — untuk fitur "lanjutkan membaca". Semua endpoint **butuh access token**.

### Daftar Riwayat

`GET /v1/history?page=1&limit=20`

Diurutkan dari yang terakhir dibaca.

### Simpan Riwayat

`POST /v1/history`

| Field | Aturan |
|-------|--------|
| `komik_slug` | wajib, maks 200 karakter |
| `chapter_slug` | wajib, maks 200 karakter |
| `chapter_title` | opsional |
| `last_panel` | opsional, integer ≥ 0 — posisi panel terakhir untuk resume scroll |

```bash
curl -X POST http://localhost:8000/v1/history \
  -H "X-API-Key: $API_KEY" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "komik_slug": "one-piece",
    "chapter_slug": "one-piece-chapter-1100",
    "chapter_title": "One Piece Chapter 1100",
    "last_panel": 5
  }'
```

> **Satu komik = satu baris riwayat.** Endpoint ini bersifat *upsert*: memanggilnya lagi untuk komik yang sama akan menimpa baris yang ada, bukan menambah baris baru. Jadi aman dipanggil berkali-kali saat user scroll — tabelnya tidak akan membengkak.

### Hapus Riwayat

`DELETE /v1/history/:slug`

---

## 👮 Endpoint Admin

Butuh access token **dan** `role = "admin"`. User biasa dapat `403`.

### Daftar User

`GET /v1/admin/users?q=bilal&page=1&limit=20`

| Query | Keterangan |
|-------|------------|
| `q` | cari di username atau email |
| `page`, `limit` | paginasi |

Field `password` tidak pernah ikut dalam response — sudah dikecualikan di level model.

### Ubah Role / Status User

`PATCH /v1/admin/users/:id`

| Field | Nilai |
|-------|-------|
| `role` | `"user"` atau `"admin"` |
| `is_active` | `true` atau `false` |

```bash
# Nonaktifkan akun
curl -X PATCH http://localhost:8000/v1/admin/users/2 \
  -H "X-API-Key: $API_KEY" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"is_active": false}'

# Jadikan admin
curl -X PATCH http://localhost:8000/v1/admin/users/2 \
  -H "X-API-Key: $API_KEY" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role": "admin"}'
```

Setelah perubahan, semua sesi user tersebut otomatis dicabut. Akun yang dinonaktifkan langsung tertolak (`403`) pada request berikutnya, tanpa menunggu token kedaluwarsa.

> **Admin tidak bisa mengubah akunnya sendiri** (`403`). Ini mencegah kejadian sistem kehilangan admin terakhirnya karena salah klik.

**Error:** `400` tidak ada field yang diubah · `403` mencoba mengubah akun sendiri · `404` user tidak ditemukan

---

## 🩺 Health Check

`GET /health` — **publik, tanpa API key**, supaya bisa dipakai uptime monitor.

```json
{
  "success": true,
  "message": "Status layanan",
  "code": 200,
  "data": {
    "database": "up",
    "provider": "up",
    "uptime": "3h12m40s",
    "time": "2026-09-18T23:30:14+07:00"
  }
}
```

Mengembalikan `503` kalau database tidak bisa di-ping.

---

## 📄 Paginasi & Filter

Endpoint yang mengembalikan list menerima query berikut:

| Query | Default | Keterangan |
|-------|---------|------------|
| `page` | `1` | halaman, mulai dari 1 |
| `limit` | `20` | jumlah per halaman, **maksimal 100** |

Response-nya membawa blok `meta` tambahan:

```json
"meta": { "page": 1, "limit": 20, "total": 153, "total_pages": 8 }
```

### `/v1/get_all_komik` juga ikut ditingkatkan

| Query | Keterangan |
|-------|------------|
| `q` | cari berdasarkan judul |
| `status` | filter `Berjalan` atau `Tamat` |
| `page`, `limit` | paginasi |

```bash
curl "http://localhost:8000/v1/get_all_komik?q=piece&status=Berjalan&page=1&limit=10" \
  -H "X-API-Key: $API_KEY"
```

> ✅ **Tidak breaking.** Kalau `page` dan `limit` sama-sama tidak dikirim, endpoint ini tetap mengembalikan **seluruh data tanpa blok `meta`**, persis seperti sebelumnya. Filter `q` dan `status` tetap bisa dipakai tanpa paginasi.

---

## 🛡️ Keputusan Keamanan

Beberapa hal yang sengaja dipilih, beserta alasannya:

### Password disimpan sebagai hash bcrypt

Bukan SHA-256 atau MD5. Bcrypt sengaja lambat, jadi kalau database bocor, menebak password dengan brute force jadi mahal. Field `Password` diberi tag `json:"-"` sehingga **mustahil** bocor lewat response — bahkan kalau nanti ada endpoint baru yang tidak sengaja mengembalikan objek user mentah.

### Refresh token disimpan sebagai hash, bukan token mentah

Yang tersimpan di tabel `refresh_tokens` adalah SHA-256 dari token. Kalau database bocor, penyerang tidak bisa memakai isinya untuk login. Bcrypt tidak dipakai di sini karena token sudah acak 256-bit — tidak ada yang bisa ditebak, jadi tidak butuh fungsi yang lambat.

### Pesan error login sengaja disamakan

"Username/email atau password salah" dipakai baik untuk username yang tidak ada maupun password yang salah. Kalau pesannya dibedakan, endpoint login bisa dipakai untuk memetakan siapa saja yang punya akun di sistem ini.

### Akun dikunci setelah 5x gagal login

Rate limit per IP saja tidak cukup — penyerang bisa memakai banyak IP untuk menyerang satu akun. Penguncian dihitung **per akun**, berlaku 15 menit, dan penghitungnya direset setiap login sukses. Keduanya bisa diatur lewat `MAX_LOGIN_ATTEMPTS` dan `LOGIN_LOCK_DURATION`.

### User dimuat ulang dari database setiap request

Middleware `RequireAuth` tidak cukup percaya pada isi JWT — ia selalu mengambil user dari database. Biayanya satu query per request, imbalannya: akun yang dinonaktifkan atau diturunkan rolenya **langsung** kehilangan akses, tidak perlu menunggu tokennya kedaluwarsa.

### Rate limiter endpoint auth dipisah

Endpoint auth dibatasi 10 request/menit, jauh lebih ketat dari 2 request/detik untuk endpoint biasa.

> 🐛 Sebagai bagian dari perubahan ini ada **bug yang diperbaiki**: `RateLimiter()` dulu memakai satu `store` global, sehingga limiter kedua yang dipasang pada IP yang sama akan memakai konfigurasi limiter pertama dan batas ketatnya tidak pernah berlaku. Sekarang setiap pemanggilan `RateLimiter()` punya store sendiri.

### Perbandingan API key memakai waktu konstan

`subtle.ConstantTimeCompare` dipakai menggantikan `!=`, supaya lama waktu respons tidak membocorkan berapa karakter awal tebakan yang sudah benar.

### Pembersihan refresh token otomatis

Goroutine berjalan setiap 6 jam untuk menghapus token yang sudah kedaluwarsa atau sudah dicabut lebih dari 24 jam, supaya tabelnya tidak tumbuh selamanya.

---

## 🗄️ Skema Database Baru

Empat tabel baru. **Tabel komik yang sudah ada tidak berubah sama sekali.**

### Tabel `users`

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uint | Primary key |
| `username` | varchar(50) | Unik |
| `email` | varchar(120) | Unik |
| `password` | varchar(120) | Hash bcrypt, tidak pernah dikembalikan |
| `role` | varchar(20) | `user` (default) atau `admin` |
| `is_active` | bool | Default `true` |
| `failed_login_attempts` | int | Penghitung gagal login |
| `locked_until` | datetime | Kapan penguncian berakhir (nullable) |
| `last_login_at` | datetime | Login sukses terakhir (nullable) |
| `created_at` / `updated_at` / `deleted_at` | datetime | Dari `gorm.Model` |

### Tabel `refresh_tokens`

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uint | Primary key |
| `user_id` | uint | Index, pemilik token |
| `token_hash` | varchar(64) | **Unik** — SHA-256 dari token |
| `expires_at` | datetime | Index |
| `revoked_at` | datetime | Nullable; terisi = tidak berlaku lagi |
| `user_agent` | varchar(255) | Perangkat saat token dibuat |
| `ip` | varchar(64) | IP saat token dibuat |

### Tabel `bookmarks`

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uint | Primary key |
| `user_id` | uint | Unique index gabungan dengan `komik_slug` |
| `komik_slug` | varchar(200) | Unique index gabungan dengan `user_id` |
| `title` | varchar(255) | Denormalisasi dari tabel komik |
| `imgurl` | varchar(500) | Denormalisasi dari tabel komik |

### Tabel `reading_histories`

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | uint | Primary key |
| `user_id` | uint | Unique index gabungan dengan `komik_slug` |
| `komik_slug` | varchar(200) | Unique index gabungan dengan `user_id` |
| `chapter_slug` | varchar(200) | Chapter terakhir dibaca |
| `chapter_title` | varchar(255) | Judul chapter |
| `last_panel` | int | Posisi panel terakhir |
| `last_read_at` | datetime | Index, dipakai untuk pengurutan |

---

## 🚦 Daftar Kode Error

| Kode | Kapan muncul |
|------|--------------|
| `400` | Body JSON tidak valid atau gagal validasi |
| `401` | API key salah · access token hilang/tidak valid/kedaluwarsa · kredensial login salah · refresh token tidak berlaku |
| `403` | Akun dinonaktifkan · role tidak mencukupi · admin mengubah akun sendiri |
| `404` | Bookmark, riwayat, atau user tidak ditemukan |
| `409` | Username atau email sudah terdaftar |
| `429` | Rate limit terlampaui, atau akun dikunci karena gagal login berulang |
| `500` | Kegagalan database atau internal |
| `503` | Health check: database tidak bisa dihubungi |

Semua error memakai format response yang sama dengan endpoint lain:

```json
{ "success": false, "message": "...", "code": 401, "data": null }
```

---

## 📋 Catatan Migrasi

Untuk yang sudah memakai versi sebelumnya:

| Perubahan | Dampak |
|-----------|--------|
| `JWT_SECRET` wajib diisi di `.env` | ⚠️ **Aplikasi tidak akan start tanpa ini** |
| API key pindah ke header `X-API-Key` | ✅ Tidak breaking — header `Authorization` masih diterima |
| API key salah balasannya `401`, bukan `400` | ⚠️ Cek kalau klien kamu mencocokkan kode status. `400` dulu memang keliru untuk kasus ini |
| `/v1/get_all_komik` dapat paginasi | ✅ Tidak breaking — tanpa `page`/`limit`, perilakunya persis sama |
| Response bisa punya field `meta` | ✅ Tidak breaking — `omitempty`, hanya muncul saat paginasi aktif |
| Port bisa diatur lewat `PORT` | ✅ Default tetap `8000` |
| CORS menerima POST/PATCH/DELETE dan header `Content-Type`, `X-API-Key` | ✅ Diperlukan agar endpoint baru bisa dipanggil dari browser |
| Empat tabel baru dibuat otomatis | ✅ Tabel komik tidak tersentuh |

### Checklist upgrade

```bash
# 1. Ambil dependensi baru (JWT + bcrypt)
go mod download

# 2. Tambahkan JWT_SECRET ke .env
echo "JWT_SECRET=\"$(openssl rand -hex 32)\"" >> .env

# 3. Jalankan — AutoMigrate membuat tabel baru
./start.sh

# 4. Verifikasi
curl http://localhost:8000/health

# 5. Buat admin pertama setelah mendaftar
# mysql> UPDATE users SET role = 'admin' WHERE username = 'namakamu';
```

---

## 🧩 File Baru

```
komikindo-scraper/
├── controllers/
│   ├── auth.controller.go           # Register, login, refresh, logout, profil, ganti password
│   ├── user.controller.go           # Bookmark, riwayat baca, manajemen user (admin)
│   ├── health.controller.go         # Health check
│   └── pagination.go                # Parsing query paginasi & pembuatan meta
├── helpers/
│   ├── auth.go                      # bcrypt, token acak, hash SHA-256, perbandingan konstan
│   └── jwt.go                       # Pembuatan & validasi access token
├── middleware/
│   └── auth.go                      # RequireAuth, RequireRole, OptionalAuth
├── model/
│   └── user/
│       └── user.model.go            # User, RefreshToken, Bookmark, ReadingHistory
└── routine/
    └── auth.routine.go              # Pembersihan refresh token kedaluwarsa
```

---

## 💡 Ide Lanjutan

Yang belum dikerjakan tapi masuk akal sebagai langkah berikutnya:

- **Verifikasi email** saat registrasi — butuh layanan pengirim email (SMTP/Resend/Mailgun)
- **Lupa password** lewat token reset sekali pakai — polanya sama persis dengan refresh token yang sudah ada di sini
- **API key per user**, supaya tiap klien punya kuota dan bisa dicabut sendiri-sendiri
- **Notifikasi chapter baru** untuk komik yang di-bookmark — datanya sudah tersedia, background routine-nya sudah jalan
- **Rekomendasi berdasarkan riwayat baca**
- **Login OAuth** (Google) sebagai alternatif password
