package routes

import (
	"komikindo-scraper/config"
	"komikindo-scraper/controllers"
	"komikindo-scraper/middleware"
	model_user "komikindo-scraper/model/user"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

func InitRoute(app *gin.Engine, db *gorm.DB) {

	komikindoController := controllers.NewKomikindoController(db)
	authController := controllers.NewAuthController(db)
	userController := controllers.NewUserController(db)
	healthController := controllers.NewHealthController(db)

	route := app

	route.Use(cors.New(cors.Config{
		AllowOrigins:     config.CORS_ORIGINS,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	route.Use(gin.Recovery())

	// Health check dibiarkan publik supaya uptime monitor tidak perlu API key.
	route.GET("/health", healthController.Health)

	v1 := route.Group("/v1")
	v1.Use(middleware.RateLimiter(rate.Limit(config.RATE_LIMIT_RPS), config.RATE_LIMIT_BURST))
	v1.Use(middleware.RequireApiKey())

	// ---------------------------------------------------------------------
	// Autentikasi
	// ---------------------------------------------------------------------
	// Refresh menukar token acak, bukan kredensial yang bisa ditebak, jadi tidak
	// ikut limiter ketat login. Kalau ikut, beberapa request paralel dari satu
	// pengunjung bisa menghabiskan jatah dan menendang sesi yang masih sah.
	v1.POST("/auth/refresh", authController.Refresh)

	// Endpoint kredensial dibatasi jauh lebih ketat dari endpoint biasa karena ini
	// permukaan yang paling menarik untuk brute force: 10 request per menit, burst 5.
	auth := v1.Group("/auth")
	auth.Use(middleware.RateLimiter(rate.Every(6*time.Second), 5))
	{
		auth.POST("/register", authController.Register)
		auth.POST("/login", authController.Login)
		auth.POST("/logout", authController.Logout)

		authed := auth.Group("")
		authed.Use(middleware.RequireAuth(db))
		{
			authed.GET("/me", authController.Me)
			authed.GET("/sessions", authController.Sessions)
			authed.POST("/logout-all", authController.LogoutAll)
			authed.PATCH("/change-password", authController.ChangePassword)
		}
	}

	// ---------------------------------------------------------------------
	// Data komik (publik, cukup API key)
	// ---------------------------------------------------------------------
	v1.GET("/get_all_komik", komikindoController.GetAllScrapedKomik)

	v1.GET("/populer_komik", komikindoController.GetAllPopulerKomik)

	v1.GET("/search_komik", komikindoController.SearchKomik)

	v1.GET("/get_all_chapter/:slug", komikindoController.GetAllChaptersKomik)

	v1.GET("/get_panel_komik/:chapter", komikindoController.GetPanelKomik)

	// ---------------------------------------------------------------------
	// Milik user (butuh login)
	// ---------------------------------------------------------------------
	me := v1.Group("")
	me.Use(middleware.RequireAuth(db))
	{
		me.GET("/bookmarks", userController.GetBookmarks)
		me.POST("/bookmarks", userController.AddBookmark)
		me.DELETE("/bookmarks/:slug", userController.DeleteBookmark)

		me.GET("/history", userController.GetHistory)
		me.POST("/history", userController.SaveHistory)
		me.DELETE("/history/:slug", userController.DeleteHistory)
	}

	// ---------------------------------------------------------------------
	// Admin
	// ---------------------------------------------------------------------
	admin := v1.Group("/admin")
	admin.Use(middleware.RequireAuth(db), middleware.RequireRole(model_user.RoleAdmin))
	{
		admin.GET("/users", userController.ListUsers)
		admin.PATCH("/users/:id", userController.UpdateUser)
	}

	app.Run(":" + config.PORT)
}
