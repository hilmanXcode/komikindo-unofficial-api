package scraper

import (
	model_komik "komikindo-scraper/model/komik"
	"log"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/gosimple/slug"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ScraperKomikindo struct {
	db *gorm.DB
}

func NewScraperKomikindo(db *gorm.DB) *ScraperKomikindo {
	return &ScraperKomikindo{
		db: db,
	}
}

var provider_url = "https://komikindo.ch/"

func (s *ScraperKomikindo) ScrapeChapterKomik(komik model_komik.Komik) {

	var dataKomik []model_komik.KomikChapter

	url := provider_url + "komik/" + komik.Slug

	cly := colly.NewCollector()
	// Find and visit all links
	cly.OnHTML("div#chapter_list", func(e *colly.HTMLElement) {

		e.ForEach("li>span.lchx", func(i int, el *colly.HTMLElement) {

			titleChapter := el.ChildAttr("a", "title")
			slugChapter := strings.Replace(slug.Make(titleChapter), "komik-", "", -1)

			komikChapter := model_komik.KomikChapter{
				Title:       titleChapter,
				SlugChapter: slugChapter,
				KomikId:     strconv.FormatUint(uint64(komik.ID), 10),
			}

			dataKomik = append(dataKomik, komikChapter)
		})

	})

	cly.OnRequest(func(r *colly.Request) {
		log.Println("Scraping chapter:", url)
	})

	if err := cly.Visit(url); err != nil {
		log.Println("Gagal membuka", url, ":", err)
		return
	}

	// Provider yang sedang down, halaman yang hilang, atau struktur HTML yang
	// berubah membuat daftar chapter kosong. Tanpa penjagaan ini GORM menolak
	// insert dengan "empty slice found".
	if len(dataKomik) == 0 {
		log.Println("Tidak ada chapter terbaca untuk", komik.Slug, ", halaman dilewati")
		return
	}

	if len(dataKomik) == len(komik.KomikChapter) {
		return
	}

	err := s.db.Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&dataKomik).Error

	// Kegagalan menyimpan satu komik tidak boleh mematikan API: routine ini
	// jalan di latar belakang, jadi errornya dicatat lalu dilanjutkan.
	if err != nil {
		log.Println("Gagal menyimpan chapter untuk", komik.Slug, ":", err)
		return
	}

	log.Println("Chapter baru tersimpan untuk", komik.Slug)
}
