# Deploy dengan Docker

API dan MySQL berjalan sebagai dua container lewat `docker compose`. Keduanya
otomatis hidup lagi kalau crash atau server reboot.

Semua perintah di bawah dijalankan di server (Linux), sebagai user yang boleh
menjalankan `docker`.

## 1. Pasang Docker (sekali saja)

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER     # lalu logout dan login lagi
docker compose version            # pastikan perintah ini jalan
```

## 2. Ambil kode dan isi konfigurasi

```bash
sudo mkdir -p /opt/komikindo-api && sudo chown $USER /opt/komikindo-api
git clone https://github.com/hilmanXcode/komikindo-unofficial-api.git /opt/komikindo-api
cd /opt/komikindo-api
cp .env.example .env
openssl rand -hex 24   # jalankan dua kali: untuk DB_PASSWORD dan DB_ROOT_PASSWORD
nano .env
```

Yang wajib diisi di `.env`:

| Variabel | Isi |
|---|---|
| `SECRET_SELF_API_KEY` | Sama dengan `API_KEY` frontend di Vercel |
| `JWT_SECRET` | Pakai nilai lama supaya user tidak perlu login ulang |
| `DB_PASSWORD`, `DB_ROOT_PASSWORD` | Hasil `openssl rand -hex 24` |
| `CORS_ORIGINS` | Boleh dikosongkan (pakai default) |

`DATABASE_DSN` tidak perlu diubah, karena compose membentuknya sendiri.

## 3. Pindahkan data dari database lama

Lewati langkah ini kalau mulai dari database kosong.

```bash
# a. Dump database lama (sesuaikan host, user, dan nama database)
mysqldump --single-transaction -h HOST_LAMA -u USER_LAMA -p NAMA_DB_LAMA > dump-lama.sql

# b. Nyalakan MySQL saja, tunggu sampai kolom STATUS "healthy"
docker compose up -d db
docker compose ps

# c. Import dump
docker compose exec -T db sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" db_komik' < dump-lama.sql

# d. Migrasi Tahap 1 (aman dijalankan ulang kalau sudah pernah)
docker compose exec -T db sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" db_komik' < migrations/2026-09-27-tahap1.sql
```

## 4. Jalankan

Matikan dulu backend lama (proses `go run`, screen, pm2, systemd, dll.) supaya
port 8000 bebas, lalu:

```bash
docker compose up -d --build
docker compose logs -f api        # Ctrl+C untuk keluar
```

Log yang benar berisi `Berhasil terkoneksi dengan database` dan
`Server berjalan di port 8000`. Cek dari server:

```bash
curl http://127.0.0.1:8000/health
```

## 5. Buka ke internet

API hanya mendengarkan di `127.0.0.1:8000`, jadi perlu reverse proxy di depannya.
Kalau sebelumnya sudah ada Nginx/Caddy/Cloudflare Tunnel yang mengarah ke port
8000, tidak ada yang perlu diubah. Kalau belum, cara paling singkat dengan Caddy
(HTTPS otomatis):

```bash
sudo apt install -y caddy
echo 'mangaapi.hilmanxcode.my.id {
    reverse_proxy 127.0.0.1:8000
}' | sudo tee /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## 6. Backup harian

```bash
chmod +x scripts/backup.sh
./scripts/backup.sh               # coba sekali, hasilnya di folder backups/
crontab -e
```

Tambahkan baris ini (backup tiap jam 03.00, menyimpan 7 hari terakhir):

```
0 3 * * * /opt/komikindo-api/scripts/backup.sh >> /opt/komikindo-api/backups/backup.log 2>&1
```

Sesekali salin isi `backups/` ke luar server. Backup yang hanya ada di server
yang sama ikut hilang kalau servernya rusak.

## Perintah sehari-hari

```bash
cd /opt/komikindo-api

git pull && docker compose up -d --build     # update ke versi terbaru
docker compose logs -f api                   # lihat log
docker compose ps                            # status container
docker compose restart api                   # restart manual

# Pulihkan dari backup
gunzip -c backups/NAMA_FILE.sql.gz | docker compose exec -T db sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" db_komik'
```

Jangan jalankan `docker compose down -v`: opsi `-v` menghapus volume database.
