package config

import (
	model_komik "komikindo-scraper/model/komik"
	model_user "komikindo-scraper/model/user"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDatabase() *gorm.DB {
	// TranslateError supaya pelanggaran unique index bisa dikenali lewat
	// gorm.ErrDuplicatedKey, bukan kode error khusus MySQL.
	db, err := gorm.Open(mysql.Open(DSN), &gorm.Config{TranslateError: true})

	if err != nil {
		log.Fatal("Gagal terkoneksi ke database ", err)
	}

	log.Println("Berhasil terkoneksi dengan database")

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Gagal membuka pool database ", err)
	}

	// Koneksi dibuang sebelum MySQL menutupnya sendiri (wait_timeout), supaya
	// tidak muncul error "invalid connection" setelah server lama sepi.
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Skema yang gagal dimigrasi membuat query gagal diam-diam di tempat lain,
	// jadi lebih baik server tidak start sama sekali.
	err = db.AutoMigrate(
		&model_komik.Komik{},
		&model_komik.KomikChapter{},
		&model_komik.KomikPanel{},
		&model_user.User{},
		&model_user.RefreshToken{},
		&model_user.Bookmark{},
		&model_user.ReadingHistory{},
	)

	if err != nil {
		log.Fatal("Gagal migrasi database ", err)
	}

	return db
}
