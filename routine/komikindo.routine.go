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
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	// Putaran pertama langsung jalan saat start, supaya server yang sering
	// restart tetap sempat memperbarui chapter.
	for ; ; <-ticker.C {

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

			// Jeda antar komik supaya provider tidak menganggap ini serangan.
			time.Sleep(2 * time.Second)
		}

	}

}
