#!/usr/bin/env bash
# Backup harian database dari container MySQL ke folder backups/, menyimpan
# 7 backup terakhir. Dijalankan lewat cron di server, lihat DEPLOY.md.
# pipefail: kalau mysqldump gagal, jangan simpan file backup kosong.
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p backups

file="backups/db_komik-$(date +%F-%H%M).sql.gz"

docker compose exec -T db sh -c 'exec mysqldump --single-transaction -uroot -p"$MYSQL_ROOT_PASSWORD" db_komik' | gzip > "$file.tmp"
mv "$file.tmp" "$file"

find backups -name 'db_komik-*.sql.gz' -mtime +7 -delete

echo "Backup tersimpan: $file"
