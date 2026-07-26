package bootstrap

import (
	"komikindo-scraper/routes"

	"github.com/gin-gonic/gin"
)

func BootstrapApp() {

	app := gin.Default()

	routes.InitRoute(app)

}
