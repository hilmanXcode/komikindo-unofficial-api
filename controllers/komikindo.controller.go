package controllers

import (
	"errors"
	"fmt"
	"komikindo-scraper/helpers"
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/scraper"
	"log"
	"net/http"
	neturl "net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly/v2"
	"github.com/gosimple/slug"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type KomikindoController struct {
	db *gorm.DB
}

func NewKomikindoController(db *gorm.DB) *KomikindoController {
	return &KomikindoController{
		db: db,
	}
}

// GetAllScrapedKomik mengembalikan komik yang sudah tersimpan di database.
//
// Mendukung query `q` (cari judul), `status`, `page`, dan `limit`. Tanpa `page`
// atau `limit` endpoint ini tetap mengembalikan seluruh data seperti versi
// sebelumnya, supaya klien lama tidak ikut berubah perilakunya.
func (controller *KomikindoController) GetAllScrapedKomik(c *gin.Context) {

	var dataKomik []model_komik.Komik

	query := controller.db.Model(&model_komik.Komik{})

	if keyword := strings.TrimSpace(c.Query("q")); keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	// Session() supaya query bisa dipakai ulang untuk Count dan Find tanpa
	// kondisi dari pemanggilan pertama ikut terbawa.
	query = query.Session(&gorm.Session{})

	if !isPaginated(c) {
		if query.Find(&dataKomik).Error != nil {
			c.JSON(
				http.StatusInternalServerError,
				helpers.APIResponse(
					http.StatusInternalServerError,
					false,
					"Gagal mengambil data komik di database",
					nil,
				),
			)
			return
		}

		c.JSON(
			http.StatusOK,
			helpers.APIResponse(
				http.StatusOK,
				true,
				"Berhasil Mengambil Data",
				dataKomik,
			),
		)

		return
	}

	page, limit := parsePagination(c, 20)

	var total int64
	query.Count(&total)

	err := query.
		Order("title asc").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&dataKomik).Error

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			helpers.APIResponse(
				http.StatusInternalServerError,
				false,
				"Gagal mengambil data komik di database",
				nil,
			),
		)
		return
	}

	c.JSON(
		http.StatusOK,
		helpers.APIResponseWithMeta(
			http.StatusOK,
			true,
			"Berhasil Mengambil Data",
			dataKomik,
			buildMeta(page, limit, total),
		),
	)

}

func (controller *KomikindoController) GetAllPopulerKomik(c *gin.Context) {

	var dataKomik []model_komik.Komik
	cly := colly.NewCollector()

	// Find and visit all links

	cly.OnHTML("div.odadingslider", func(e *colly.HTMLElement) {

		// fmt.Println(e.Body)
		e.ForEach(".animepost", func(i int, el *colly.HTMLElement) {
			urlKomik := el.ChildAttr(".animposx>a", "href")

			// fmt.Println(urlKomik)
			titleKomik := el.ChildAttr(".animposx>a", "title")
			imgUrl := el.ChildAttr(".animposx>a .limit>img", "src")
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

	if err := cly.Visit(scraper.ProviderURL); err != nil {
		fmt.Println("Gagal membuka", scraper.ProviderURL, ":", err)
	}

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

func (controller *KomikindoController) SearchKomik(c *gin.Context) {

	input := c.DefaultQuery("komik", "")
	var url = fmt.Sprintf(`%s?s=%s`, scraper.ProviderURL, neturl.QueryEscape(input))

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

	if err := cly.Visit(url); err != nil {
		fmt.Println("Gagal membuka", url, ":", err)
	}

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

func (controller *KomikindoController) GetAllChaptersKomik(c *gin.Context) {

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

	dataKomik, err := controller.findKomik(slugKomik)
	if err != nil {
		log.Println("Gagal mengambil komik", slugKomik, ":", err)
		internalError(c, "Gagal mengambil data chapter")
		return
	}

	if dataKomik.ID == 0 {
		dataKomik, err = scraper.FetchKomik(slugKomik)
		if err != nil {
			log.Println("Gagal scraping komik", slugKomik, ":", err)
			notFound(c, "Data chapter tidak ditemukan")
			return
		}

		if err := controller.db.Create(&dataKomik).Error; err != nil {
			if !errors.Is(err, gorm.ErrDuplicatedKey) {
				log.Println("Gagal menyimpan komik", slugKomik, ":", err)
				internalError(c, "Gagal menyimpan data komik")
				return
			}

			// Request lain menyimpan komik yang sama lebih dulu, pakai versi
			// yang sudah ada di database.
			if dataKomik, err = controller.findKomik(slugKomik); err != nil {
				log.Println("Gagal mengambil komik", slugKomik, ":", err)
				internalError(c, "Gagal mengambil data chapter")
				return
			}
		}
	}

	if len(dataKomik.KomikChapter) == 0 {
		notFound(c, "Data chapter tidak ditemukan")
		return
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil data chapter",
		dataKomik,
	))
}

// findKomik mengembalikan komik beserta chapter-nya, atau Komik kosong (ID 0)
// kalau belum tersimpan.
func (controller *KomikindoController) findKomik(slugKomik string) (model_komik.Komik, error) {
	var dataKomik model_komik.Komik
	err := controller.db.Preload("KomikChapter").Where("slug = ?", slugKomik).Limit(1).Find(&dataKomik).Error
	return dataKomik, err
}

func (controller *KomikindoController) GetPanelKomik(c *gin.Context) {
	chapter := c.Param("chapter")

	if chapter == "" {
		c.JSON(http.StatusBadRequest, helpers.APIResponse(
			http.StatusBadRequest,
			false,
			"Parameter chapter tidak ditemukan",
			nil,
		))

		return
	}

	var dataPanel []model_komik.KomikPanel

	err := controller.db.Where("slug_chapter = ?", chapter).Order("panel_number").Find(&dataPanel).Error
	if err != nil {
		log.Println("Gagal mengambil panel", chapter, ":", err)
		internalError(c, "Gagal mengambil data panel")
		return
	}

	if len(dataPanel) == 0 {
		dataPanel, err = scraper.FetchPanels(chapter)
		if err != nil {
			log.Println("Gagal scraping panel", chapter, ":", err)
			notFound(c, "Data panel tidak ditemukan")
			return
		}

		// Dua pembaca yang membuka chapter baru bersamaan sama-sama men-scrape;
		// unique index idx_chapter_panel memastikan panelnya tidak tersimpan dua
		// kali. Gagal menyimpan tidak menghalangi user membaca, request
		// berikutnya akan mencoba lagi.
		if err := controller.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&dataPanel).Error; err != nil {
			log.Println("Gagal menyimpan panel", chapter, ":", err)
		}
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil data panel",
		dataPanel,
	))

}

func notFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, helpers.APIResponse(
		http.StatusNotFound,
		false,
		message,
		nil,
	))
}
