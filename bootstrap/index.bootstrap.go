package bootstrap

import (
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

	app := gin.Default()

	routes.InitRoute(app)

}
