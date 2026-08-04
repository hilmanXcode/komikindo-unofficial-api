package middleware

import (
	"komikindo-scraper/config"
	"komikindo-scraper/helpers"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RequireApiKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")

		if token != config.API_KEY {
			c.JSON(
				http.StatusBadRequest,
				helpers.APIResponse(
					http.StatusBadRequest,
					false,
					"Invalid API KEY",
					nil,
				),
			)

			c.Abort()

			return
		}
		c.Next()
	}
}
