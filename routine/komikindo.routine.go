package routine

import (
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/scraper"
	"log"
	"time"

	"gorm.io/gorm"
)

func KomikindoRoutine(dataKomik []model_komik.Komik, scraperKomikindo *scraper.ScraperKomikindo, db *gorm.DB) {
	go scraperSavedKomik(dataKomik, scraperKomikindo, db)
}

func scraperSavedKomik(dataKomik []model_komik.Komik, scraperKomikindo *scraper.ScraperKomikindo, db *gorm.DB) {
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
