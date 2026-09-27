-- Jalankan SEKALI di database production SEBELUM deploy backend versi ini.
--
-- Backend versi ini membuat unique index pada komiks.slug dan
-- komik_panels(slug_chapter, panel_number), lalu berhenti start kalau migrasi
-- gagal. Data dobel harus dibersihkan dulu. Aman dijalankan ulang.
--
-- Backup dulu:
--   mysqldump -u <user> -p db_komik > backup-sebelum-tahap1.sql


-- 0. Pengecekan. Kedua query harus mengembalikan 0 baris; kalau tidak, kolom
--    akan terpotong saat diubah ke varchar(200). Kabari sebelum lanjut.
SELECT id, slug FROM komiks WHERE CHAR_LENGTH(slug) > 200;
SELECT id, slug_chapter FROM komik_panels WHERE CHAR_LENGTH(slug_chapter) > 200;


-- 1. Bookmark dan riwayat yang dulu dihapus (soft delete) masih memegang unique
--    index, sehingga komik yang sama tidak bisa disimpan lagi.
DELETE FROM bookmarks WHERE deleted_at IS NOT NULL;
DELETE FROM reading_histories WHERE deleted_at IS NOT NULL;


-- 2. Panel dobel: sisakan satu baris (id terkecil) per nomor panel per chapter.
DELETE FROM komik_panels
WHERE id NOT IN (
    SELECT id FROM (
        SELECT MIN(id) AS id FROM komik_panels GROUP BY slug_chapter, panel_number
    ) AS keep_rows
);


-- 3. Komik dobel: sisakan id terkecil per slug. Chapter milik baris dobel ikut
--    dihapus; komik yang disisakan sudah punya daftar chapter sendiri.
CREATE TEMPORARY TABLE komik_dobel AS
    SELECT id FROM komiks
    WHERE id NOT IN (
        SELECT id FROM (SELECT MIN(id) AS id FROM komiks GROUP BY slug) AS keep_rows
    );

DELETE FROM komik_chapters WHERE komik_id IN (SELECT id FROM komik_dobel);
DELETE FROM komiks WHERE id IN (SELECT id FROM komik_dobel);

DROP TEMPORARY TABLE komik_dobel;


-- 4. Slug chapter dulu dibuat dari judul, jadi judul seperti "HiGH & LOW"
--    menghasilkan slug yang 404 di provider. Routine sekarang menyamakan daftar
--    chapter dengan provider, tapi hanya untuk komik "Berjalan". Semua komik
--    dijadikan "Berjalan" supaya ikut diperiksa sekali saat backend start;
--    routine mengembalikan status "Tamat" dari halaman provider.
UPDATE komiks SET status = 'Berjalan';
