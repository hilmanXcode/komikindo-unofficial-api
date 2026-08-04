package scraper

import (
	"fmt"
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
		fmt.Println("Visiting", url)
	})

	cly.Visit(url)

	if len(dataKomik) != len(komik.KomikChapter) {
		err := s.db.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Create(&dataKomik).Error

		if err != nil {
			log.Fatal("Gagal menginsert data komik baru:", err.Error())
			return
		}

		fmt.Println("Berhasil menambahkan data baru melalui go routine")
	}

}
