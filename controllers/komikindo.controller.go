package controllers

import (
	"fmt"
	"komikindo-scraper/helpers"
	model_komik "komikindo-scraper/model/komik"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly/v2"
	"github.com/gosimple/slug"
	"gorm.io/gorm"
)

type KomikController struct {
	db *gorm.DB
}

func NewKomikController(db *gorm.DB) *KomikController {
	return &KomikController{
		db: db,
	}
}

var provider_url = "https://komikindo.ch/"

func (controller *KomikController) GetAllPopulerKomik(c *gin.Context) {

	var dataKomik []model_komik.Komik
	cly := colly.NewCollector()

	// Find and visit all links
	cly.OnHTML(".serieslist.pop", func(e *colly.HTMLElement) {

		e.ForEach("ul>li", func(i int, el *colly.HTMLElement) {
			urlKomik := el.ChildAttr("a", "href")
			titleKomik := el.ChildAttr("a", "title")
			imgUrl := el.ChildAttr("img", "src")
			slugKomik := strings.Replace(slug.Make(urlKomik), "https-komikindo-ch-komik-", "", -1)

			komikBaru := model_komik.Komik{
				Title:  titleKomik,
				ImgUrl: imgUrl,
				Slug:   slugKomik,
			}

			dataKomik = append(dataKomik, komikBaru)
		})

	})

	cly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	cly.Visit(provider_url)

	if dataKomik == nil {

		c.JSON(http.StatusRequestTimeout, helpers.APIResponse(
			http.StatusRequestTimeout,
			false,
			"Gagal mengambil data komik",
			nil,
		))

		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil data komik",
		dataKomik,
	))

}

func (controller *KomikController) SearchKomik(c *gin.Context) {

	input := c.DefaultQuery("komik", "")
	var url = fmt.Sprintf(`%s?s=%s`, provider_url, input)

	var dataKomik []model_komik.Komik
	cly := colly.NewCollector()

	// Find and visit all links
	cly.OnHTML(".film-list", func(e *colly.HTMLElement) {

		e.ForEach("div.animepost", func(i int, el *colly.HTMLElement) {
			urlKomik := el.ChildAttr("a", "href")
			titleKomik := el.ChildAttr("a", "title")
			imageUrl := el.ChildAttr("img", "src")
			titleSlug := strings.Replace(slug.Make(urlKomik), "https-komikindo-ch-komik-", "", -1)

			komikBaru := model_komik.Komik{
				Title:  titleKomik,
				Slug:   slug.Make(titleSlug),
				ImgUrl: imageUrl,
			}

			dataKomik = append(dataKomik, komikBaru)
		})

	})

	cly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	cly.Visit(url)

	if dataKomik == nil {

		c.JSON(http.StatusNotFound, helpers.APIResponse(
			http.StatusNotFound,
			false,
			"Gagal menemukan komik",
			nil,
		))
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Komik ditemukan",
		dataKomik,
	))

}

func (controller *KomikController) GetAllChaptersKomik(c *gin.Context) {

	slugKomik := c.Param("slug")

	if slugKomik == "" {
		c.JSON(http.StatusBadRequest, helpers.APIResponse(
			http.StatusBadRequest,
			false,
			"Parameter slug tidak boleh kosong",
			nil,
		))

		return
	}

	var url = provider_url + "komik/" + slugKomik

	// var dataChapter []model_komik.KomikChapter
	var dataKomik model_komik.Komik

	result := controller.db.Preload("KomikChapter").Where("slug = ?", slugKomik).Find(&dataKomik)

	if result.Error != nil {

		c.JSON(
			http.StatusInternalServerError,
			helpers.APIResponse(
				http.StatusInternalServerError,
				false,
				"Gagal mengambil data chapter, "+result.Error.Error(),
				nil,
			),
		)
		return

	}

	if dataKomik.Slug == "" {

		cly := colly.NewCollector()

		cly.OnHTML(".infoanime", func(e *colly.HTMLElement) {

			title := e.ChildText(".entry-title")
			description := e.ChildText(".infox .shortcsc.sht2>p")
			imgurl := e.ChildAttr(".thumb>img", "src")

			dataKomik.Title = strings.Join(strings.Fields(title), " ")
			dataKomik.Description = strings.Join(strings.Fields(description), " ")
			dataKomik.Slug = slugKomik
			dataKomik.ImgUrl = imgurl

			// dataKomik = model_komik.Komik{
			// 	Title:       strings.Join(strings.Fields(title), " "),
			// 	Description: strings.Join(strings.Fields(description), " "),
			// 	Slug:        slugKomik,
			// 	ImgUrl:      imgurl,
			// }

			// result := controller.db.Create(&dataKomik)

			if result.Error != nil {
				log.Fatal("Gagal menginsert ke database")
				return
			}

		})

		// Find and visit all links
		cly.OnHTML("div#chapter_list", func(e *colly.HTMLElement) {

			e.ForEach("li>span.lchx", func(i int, el *colly.HTMLElement) {

				titleChapter := el.ChildAttr("a", "title")
				slugChapter := strings.Replace(slug.Make(titleChapter), "komik-", "", -1)

				komikChapter := model_komik.KomikChapter{
					Title:       titleChapter,
					SlugChapter: slugChapter,
				}

				dataKomik.KomikChapter = append(dataKomik.KomikChapter, komikChapter)
			})

			result := controller.db.Create(&dataKomik)

			if result.Error != nil {
				c.JSON(
					http.StatusInternalServerError,
					helpers.APIResponse(
						http.StatusInternalServerError,
						false,
						"Gagal menginsert data ke database",
						nil,
					),
				)

				return
			}

		})

		cly.OnRequest(func(r *colly.Request) {
			fmt.Println("Visiting", url)
		})

		cly.Visit(url)
	}

	if len(dataKomik.KomikChapter) == 0 || dataKomik.Title == "" {
		c.JSON(http.StatusNoContent, helpers.APIResponse(
			http.StatusNoContent,
			false,
			"Data chapter tidak ditemukan",
			nil,
		))

		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil data chapter",
		dataKomik,
	))
}

func (controller *KomikController) GetPanelKomik(c *gin.Context) {
	chapter := c.Param("chapter")

	if chapter == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Parameter chapter tidak ditemukan",
			"success": nil,
			"data":    nil,
		})

		return
	}

	var url = provider_url + chapter

	var dataPanel []model_komik.KomikPanel

	result := controller.db.Where("slug_chapter = ?", chapter).Find(&dataPanel)

	if result.Error != nil {
		c.JSON(
			http.StatusInternalServerError,
			helpers.APIResponse(
				http.StatusInternalServerError,
				false,
				"Gagal mengambild data panel",
				nil,
			),
		)

		return
	}

	if len(dataPanel) == 0 {

		fmt.Println("MASUK KE COLLECT")

		cly := colly.NewCollector()

		// Find and visit all links
		cly.OnHTML("div#chimg-auh", func(e *colly.HTMLElement) {
			images := e.ChildAttrs("img", "src")

			panelNum := 1
			for _, img := range images {

				komikPanel := model_komik.KomikPanel{
					SlugChapter: chapter,
					PanelNumber: panelNum,
					ImgUrl:      img,
				}

				dataPanel = append(dataPanel, komikPanel)

				panelNum++
			}

			result := controller.db.Create(&dataPanel)

			if result.Error != nil {
				c.JSON(
					http.StatusInternalServerError,
					helpers.APIResponse(
						http.StatusInternalServerError,
						false,
						"Gagal menginsert data ke database",
						nil,
					),
				)

				return
			}

		})

		cly.OnRequest(func(r *colly.Request) {
			fmt.Println("Visiting", url)
		})

		cly.Visit(url)
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil data panel",
		dataPanel,
	))

}
