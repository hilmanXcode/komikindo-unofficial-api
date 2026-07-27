package komik_controller

import (
	"fmt"
	model_komik "komikindo-scraper/model/komik"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly/v2"
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
			urlKomik := el.ChildAttrs("a", "href")
			imageUrl := el.ChildAttrs("img", "src")

			komikBaru := model_komik.Komik{
				Title:  titleKomik[0],
				Url:    urlKomik[0],
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
	url := c.Query("url")

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
			urlChapter := el.ChildAttr("a", "href")

			komikChapter := model_komik.KomikChapter{
				Title:      titleChapter,
				UrlChapter: urlChapter,
			}

			dataChapter = append(dataChapter, komikChapter)

		})

	})

	cly.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", url)
	})

	cly.Visit(url)

	if len(dataChapter) == 0 {
		c.JSON(http.StatusOK, gin.H{
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
