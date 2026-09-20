package controllers

import (
	"komikindo-scraper/helpers"
	"komikindo-scraper/middleware"
	model_komik "komikindo-scraper/model/komik"
	model_user "komikindo-scraper/model/user"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserController struct {
	db *gorm.DB
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{
		db: db,
	}
}

type bookmarkInput struct {
	KomikSlug string `json:"komik_slug" binding:"required,max=200"`
	Title     string `json:"title" binding:"max=255"`
	ImgUrl    string `json:"imgurl" binding:"max=500"`
}

type historyInput struct {
	KomikSlug    string `json:"komik_slug" binding:"required,max=200"`
	ChapterSlug  string `json:"chapter_slug" binding:"required,max=200"`
	ChapterTitle string `json:"chapter_title" binding:"max=255"`
	LastPanel    int    `json:"last_panel" binding:"min=0"`
}

// GetBookmarks menampilkan komik yang di-bookmark user, terbaru lebih dulu.
func (controller *UserController) GetBookmarks(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	page, limit := parsePagination(c, 20)

	var total int64
	controller.db.Model(&model_user.Bookmark{}).Where("user_id = ?", user.ID).Count(&total)

	var bookmarks []model_user.Bookmark
	err := controller.db.
		Where("user_id = ?", user.ID).
		Order("created_at desc").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&bookmarks).Error

	if err != nil {
		internalError(c, "Gagal mengambil data bookmark")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponseWithMeta(
		http.StatusOK,
		true,
		"Berhasil mengambil data bookmark",
		bookmarks,
		buildMeta(page, limit, total),
	))
}

// AddBookmark menyimpan komik ke bookmark user. Judul dan gambar diisi otomatis
// dari database komik kalau klien tidak mengirimnya.
func (controller *UserController) AddBookmark(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	var input bookmarkInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	input.KomikSlug = strings.TrimSpace(input.KomikSlug)

	if input.Title == "" || input.ImgUrl == "" {
		var komik model_komik.Komik
		if controller.db.Where("slug = ?", input.KomikSlug).First(&komik).Error == nil {
			if input.Title == "" {
				input.Title = komik.Title
			}
			if input.ImgUrl == "" {
				input.ImgUrl = komik.ImgUrl
			}
		}
	}

	bookmark := model_user.Bookmark{
		UserID:    user.ID,
		KomikSlug: input.KomikSlug,
		Title:     input.Title,
		ImgUrl:    input.ImgUrl,
	}

	// Bookmark ulang komik yang sama bukan error, cukup diabaikan.
	err := controller.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&bookmark).Error

	if err != nil {
		internalError(c, "Gagal menyimpan bookmark")
		return
	}

	c.JSON(http.StatusCreated, helpers.APIResponse(
		http.StatusCreated,
		true,
		"Berhasil menambahkan bookmark",
		bookmark,
	))
}

// DeleteBookmark menghapus satu bookmark milik user.
func (controller *UserController) DeleteBookmark(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	slugKomik := c.Param("slug")

	result := controller.db.
		Where("user_id = ? AND komik_slug = ?", user.ID, slugKomik).
		Delete(&model_user.Bookmark{})

	if result.Error != nil {
		internalError(c, "Gagal menghapus bookmark")
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, helpers.APIResponse(
			http.StatusNotFound,
			false,
			"Bookmark tidak ditemukan",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil menghapus bookmark",
		nil,
	))
}

// GetHistory menampilkan riwayat baca user, yang terakhir dibaca lebih dulu.
func (controller *UserController) GetHistory(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	page, limit := parsePagination(c, 20)

	var total int64
	controller.db.Model(&model_user.ReadingHistory{}).Where("user_id = ?", user.ID).Count(&total)

	var history []model_user.ReadingHistory
	err := controller.db.
		Where("user_id = ?", user.ID).
		Order("last_read_at desc").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&history).Error

	if err != nil {
		internalError(c, "Gagal mengambil riwayat baca")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponseWithMeta(
		http.StatusOK,
		true,
		"Berhasil mengambil riwayat baca",
		history,
		buildMeta(page, limit, total),
	))
}

// SaveHistory mencatat chapter terakhir yang dibaca. Satu komik hanya punya satu
// baris riwayat, jadi pemanggilan berikutnya menimpa yang lama (upsert).
func (controller *UserController) SaveHistory(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	var input historyInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	history := model_user.ReadingHistory{
		UserID:       user.ID,
		KomikSlug:    strings.TrimSpace(input.KomikSlug),
		ChapterSlug:  strings.TrimSpace(input.ChapterSlug),
		ChapterTitle: input.ChapterTitle,
		LastPanel:    input.LastPanel,
		LastReadAt:   time.Now(),
	}

	err := controller.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "komik_slug"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"chapter_slug", "chapter_title", "last_panel", "last_read_at", "updated_at",
		}),
	}).Create(&history).Error

	if err != nil {
		internalError(c, "Gagal menyimpan riwayat baca")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil menyimpan riwayat baca",
		history,
	))
}

// DeleteHistory menghapus riwayat baca satu komik.
func (controller *UserController) DeleteHistory(c *gin.Context) {

	user, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	slugKomik := c.Param("slug")

	result := controller.db.
		Where("user_id = ? AND komik_slug = ?", user.ID, slugKomik).
		Delete(&model_user.ReadingHistory{})

	if result.Error != nil {
		internalError(c, "Gagal menghapus riwayat baca")
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, helpers.APIResponse(
			http.StatusNotFound,
			false,
			"Riwayat baca tidak ditemukan",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil menghapus riwayat baca",
		nil,
	))
}

// ListUsers menampilkan daftar akun. Khusus admin.
func (controller *UserController) ListUsers(c *gin.Context) {

	page, limit := parsePagination(c, 20)

	query := controller.db.Model(&model_user.User{})

	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", like, like)
	}

	query = query.Session(&gorm.Session{})

	var total int64
	query.Count(&total)

	var users []model_user.User
	err := query.
		Order("created_at desc").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&users).Error

	if err != nil {
		internalError(c, "Gagal mengambil data user")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponseWithMeta(
		http.StatusOK,
		true,
		"Berhasil mengambil data user",
		users,
		buildMeta(page, limit, total),
	))
}

type updateUserInput struct {
	Role     *model_user.Role `json:"role" binding:"omitempty,oneof=user admin"`
	IsActive *bool            `json:"is_active"`
}

// UpdateUser mengubah role atau status aktif sebuah akun. Khusus admin.
func (controller *UserController) UpdateUser(c *gin.Context) {

	admin, ok := middleware.CurrentUser(c)
	if !ok {
		internalError(c, "Gagal membaca user dari context")
		return
	}

	var input updateUserInput

	if err := c.ShouldBindJSON(&input); err != nil {
		badRequest(c, "Input tidak valid: "+err.Error())
		return
	}

	if input.Role == nil && input.IsActive == nil {
		badRequest(c, "Tidak ada field yang diubah")
		return
	}

	var target model_user.User
	if err := controller.db.Where("id = ?", c.Param("id")).First(&target).Error; err != nil {
		c.JSON(http.StatusNotFound, helpers.APIResponse(
			http.StatusNotFound,
			false,
			"User tidak ditemukan",
			nil,
		))
		return
	}

	// Admin tidak boleh menurunkan atau menonaktifkan akunnya sendiri, supaya
	// tidak ada kemungkinan sistem kehilangan admin terakhirnya karena salah klik.
	if target.ID == admin.ID {
		c.JSON(http.StatusForbidden, helpers.APIResponse(
			http.StatusForbidden,
			false,
			"Tidak bisa mengubah role atau status akun sendiri",
			nil,
		))
		return
	}

	updates := map[string]interface{}{}

	if input.Role != nil {
		updates["role"] = *input.Role
	}

	if input.IsActive != nil {
		updates["is_active"] = *input.IsActive
	}

	if err := controller.db.Model(&target).Updates(updates).Error; err != nil {
		internalError(c, "Gagal memperbarui user")
		return
	}

	// Akun yang dinonaktifkan atau diturunkan rolenya tidak boleh lanjut memakai
	// token lamanya.
	controller.db.Model(&model_user.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", target.ID).
		Update("revoked_at", time.Now())

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil memperbarui user",
		target,
	))
}
