package controllers

import (
	"bytes"
	"fmt"
	"komikindo-scraper/config"
	"komikindo-scraper/middleware"
	model_user "komikindo-scraper/model/user"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Test ini butuh MySQL sungguhan karena yang diuji adalah perilaku unique
// index dan upsert MySQL. Pakai database kosong khusus test, contoh:
//
//	TEST_DATABASE_DSN="root@tcp(127.0.0.1:3306)/db_komik_test?parseTime=true" go test ./...
func testDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN kosong")
	}

	gin.SetMode(gin.TestMode)
	config.DSN = dsn
	config.JWT_SECRET = []byte("test-secret-yang-panjangnya-minimal-32-karakter")
	config.ACCESS_TOKEN_TTL = 15 * time.Minute
	config.REFRESH_TOKEN_TTL = time.Hour

	return config.InitDatabase()
}

func testUser(t *testing.T, db *gorm.DB) *model_user.User {
	name := fmt.Sprintf("t%d", time.Now().UnixNano())
	user := &model_user.User{Username: name, Email: name + "@test.local", Password: "x", Role: model_user.RoleUser, IsActive: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func post(r http.Handler, path, body string) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w.Code
}

// Request paralel dengan refresh token yang sama tidak boleh menendang user
// keluar; pemakaian ulang setelah masa tenggang tetap dianggap pencurian.
func TestRefreshParallel(t *testing.T) {
	db := testDB(t)
	user := testUser(t, db)
	auth := NewAuthController(db)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	pair, err := auth.issueTokenPair(c, user)
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.POST("/refresh", auth.Refresh)
	body := `{"refresh_token":"` + pair.RefreshToken + `"}`

	codes := map[int]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code := post(r, "/refresh", body)
			mu.Lock()
			codes[code]++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if codes[http.StatusOK] != 1 || codes[http.StatusConflict] != 7 {
		t.Fatalf("status = %v, mau 1x200 dan 7x409", codes)
	}

	var active int64
	db.Model(&model_user.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Count(&active)
	if active != 1 {
		t.Fatalf("sesi aktif = %d, mau 1 (token hasil rotasi)", active)
	}

	// Lewat masa tenggang: token lama dipakai lagi berarti bocor.
	db.Model(&model_user.RefreshToken{}).Where("user_id = ? AND revoked_at IS NOT NULL", user.ID).
		Update("revoked_at", time.Now().Add(-time.Minute))

	if code := post(r, "/refresh", body); code != http.StatusUnauthorized {
		t.Fatalf("pakai ulang setelah tenggang = %d, mau 401", code)
	}

	db.Model(&model_user.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Count(&active)
	if active != 0 {
		t.Fatalf("sesi aktif = %d, mau 0 setelah pencurian terdeteksi", active)
	}
}

// Bookmark dan riwayat yang dihapus harus bisa disimpan lagi.
func TestDeleteThenSaveAgain(t *testing.T) {
	db := testDB(t)
	user := testUser(t, db)
	uc := NewUserController(db)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(middleware.ContextUserKey, user) })
	r.POST("/bookmarks", uc.AddBookmark)
	r.DELETE("/bookmarks/:slug", uc.DeleteBookmark)
	r.POST("/history", uc.SaveHistory)
	r.DELETE("/history/:slug", uc.DeleteHistory)

	del := func(path string) int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, path, nil))
		return w.Code
	}

	for range 2 {
		if code := post(r, "/bookmarks", `{"komik_slug":"worst","title":"Worst"}`); code != http.StatusCreated {
			t.Fatalf("tambah bookmark = %d", code)
		}
		if code := post(r, "/history", `{"komik_slug":"worst","chapter_slug":"worst-chapter-1"}`); code != http.StatusOK {
			t.Fatalf("simpan riwayat = %d", code)
		}

		var bookmarks, history int64
		db.Model(&model_user.Bookmark{}).Where("user_id = ?", user.ID).Count(&bookmarks)
		db.Model(&model_user.ReadingHistory{}).Where("user_id = ?", user.ID).Count(&history)
		if bookmarks != 1 || history != 1 {
			t.Fatalf("bookmark = %d, riwayat = %d, mau 1 dan 1", bookmarks, history)
		}

		if del("/bookmarks/worst") != http.StatusOK || del("/history/worst") != http.StatusOK {
			t.Fatal("gagal menghapus")
		}
	}
}
