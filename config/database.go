package config

import (
	model_komik "komikindo-scraper/model/komik"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDatabase() *gorm.DB {
	db, err := gorm.Open(mysql.Open(DSN), &gorm.Config{})

	if err != nil {
		log.Fatal("Gagal terkoneksi ke database ", err)
	}

	log.Println("Berhasil terkoneksi dengan database")

	db.AutoMigrate(&model_komik.Komik{}, &model_komik.KomikChapter{}, &model_komik.KomikPanel{})

	return db
}
