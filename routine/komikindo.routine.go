package routine

import (
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/scraper"
	"log"
	"time"

	"gorm.io/gorm"
)

var dataKomik []model_komik.Komik

func KomikindoRoutine(db *gorm.DB) {
	// scraping kembali semua komik yang sudah ada di database dan status nya itu berjalan
	scraperKomikindo := scraper.NewScraperKomikindo(db)
	go scraperSavedKomik(db, scraperKomikindo)

	// other routine goes hereeeee
}

func scraperSavedKomik(db *gorm.DB, scraperKomikindo *scraper.ScraperKomikindo) {
	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	for range ticker.C {

		result := db.Preload("KomikChapter").Where("status = 'Berjalan'").Find(&dataKomik)

		if result.Error != nil {
			log.Fatal("Gagal mendapatkan data komik saat scraping")
			return
		}

		for _, v := range dataKomik {
			scraperKomikindo.ScrapeChapterKomik(v)
		}

	}

}
