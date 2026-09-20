package middleware

import (
	"komikindo-scraper/helpers"
	model_user "komikindo-scraper/model/user"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	// ContextUserKey adalah key context tempat *model_user.User disimpan.
	ContextUserKey = "currentUser"
	// ContextUserIDKey adalah key context tempat ID user disimpan.
	ContextUserIDKey = "currentUserID"
)

// RequireAuth memvalidasi access token pada header `Authorization: Bearer <token>`
// lalu memuat user-nya dari database. User di-load ulang setiap request supaya
// perubahan role atau penonaktifan akun langsung berlaku tanpa menunggu token
// kedaluwarsa.
func RequireAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			unauthorized(c, "Access token tidak ditemukan pada header Authorization")
			return
		}

		claims, err := helpers.ParseAccessToken(token)
		if err != nil {
			unauthorized(c, "Access token tidak valid atau sudah kedaluwarsa")
			return
		}

		userID, err := claims.UserID()
		if err != nil {
			unauthorized(c, "Access token tidak valid")
			return
		}

		var user model_user.User
		if err := db.First(&user, userID).Error; err != nil {
			unauthorized(c, "Akun tidak ditemukan")
			return
		}

		if !user.IsActive {
			c.JSON(http.StatusForbidden, helpers.APIResponse(
				http.StatusForbidden,
				false,
				"Akun dinonaktifkan",
				nil,
			))
			c.Abort()
			return
		}

		c.Set(ContextUserKey, &user)
		c.Set(ContextUserIDKey, user.ID)

		c.Next()
	}
}

// RequireRole membatasi akses hanya untuk role tertentu. Harus dipasang
// setelah RequireAuth.
func RequireRole(roles ...model_user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			unauthorized(c, "Autentikasi dibutuhkan")
			return
		}

		for _, role := range roles {
			if user.Role == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, helpers.APIResponse(
			http.StatusForbidden,
			false,
			"Role kamu tidak punya akses ke resource ini",
			nil,
		))
		c.Abort()
	}
}

// OptionalAuth mengisi context user kalau ada token yang valid, tapi tetap
// meneruskan request kalau token tidak ada atau tidak valid. Berguna untuk
// endpoint publik yang responsnya bisa dipersonalisasi.
func OptionalAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			c.Next()
			return
		}

		claims, err := helpers.ParseAccessToken(token)
		if err != nil {
			c.Next()
			return
		}

		userID, err := claims.UserID()
		if err != nil {
			c.Next()
			return
		}

		var user model_user.User
		if err := db.First(&user, userID).Error; err == nil && user.IsActive {
			c.Set(ContextUserKey, &user)
			c.Set(ContextUserIDKey, user.ID)
		}

		c.Next()
	}
}

// CurrentUser mengambil user yang sedang login dari context.
func CurrentUser(c *gin.Context) (*model_user.User, bool) {
	value, exists := c.Get(ContextUserKey)
	if !exists {
		return nil, false
	}

	user, ok := value.(*model_user.User)
	return user, ok
}

func bearerToken(c *gin.Context) (string, bool) {
	header := c.GetHeader("Authorization")
	if header == "" {
		return "", false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}

	return token, true
}

func unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, helpers.APIResponse(
		http.StatusUnauthorized,
		false,
		message,
		nil,
	))
	c.Abort()
}
