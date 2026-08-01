package bootstrap

import (
	"komikindo-scraper/config"
	"komikindo-scraper/routes"
	"log"
	"net"
	"time"

	"github.com/gin-gonic/gin"
)

func BootstrapApp() {

	timeout := 1 * time.Second
	_, err := net.DialTimeout("tcp", "komikindo.ch:8080", timeout)

	if err != nil {
		log.Fatal("Site unreachable, error: ", err)

	}

	config.LoadEnvVariables()

	db := config.InitDatabase()

	app := gin.Default()

	routes.InitRoute(app, db)

}
