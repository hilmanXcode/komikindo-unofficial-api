package bootstrap

import (
	"komikindo-scraper/config"
	"komikindo-scraper/helpers"
	"komikindo-scraper/middleware"
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/routes"
	"komikindo-scraper/scraper"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

func BootstrapApp() {

	gin.SetMode(gin.ReleaseMode)

	if providerIsOk := helpers.CheckKomikindoConnection(); !providerIsOk {
		log.Fatal("Koneksi ke komikindo gagal")
		return
	}

	config.LoadEnvVariables()

	db := config.InitDatabase()

	scraperKomikindo := scraper.NewScraperKomikindo(db)

	app := gin.Default()

	var dataKomik []model_komik.Komik

	go func() {
		ticker := time.NewTicker(12 * time.Hour)
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
	}()

	middleware.CleanupIps()

	routes.InitRoute(app, db)

}
