package routes

import (
	komik_controller "komikindo-scraper/controllers/komik"

	"github.com/gin-gonic/gin"
)

func InitRoute(app *gin.Engine) {

	route := app

	route.GET("/v1/populer_komik", komik_controller.GetAllPopulerKomik)

	route.GET("/v1/search_komik", komik_controller.SearchKomik)

	route.GET("/v1/get_all_chapter/:slug", komik_controller.GetAllChaptersKomik)

	route.GET("/v1/get_panel_komik/:chapter", komik_controller.GetPanelKomik)

	app.Run(":8000")
}
