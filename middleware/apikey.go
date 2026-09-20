package middleware

import (
	"komikindo-scraper/config"
	"komikindo-scraper/helpers"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireApiKey memvalidasi API key aplikasi.
//
// Key dibaca dari header `X-API-Key`. Untuk kompatibilitas dengan klien lama,
// header `Authorization` masih diterima selama isinya bukan token `Bearer` —
// sejak ada JWT, header Authorization dipakai untuk access token user.
func RequireApiKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")

		if key == "" {
			if legacy := c.GetHeader("Authorization"); !strings.HasPrefix(strings.ToLower(legacy), "bearer ") {
				key = legacy
			}
		}

		if !helpers.SecureCompare(key, config.API_KEY) {
			c.JSON(
				http.StatusUnauthorized,
				helpers.APIResponse(
					http.StatusUnauthorized,
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
