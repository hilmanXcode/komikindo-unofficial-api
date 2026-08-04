package routes

import (
	"komikindo-scraper/controllers"
	"komikindo-scraper/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRoute(app *gin.Engine, db *gorm.DB) {

	komikindoController := controllers.NewKomikindoController(db)

	route := app

	route.Use(middleware.RateLimiter(2, 5))
	route.Use(middleware.RequireApiKey())
	route.Use(gin.Recovery())

	route.GET("/v1/populer_komik", komikindoController.GetAllPopulerKomik)

	route.GET("/v1/search_komik", komikindoController.SearchKomik)

	route.GET("/v1/get_all_chapter/:slug", komikindoController.GetAllChaptersKomik)

	route.GET("/v1/get_panel_komik/:chapter", komikindoController.GetPanelKomik)

	app.Run(":8000")
}
