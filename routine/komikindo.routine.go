package routine

import (
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/scraper"
	"log"
	"time"

	"gorm.io/gorm"
)

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

		var dataKomik []model_komik.Komik

		result := db.Preload("KomikChapter").Where("status = 'Berjalan'").Find(&dataKomik)

		// Database yang sedang bermasalah cukup membuat putaran ini dilewati,
		// bukan menghentikan API. Putaran berikutnya mencoba lagi.
		if result.Error != nil {
			log.Println("Gagal mendapatkan data komik saat scraping:", result.Error)
			continue
		}

		for _, v := range dataKomik {
			scraperKomikindo.ScrapeChapterKomik(v)
		}

	}

}
