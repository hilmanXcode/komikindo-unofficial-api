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

	config.LoadEnvVariables()

	// Provider yang sedang down tidak boleh membuat API gagal start: data yang
	// sudah ada di database tetap bisa dilayani, dan /health yang melaporkannya.
	if providerIsOk := helpers.CheckKomikindoConnection(); !providerIsOk {
		log.Println("Peringatan: koneksi ke komikindo gagal, endpoint scraping tidak akan bekerja")
	}

	db := config.InitDatabase()

	app := gin.Default()

	// Rate limiter menghitung per IP, jadi X-Forwarded-For dari proxy di depan
	// aplikasi (frontend SSR, CDN) harus dipercaya supaya IP yang dipakai adalah
	// IP pengunjung, bukan IP proxy-nya. Kosong = percaya semua (default Gin).
	if len(config.TRUSTED_PROXIES) > 0 {
		if err := app.SetTrustedProxies(config.TRUSTED_PROXIES); err != nil {
			log.Fatal("TRUSTED_PROXIES tidak valid: ", err)
		}
	}

	routine.KomikindoRoutine(db)

	routine.AuthRoutine(db)

	routes.InitRoute(app, db)

}
