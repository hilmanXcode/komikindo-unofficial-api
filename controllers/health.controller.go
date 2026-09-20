package controllers

import (
	"komikindo-scraper/helpers"
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

	providerStatus := "down"
	if helpers.CheckKomikindoConnection() {
		providerStatus = "up"
	}

	c.JSON(httpStatus, helpers.APIResponse(
		httpStatus,
		httpStatus == http.StatusOK,
		"Status layanan",
		gin.H{
			"database": dbStatus,
			"provider": providerStatus,
			"uptime":   time.Since(controller.startedAt).Round(time.Second).String(),
			"time":     time.Now().Format(time.RFC3339),
		},
	))
}
