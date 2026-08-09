package bootstrap

import (
	"komikindo-scraper/config"
	"komikindo-scraper/helpers"
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/routes"
	"komikindo-scraper/routine"
	"komikindo-scraper/scraper"
	"log"

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

	routine.KomikindoRoutine(dataKomik, scraperKomikindo, db)

	routes.InitRoute(app, db)

}
