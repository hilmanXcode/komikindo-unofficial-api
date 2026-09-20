package routine

import (
	model_user "komikindo-scraper/model/user"
	"log"
	"time"

	"gorm.io/gorm"
)

// AuthRoutine menjalankan pemeliharaan berkala untuk data autentikasi.
func AuthRoutine(db *gorm.DB) {
	go cleanupRefreshTokens(db)
}

// cleanupRefreshTokens membuang refresh token yang sudah kedaluwarsa atau sudah
// lama dicabut, supaya tabelnya tidak tumbuh tanpa batas.
func cleanupRefreshTokens(db *gorm.DB) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		cutoff := time.Now().Add(-24 * time.Hour)

		result := db.Unscoped().
			Where("expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)", time.Now(), cutoff).
			Delete(&model_user.RefreshToken{})

		if result.Error != nil {
			log.Println("Gagal membersihkan refresh token:", result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("Membersihkan %d refresh token kedaluwarsa", result.RowsAffected)
		}

		<-ticker.C
	}
}
