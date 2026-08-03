package model_komik

import "gorm.io/gorm"

type Komik struct {
	gorm.Model
	Title        string         `json:"title"`
	ImgUrl       string         `json:"imgurl"`
	Slug         string         `json:"slug"`
	Description  string         `json:"description"`
	KomikChapter []KomikChapter `gorm:"foreignKey:KomikId"`
}

type KomikChapter struct {
	gorm.Model
	Title       string `json:"title"`
	SlugChapter string `json:"slugchapter"`
	KomikId     string `json:"komikid"`
}

type KomikPanel struct {
	gorm.Model
	SlugChapter string
	PanelNumber int    `json:"panelnumber"`
	ImgUrl      string `json:"imgurl"`
}
