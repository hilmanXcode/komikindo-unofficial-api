package model_komik

import "gorm.io/gorm"

type Komik struct {
	gorm.Model
	Title        string         `json:"title"`
	ImgUrl       string         `json:"imgurl"`
	Slug         string         `json:"slug" gorm:"type:varchar(200);uniqueIndex"`
	Description  string         `json:"description"`
	Status       string         `json:"status"`
	KomikChapter []KomikChapter `gorm:"foreignKey:KomikId"`
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
