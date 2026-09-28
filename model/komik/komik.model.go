package model_komik

import (
	"time"

	"gorm.io/gorm"
)

type Komik struct {
	gorm.Model
	Title        string         `json:"title"`
	ImgUrl       string         `json:"imgurl"`
	Slug         string         `json:"slug" gorm:"type:varchar(200);uniqueIndex"`
	Description  string         `json:"description"`
	Status       string         `json:"status"`
	KomikChapter []KomikChapter `gorm:"foreignKey:KomikId"`

	// Diisi routine saat menemukan chapter baru, untuk daftar "update
	// terbaru" dan penanda chapter baru di bookmark. Kosong kalau belum
	// pernah ada chapter baru sejak komik pertama kali disimpan.
	LastChapterAt    *time.Time `json:"last_chapter_at" gorm:"index"`
	LastChapterSlug  string     `json:"last_chapter_slug" gorm:"type:varchar(200)"`
	LastChapterTitle string     `json:"last_chapter_title"`
}

type KomikChapter struct {
	gorm.Model
	Title       string `json:"title"`
	SlugChapter string `json:"slugchapter" gorm:"type:varchar(200);uniqueIndex:idx_comic_chapter"`
	KomikId     uint   `json:"komikid" gorm:"uniqueIndex:idx_comic_chapter"`
}

type KomikPanel struct {
	gorm.Model
	SlugChapter string `gorm:"type:varchar(200);uniqueIndex:idx_chapter_panel"`
	PanelNumber int    `json:"panelnumber" gorm:"uniqueIndex:idx_chapter_panel"`
	ImgUrl      string `json:"imgurl"`
}

type KomikList struct {
	Komik []Komik
}
