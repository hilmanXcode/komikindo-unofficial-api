package bootstrap

import (
	"komikindo-scraper/config"
	"komikindo-scraper/helpers"
	"komikindo-scraper/routes"
	"komikindo-scraper/routine"
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

	app := gin.Default()

	routine.KomikindoRoutine(db)

	routes.InitRoute(app, db)

}
