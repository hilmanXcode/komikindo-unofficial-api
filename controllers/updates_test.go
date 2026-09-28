package controllers

import (
	"encoding/json"
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/middleware"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Penanda chapter baru di bookmark: muncul setelah routine menemukan chapter
// baru, hilang setelah user membaca komik itu.
func TestBookmarkHasUpdate(t *testing.T) {
	db := testDB(t)
	user := testUser(t, db)
	uc := NewUserController(db)

	komik := model_komik.Komik{Title: "Tes Update", Slug: "tes-update-" + user.Username}
	if err := db.Create(&komik).Error; err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(middleware.ContextUserKey, user) })
	r.GET("/bookmarks", uc.GetBookmarks)
	r.POST("/bookmarks", uc.AddBookmark)
	r.POST("/history", uc.SaveHistory)

	type result struct {
		Data []struct {
			KomikSlug       string `json:"komik_slug"`
			HasUpdate       bool   `json:"has_update"`
			LastChapterSlug string `json:"last_chapter_slug"`
		}
		Meta struct{ Total int64 }
	}
	get := func(query string) result {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bookmarks"+query, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET /bookmarks%s = %d: %s", query, w.Code, w.Body)
		}
		var res result
		json.Unmarshal(w.Body.Bytes(), &res)
		return res
	}

	post(r, "/bookmarks", `{"komik_slug":"`+komik.Slug+`"}`)
	post(r, "/bookmarks", `{"komik_slug":"komik-yang-belum-tersimpan"}`)

	if res := get(""); len(res.Data) != 2 || res.Data[0].HasUpdate || res.Data[1].HasUpdate {
		t.Fatalf("sebelum ada chapter baru: %+v", res.Data)
	}

	// Routine menemukan chapter baru setelah komik disimpan.
	db.Model(&komik).Updates(map[string]interface{}{
		"last_chapter_at":   time.Now().Add(time.Second),
		"last_chapter_slug": "tes-update-chapter-2",
	})

	res := get("")
	if len(res.Data) != 2 || res.Data[0].KomikSlug != komik.Slug || !res.Data[0].HasUpdate ||
		res.Data[0].LastChapterSlug != "tes-update-chapter-2" || res.Data[1].HasUpdate {
		t.Fatalf("setelah chapter baru, yang ada update harus di atas: %+v", res.Data)
	}
	if res := get("?has_update=1&limit=1"); res.Meta.Total != 1 {
		t.Fatalf("jumlah notifikasi = %d, mau 1", res.Meta.Total)
	}

	// User membaca komik itu, penandanya hilang.
	db.Model(&komik).Update("last_chapter_at", time.Now().Add(-time.Minute))
	post(r, "/history", `{"komik_slug":"`+komik.Slug+`","chapter_slug":"tes-update-chapter-2"}`)

	if res := get("?has_update=1"); res.Meta.Total != 0 {
		t.Fatalf("setelah dibaca, jumlah notifikasi = %d, mau 0", res.Meta.Total)
	}
}
