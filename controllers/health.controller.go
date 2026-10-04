package controllers

import (
	"komikindo-scraper/helpers"
	"komikindo-scraper/scraper"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthController struct {
	db        *gorm.DB
	startedAt time.Time
}

func NewHealthController(db *gorm.DB) *HealthController {
	return &HealthController{
		db:        db,
		startedAt: time.Now(),
	}
}

// Health melaporkan status database dan provider. Endpoint ini publik (tanpa
// API key) supaya bisa dipakai uptime monitor.
func (controller *HealthController) Health(c *gin.Context) {

	dbStatus := "up"
	httpStatus := http.StatusOK

	sqlDB, err := controller.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		dbStatus = "down"
		httpStatus = http.StatusServiceUnavailable
	}

	// Frontend memanggil /health tiap menit untuk setiap pengunjung, jadi
	// koneksi ke provider cukup dicek sekali per menit.
	providerUp, _ := cached("health:provider", time.Minute, func() (bool, error) {
		return scraper.CheckConnection(), nil
	})

	providerStatus := "down"
	if providerUp {
		providerStatus = "up"
	}

	// Port provider yang terbuka belum berarti scraping jalan (mis. diblokir
	// Cloudflare atau HTML-nya berubah), jadi waktu scraping terakhir yang
	// berhasil ikut dilaporkan.
	var lastScrapeOK *time.Time
	if t := scraper.LastSuccess(); !t.IsZero() {
		lastScrapeOK = &t
	}

	c.JSON(httpStatus, helpers.APIResponse(
		httpStatus,
		httpStatus == http.StatusOK,
		"Status layanan",
		gin.H{
			"database":       dbStatus,
			"provider":       providerStatus,
			"providers":      scraper.ProviderNames(),
			"last_scrape_ok": lastScrapeOK,
			"uptime":         time.Since(controller.startedAt).Round(time.Second).String(),
			"time":           time.Now().Format(time.RFC3339),
		},
	))
}
