package controllers

import (
	"errors"
	"komikindo-scraper/config"
	"komikindo-scraper/helpers"
	"komikindo-scraper/middleware"
	model_user "komikindo-scraper/model/user"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthController struct {
	db *gorm.DB
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{
		db: db,
	}
}

type registerInput struct {
	Username string `json:"username" binding:"required,min=3,max=30,alphanum"`
	Email    string `json:"email" binding:"required,email,max=120"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type loginInput struct {
	// Identifier bisa diisi username atau email.
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type refreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type changePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

type tokenPair struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	TokenType    string           `json:"token_type"`
	ExpiresIn    int              `json:"expires_in"`
	ExpiresAt    time.Time        `json:"expires_at"`
	User         *model_user.User `json:"user,omitempty"`
}

// Register membuat akun baru. Username dan email harus unik.
func (controller *AuthController) Register(c *gin.Context) {

	var input registerInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	var existing int64
	controller.db.Model(&model_user.User{}).
		Where("username = ? OR email = ?", input.Username, input.Email).
		Count(&existing)

	if existing > 0 {
		c.JSON(http.StatusConflict, helpers.APIResponse(
			http.StatusConflict,
			false,
			"Username atau email sudah terdaftar",
			nil,
		))
		return
	}

	hashed, err := helpers.HashPassword(input.Password)
	if err != nil {
		internalError(c, "Gagal memproses password")
		return
	}

	user := model_user.User{
		Username: input.Username,
		Email:    input.Email,
		Password: hashed,
		Role:     model_user.RoleUser,
		IsActive: true,
	}

	if err := controller.db.Create(&user).Error; err != nil {
		internalError(c, "Gagal membuat akun")
		return
	}

	pair, err := controller.issueTokenPair(c, &user)
	if err != nil {
		internalError(c, "Akun dibuat tapi gagal menerbitkan token, silakan login")
		return
	}

	c.JSON(http.StatusCreated, helpers.APIResponse(
		http.StatusCreated,
		true,
		"Registrasi berhasil",
		pair,
	))
}

// Login menukar kredensial dengan sepasang access token dan refresh token.
func (controller *AuthController) Login(c *gin.Context) {

	var input loginInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	identifier := strings.ToLower(strings.TrimSpace(input.Identifier))

	var user model_user.User
	err := controller.db.Where("username = ? OR email = ?", identifier, identifier).First(&user).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			internalError(c, "Gagal memproses login")
			return
		}

		// Pesan sengaja disamakan dengan kasus password salah supaya tidak
		// bisa dipakai menebak username mana yang terdaftar.
		invalidCredentials(c)
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, helpers.APIResponse(
			http.StatusForbidden,
			false,
			"Akun dinonaktifkan",
			nil,
		))
		return
	}

	if user.IsLocked() {
		c.JSON(http.StatusTooManyRequests, helpers.APIResponse(
			http.StatusTooManyRequests,
			false,
			"Akun dikunci sementara karena terlalu banyak percobaan login gagal. Coba lagi pada "+
				user.LockedUntil.Format(time.RFC3339),
			nil,
		))
		return
	}

	if !helpers.CheckPassword(user.Password, input.Password) {
		controller.registerFailedLogin(&user)
		invalidCredentials(c)
		return
	}

	now := time.Now()
	controller.db.Model(&user).Updates(map[string]interface{}{
		"failed_login_attempts": 0,
		"locked_until":          nil,
		"last_login_at":         now,
	})
	user.LastLoginAt = &now

	pair, err := controller.issueTokenPair(c, &user)
	if err != nil {
		internalError(c, "Gagal menerbitkan token")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Login berhasil",
		pair,
	))
}

// Refresh menukar refresh token yang masih berlaku dengan pasangan token baru.
// Token lama langsung dicabut (rotasi), jadi satu refresh token hanya bisa
// dipakai sekali.
func (controller *AuthController) Refresh(c *gin.Context) {

	var input refreshInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	var stored model_user.RefreshToken
	err := controller.db.Where("token_hash = ?", helpers.HashToken(input.RefreshToken)).First(&stored).Error

	if err != nil {
		c.JSON(http.StatusUnauthorized, helpers.APIResponse(
			http.StatusUnauthorized,
			false,
			"Refresh token tidak valid",
			nil,
		))
		return
	}

	if !stored.IsUsable() {
		// Refresh token yang sudah dicabut tapi dipakai lagi adalah tanda token
		// bocor: cabut semua sesi user supaya penyerang ikut tertendang.
		if stored.RevokedAt != nil {
			controller.revokeAllSessions(stored.UserID)
		}

		c.JSON(http.StatusUnauthorized, helpers.APIResponse(
			http.StatusUnauthorized,
			false,
			"Refresh token sudah kedaluwarsa atau dicabut, silakan login ulang",
			nil,
		))
		return
	}

	var user model_user.User
	if err := controller.db.First(&user, stored.UserID).Error; err != nil || !user.IsActive {
		c.JSON(http.StatusUnauthorized, helpers.APIResponse(
			http.StatusUnauthorized,
			false,
			"Akun tidak aktif atau tidak ditemukan",
			nil,
		))
		return
	}

	now := time.Now()
	controller.db.Model(&stored).Update("revoked_at", now)

	pair, err := controller.issueTokenPair(c, &user)
	if err != nil {
		internalError(c, "Gagal menerbitkan token")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Token berhasil diperbarui",
		pair,
	))
}

// Logout mencabut satu refresh token (sesi perangkat ini saja).
func (controller *AuthController) Logout(c *gin.Context) {

	var input refreshInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	controller.db.Model(&model_user.RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", helpers.HashToken(input.RefreshToken)).
		Update("revoked_at", time.Now())

	// Selalu balas sukses: klien tidak perlu tahu token itu ada atau tidak,
	// dan hasil akhirnya sama — token tersebut tidak bisa dipakai lagi.
	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Logout berhasil",
		nil,
	))
}

// LogoutAll mencabut seluruh refresh token milik user yang sedang login.
func (controller *AuthController) LogoutAll(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	controller.revokeAllSessions(user.ID)

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Semua sesi berhasil dicabut",
		nil,
	))
}

// Me mengembalikan profil user yang sedang login.
func (controller *AuthController) Me(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	var bookmarkCount, historyCount int64
	controller.db.Model(&model_user.Bookmark{}).Where("user_id = ?", user.ID).Count(&bookmarkCount)
	controller.db.Model(&model_user.ReadingHistory{}).Where("user_id = ?", user.ID).Count(&historyCount)

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil profil",
		gin.H{
			"id":             user.ID,
			"username":       user.Username,
			"email":          user.Email,
			"role":           user.Role,
			"is_active":      user.IsActive,
			"last_login_at":  user.LastLoginAt,
			"created_at":     user.CreatedAt,
			"total_bookmark": bookmarkCount,
			"total_riwayat":  historyCount,
		},
	))
}

// Sessions menampilkan daftar sesi (refresh token) yang masih aktif.
func (controller *AuthController) Sessions(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	var sessions []model_user.RefreshToken
	err := controller.db.
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", user.ID, time.Now()).
		Order("created_at desc").
		Find(&sessions).Error

	if err != nil {
		internalError(c, "Gagal mengambil daftar sesi")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil daftar sesi aktif",
		sessions,
	))
}

// ChangePassword mengganti password lalu mencabut semua sesi lama.
func (controller *AuthController) ChangePassword(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	var input changePasswordInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	if !helpers.CheckPassword(user.Password, input.OldPassword) {
		c.JSON(http.StatusUnauthorized, helpers.APIResponse(
			http.StatusUnauthorized,
			false,
			"Password lama salah",
			nil,
		))
		return
	}

	if input.OldPassword == input.NewPassword {
		badRequest(c, "Password baru harus berbeda dari password lama")
		return
	}

	hashed, err := helpers.HashPassword(input.NewPassword)
	if err != nil {
		internalError(c, "Gagal memproses password")
		return
	}

	if err := controller.db.Model(user).Update("password", hashed).Error; err != nil {
		internalError(c, "Gagal menyimpan password baru")
		return
	}

	// Ganti password = semua sesi lama dianggap tidak dipercaya lagi.
	controller.revokeAllSessions(user.ID)

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Password berhasil diubah, silakan login ulang",
		nil,
	))
}

// issueTokenPair menerbitkan access token JWT dan refresh token opaque baru.
func (controller *AuthController) issueTokenPair(c *gin.Context, user *model_user.User) (*tokenPair, error) {

	accessToken, expiresAt, err := helpers.GenerateAccessToken(user.ID, user.Username, string(user.Role))
	if err != nil {
		return nil, err
	}

	refreshToken, err := helpers.GenerateOpaqueToken()
	if err != nil {
		return nil, err
	}

	stored := model_user.RefreshToken{
		UserID:    user.ID,
		TokenHash: helpers.HashToken(refreshToken),
		ExpiresAt: time.Now().Add(config.REFRESH_TOKEN_TTL),
		UserAgent: truncate(c.Request.UserAgent(), 255),
		IP:        c.ClientIP(),
	}

	if err := controller.db.Create(&stored).Error; err != nil {
		return nil, err
	}

	return &tokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(config.ACCESS_TOKEN_TTL.Seconds()),
		ExpiresAt:    expiresAt,
		User:         user,
	}, nil
}

// registerFailedLogin menaikkan penghitung gagal login dan mengunci akun kalau
// sudah melewati batas.
func (controller *AuthController) registerFailedLogin(user *model_user.User) {

	attempts := user.FailedLoginAttempts + 1

	updates := map[string]interface{}{
		"failed_login_attempts": attempts,
	}

	if attempts >= config.MAX_LOGIN_ATTEMPTS {
		lockedUntil := time.Now().Add(config.LOGIN_LOCK_DURATION)
		updates["locked_until"] = lockedUntil
		updates["failed_login_attempts"] = 0
	}

	controller.db.Model(user).Updates(updates)
}

func (controller *AuthController) revokeAllSessions(userID uint) {
	controller.db.Model(&model_user.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max]
}

func invalidCredentials(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, helpers.APIResponse(
		http.StatusUnauthorized,
		false,
		"Username/email atau password salah",
		nil,
	))
}

func badRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, helpers.APIResponse(
		http.StatusBadRequest,
		false,
		message,
		nil,
	))
}

func internalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, helpers.APIResponse(
		http.StatusInternalServerError,
		false,
		message,
		nil,
	))
}
