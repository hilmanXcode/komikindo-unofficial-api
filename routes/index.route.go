package routes

import (
	"komikindo-scraper/controllers"

	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly/v2"
)

func InitRoute(app *gin.Engine) {

	colly := colly.NewCollector()

	komikindoController := controllers.NewKomikController(colly)

	route := app

	route.GET("/v1/populer_komik", komikindoController.GetAllPopulerKomik)

	route.GET("/v1/search_komik", komikindoController.SearchKomik)

	route.GET("/v1/get_all_chapter/:slug", komikindoController.GetAllChaptersKomik)

	route.GET("/v1/get_panel_komik/:chapter", komikindoController.GetPanelKomik)

	app.Run(":8000")
}
