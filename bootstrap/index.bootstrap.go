package bootstrap

import (
	"context"
	"errors"
	"komikindo-scraper/config"
	"komikindo-scraper/helpers"
	"komikindo-scraper/routes"
	"komikindo-scraper/routine"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Timeout mencegah koneksi lambat atau menggantung menghabiskan resource.
	// WriteTimeout dibuat longgar karena satu request bisa menunggu scraping
	// provider (timeout Colly 20 detik).
	server := &http.Server{
		Addr:              ":" + config.PORT,
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Docker mengirim SIGTERM saat container dihentikan atau diperbarui.
	// Request yang sedang berjalan diberi waktu selesai dulu.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Println("Server berjalan di port", config.PORT)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Server berhenti: ", err)
		}
	}()

	<-ctx.Done()
	log.Println("Mematikan server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Println("Server dimatikan paksa:", err)
	}
}
