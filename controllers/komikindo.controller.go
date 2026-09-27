package controllers

import (
	"errors"
	"fmt"
	"komikindo-scraper/helpers"
	model_komik "komikindo-scraper/model/komik"
	"komikindo-scraper/scraper"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// GetAllPopulerKomik mengambil komik populer dari halaman depan provider.
// Hasilnya di-cache karena endpoint ini dipanggil setiap homepage dibuka.
func (controller *KomikindoController) GetAllPopulerKomik(c *gin.Context) {

	dataKomik, err := cached("populer", 10*time.Minute, scraper.FetchPopuler)
	if err != nil {
		log.Println("Gagal mengambil komik populer:", err)
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

	keyword := strings.ToLower(strings.TrimSpace(c.Query("komik")))

	dataKomik, err := cached("search:"+keyword, 5*time.Minute, func() ([]model_komik.Komik, error) {
		return scraper.Search(keyword)
	})
	if err != nil {
		log.Println("Gagal mencari komik", keyword, ":", err)
		notFound(c, "Gagal menemukan komik")
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
		dataKomik, err = cached("komik:"+slugKomik, time.Minute, func() (model_komik.Komik, error) {
			return controller.scrapeKomik(slugKomik)
		})
		if err != nil {
			log.Println("Gagal mengambil komik", slugKomik, ":", err)
			if errors.Is(err, errDatabase) {
				internalError(c, "Gagal menyimpan data komik")
			} else {
				notFound(c, "Data chapter tidak ditemukan")
			}
			return
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

// errDatabase membedakan kegagalan database (500) dari komik yang memang tidak
// ada di provider (404).
var errDatabase = errors.New("database")

// scrapeKomik mengambil komik yang belum tersimpan dari provider lalu
// menyimpannya.
func (controller *KomikindoController) scrapeKomik(slugKomik string) (model_komik.Komik, error) {
	komik, err := scraper.FetchKomik(slugKomik)
	if err != nil {
		return komik, err
	}

	if err := controller.db.Create(&komik).Error; err != nil {
		if !errors.Is(err, gorm.ErrDuplicatedKey) {
			return komik, fmt.Errorf("%w: %v", errDatabase, err)
		}

		// Instance lain menyimpan komik yang sama lebih dulu, pakai versi yang
		// sudah ada di database.
		if komik, err = controller.findKomik(slugKomik); err != nil {
			return komik, fmt.Errorf("%w: %v", errDatabase, err)
		}
	}

	return komik, nil
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
		dataPanel, err = cached("panel:"+chapter, time.Minute, func() ([]model_komik.KomikPanel, error) {
			return controller.refreshPanels(chapter)
		})
		if err != nil {
			log.Println("Gagal scraping panel", chapter, ":", err)
			notFound(c, "Data panel tidak ditemukan")
			return
		}
	} else if time.Since(dataPanel[0].CreatedAt) > panelMaxAge {
		// Panel lama tetap dikirim sekarang; pembaruannya jalan di latar
		// belakang, paling sering sekali per 10 menit per chapter supaya
		// provider yang sedang down tidak dibanjiri percobaan ulang.
		go cached("panel-refresh:"+chapter, 10*time.Minute, func() (struct{}, error) {
			if _, err := controller.refreshPanels(chapter); err != nil {
				log.Println("Gagal memperbarui panel", chapter, ":", err)
			}
			return struct{}{}, nil
		})
	}

	c.JSON(http.StatusOK, helpers.APIResponse(
		http.StatusOK,
		true,
		"Berhasil mengambil data panel",
		dataPanel,
	))

}

// panelMaxAge umur panel sebelum di-scrape ulang. Provider sering berganti
// domain gambar, jadi URL panel yang tersimpan lama-lama mati.
const panelMaxAge = 3 * 24 * time.Hour

// refreshPanels men-scrape panel chapter lalu mengganti yang tersimpan. Gagal
// menyimpan tidak menghalangi user membaca, panel hasil scraping tetap
// dikembalikan dan request berikutnya mencoba menyimpan lagi.
func (controller *KomikindoController) refreshPanels(chapter string) ([]model_komik.KomikPanel, error) {
	panels, err := scraper.FetchPanels(chapter)
	if err != nil {
		return nil, err
	}

	// Satu transaksi supaya pembaca lain tidak pernah melihat chapter kosong.
	err = controller.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("slug_chapter = ?", chapter).Delete(&model_komik.KomikPanel{}).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&panels).Error
	})
	if err != nil {
		log.Println("Gagal menyimpan panel", chapter, ":", err)
	}

	return panels, nil
}

func notFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, helpers.APIResponse(
		http.StatusNotFound,
		false,
		message,
		nil,
	))
}
