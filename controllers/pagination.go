package controllers

import (
	"komikindo-scraper/helpers"
	"strconv"

	"github.com/gin-gonic/gin"
)

const maxPageLimit = 100

// parsePagination membaca query `page` dan `limit`. Nilai yang tidak valid
// jatuh ke default, dan limit dibatasi maxPageLimit supaya satu request tidak
// bisa menarik seluruh tabel.
func parsePagination(c *gin.Context, defaultLimit int) (page int, limit int) {

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err = strconv.Atoi(c.Query("limit"))
	if err != nil || limit < 1 {
		limit = defaultLimit
	}

	if limit > maxPageLimit {
		limit = maxPageLimit
	}

	return page, limit
}

// isPaginated true kalau klien memang meminta paginasi. Dipakai endpoint lama
// yang default-nya masih mengembalikan seluruh data demi kompatibilitas.
func isPaginated(c *gin.Context) bool {
	_, hasPage := c.GetQuery("page")
	_, hasLimit := c.GetQuery("limit")

	return hasPage || hasLimit
}

func buildMeta(page, limit int, total int64) helpers.Meta {

	totalPages := 0
	if limit > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}

	return helpers.Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
