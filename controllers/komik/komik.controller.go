package komik_controller

import (
	"fmt"
	model_komik "komikindo-scraper/model/komik"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly/v2"
	"github.com/gosimple/slug"
)

var provider_url = "https://komikindo.ch/"

func GetAllPopulerKomik(c *gin.Context) {

	var dataKomik []string
	cly := colly.NewCollector()

	// Find and visit all links
	cly.OnHTML(".serieslist.pop", func(e *colly.HTMLElement) {

		titles := e.ChildAttrs("a", "title")

		dataKomik = append(dataKomik, titles...)
	})

	cly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	cly.Visit(provider_url)

	if dataKomik != nil {

		c.JSON(http.StatusOK, gin.H{
			"data":    dataKomik,
			"error":   nil,
			"success": "Berhasil mengambil data komik",
		})

		return
	}

	c.JSON(http.StatusRequestTimeout, gin.H{
		"data":    nil,
		"error":   "Gagal mengambil data komik",
		"success": nil,
	})
}

func SearchKomik(c *gin.Context) {

	input := c.DefaultQuery("komik", "")
	var url = fmt.Sprintf(`%s?s=%s`, provider_url, input)

	var dataKomik []model_komik.Komik
	cly := colly.NewCollector()

	// Find and visit all links
	cly.OnHTML(".film-list", func(e *colly.HTMLElement) {

		e.ForEach("div.animepost", func(i int, el *colly.HTMLElement) {
			titleKomik := el.ChildAttrs("a", "title")
			imageUrl := el.ChildAttrs("img", "src")
			titleSlug := strings.Replace(slug.Make(titleKomik[0]), "komik-", "", -1)

			komikBaru := model_komik.Komik{
				Title:  titleKomik[0],
				Slug:   slug.Make(titleSlug),
				ImgUrl: imageUrl[0],
			}

			dataKomik = append(dataKomik, komikBaru)
		})

	})

	cly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})

	cly.Visit(url)

	if dataKomik != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
			"error":   nil,
			"data":    dataKomik,
		})

		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"message": nil,
		"error":   "Data komik tidak ditemukan.",
		"data":    dataKomik,
	})

}

func GetAllChaptersKomik(c *gin.Context) {

	slugKomik := c.Param("slug")

	var url = provider_url + "komik/" + slugKomik

	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Data url wajib di isi.",
			"success": nil,
			"data":    nil,
		})

		return
	}

	var dataChapter []model_komik.KomikChapter

	cly := colly.NewCollector()

	// Find and visit all links
	cly.OnHTML("div#chapter_list", func(e *colly.HTMLElement) {

		e.ForEach("li>span.lchx", func(i int, el *colly.HTMLElement) {

			titleChapter := el.ChildAttr("a", "title")
			slugChapter := strings.Replace(slug.Make(titleChapter), "komik-", "", -1)

			komikChapter := model_komik.KomikChapter{
				Title:       titleChapter,
				SlugChapter: slugChapter,
			}

			dataChapter = append(dataChapter, komikChapter)

		})

	})

	cly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", url)
	})

	cly.Visit(url)

	if len(dataChapter) == 0 {
		c.JSON(http.StatusNoContent, gin.H{
			"error":   "Komik tidak ditemukan",
			"success": nil,
			"data":    nil,
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"error":   nil,
		"success": "Berhasil mengambil data komik",
		"data":    dataChapter,
	})
}

func GetPanelKomik(c *gin.Context) {
	chapter := c.Param("chapter")

	var url = provider_url + chapter

	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Parameter chapter tidak ditemukan",
			"success": nil,
			"data":    nil,
		})

		return
	}

	var dataPanel []model_komik.KomikPanel

	cly := colly.NewCollector()

	// Find and visit all links
	cly.OnHTML("div#chimg-auh", func(e *colly.HTMLElement) {
		images := e.ChildAttrs("img", "src")

		panelNum := 1
		for _, img := range images {

			komikPanel := model_komik.KomikPanel{
				PanelNumber: panelNum,
				ImgUrl:      img,
			}

			dataPanel = append(dataPanel, komikPanel)

			panelNum++
		}

	})

	cly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", url)
	})

	cly.Visit(url)

	if len(dataPanel) == 0 {
		c.JSON(http.StatusNoContent, gin.H{
			"error":   "Data panel tidak ditemukan",
			"success": nil,
			"data":    nil,
		})

		return
	}

	c.JSON(http.StatusOK, gin.H{
		"error":   nil,
		"success": "Berhasil mengambil data panel",
		"data":    dataPanel,
	})

}
