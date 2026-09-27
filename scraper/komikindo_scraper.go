package scraper

import (
	"errors"
	model_komik "komikindo-scraper/model/komik"
	"log"
	"net/url"
	"path"
	"strings"

	"github.com/gocolly/colly/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProviderURL diekspor supaya test bisa mengarahkannya ke server lokal.
var ProviderURL = "https://komikindo.ch/"

// ErrNotFound dipakai saat halaman terbuka tapi datanya kosong: slug salah,
// halaman dihapus provider, atau struktur HTML-nya berubah.
var ErrNotFound = errors.New("data tidak ditemukan di provider")

type ScraperKomikindo struct {
	db *gorm.DB
}

func NewScraperKomikindo(db *gorm.DB) *ScraperKomikindo {
	return &ScraperKomikindo{
		db: db,
	}
}

// FetchKomik mengambil info komik beserta daftar chapter-nya dari provider.
func FetchKomik(slugKomik string) (model_komik.Komik, error) {

	komik := model_komik.Komik{Slug: slugKomik}

	cly := colly.NewCollector()

	cly.OnHTML(".infoanime", func(e *colly.HTMLElement) {
		komik.Title = strings.Join(strings.Fields(e.ChildText(".entry-title")), " ")
		komik.Description = strings.Join(strings.Fields(e.ChildText(".infox .shortcsc.sht2>p")), " ")
		komik.ImgUrl = e.ChildAttr(".thumb>img", "src")

		if strings.Contains(strings.ToLower(e.ChildText(".infox .spe>span")), "tamat") {
			komik.Status = "Tamat"
		} else {
			komik.Status = "Berjalan"
		}
	})

	cly.OnHTML("div#chapter_list", func(e *colly.HTMLElement) {
		e.ForEach("li>span.lchx", func(i int, el *colly.HTMLElement) {
			// Slug diambil dari URL chapter, bukan dibuat dari judul: judul
			// seperti "HiGH & LOW" menjadi "high-and-low" padahal URL
			// provider-nya "high-low", sehingga panelnya tidak bisa dibuka.
			href, err := url.Parse(el.ChildAttr("a", "href"))
			if err != nil || strings.Trim(href.Path, "/") == "" {
				return
			}

			komik.KomikChapter = append(komik.KomikChapter, model_komik.KomikChapter{
				Title:       el.ChildAttr("a", "title"),
				SlugChapter: path.Base(href.Path),
			})
		})
	})

	komikURL := ProviderURL + "komik/" + slugKomik
	log.Println("Scraping komik:", komikURL)

	if err := cly.Visit(komikURL); err != nil {
		return komik, err
	}

	if komik.Title == "" || len(komik.KomikChapter) == 0 {
		return komik, ErrNotFound
	}

	return komik, nil
}

// FetchPanels mengambil semua gambar panel dari satu chapter.
func FetchPanels(chapter string) ([]model_komik.KomikPanel, error) {

	var panels []model_komik.KomikPanel

	cly := colly.NewCollector()

	cly.OnHTML("div#chimg-auh", func(e *colly.HTMLElement) {
		for _, img := range e.ChildAttrs("img", "src") {
			panels = append(panels, model_komik.KomikPanel{
				SlugChapter: chapter,
				PanelNumber: len(panels) + 1,
				ImgUrl:      img,
			})
		}
	})

	chapterURL := ProviderURL + chapter
	log.Println("Scraping panel:", chapterURL)

	if err := cly.Visit(chapterURL); err != nil {
		return nil, err
	}

	if len(panels) == 0 {
		return nil, ErrNotFound
	}

	return panels, nil
}

// ScrapeChapterKomik menyimpan chapter baru dari komik yang sudah ada di
// database, dan menandai komik yang sudah tamat supaya tidak di-scrape lagi.
func (s *ScraperKomikindo) ScrapeChapterKomik(komik model_komik.Komik) {

	fetched, err := FetchKomik(komik.Slug)
	if err != nil {
		log.Println("Gagal scraping", komik.Slug, ":", err)
		return
	}

	if fetched.Status != komik.Status {
		if err := s.db.Model(&komik).Update("status", fetched.Status).Error; err != nil {
			log.Println("Gagal memperbarui status", komik.Slug, ":", err)
		}
	}

	// Daftar chapter di database disamakan dengan provider: chapter baru
	// ditambah, chapter yang slug-nya tidak ada lagi di provider dibuang.
	existing := map[string]bool{}
	for _, ch := range komik.KomikChapter {
		existing[ch.SlugChapter] = true
	}

	onProvider := map[string]bool{}
	var added []model_komik.KomikChapter
	for _, ch := range fetched.KomikChapter {
		onProvider[ch.SlugChapter] = true
		if !existing[ch.SlugChapter] {
			ch.KomikId = komik.ID
			added = append(added, ch)
		}
	}

	var stale []uint
	for _, ch := range komik.KomikChapter {
		if !onProvider[ch.SlugChapter] {
			stale = append(stale, ch.ID)
		}
	}

	if len(stale) > 0 {
		if err := s.db.Unscoped().Delete(&model_komik.KomikChapter{}, stale).Error; err != nil {
			log.Println("Gagal membuang chapter lama untuk", komik.Slug, ":", err)
		}
	}

	if len(added) == 0 {
		return
	}

	// Kegagalan satu komik tidak boleh mematikan routine.
	err = s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&added).Error
	if err != nil {
		log.Println("Gagal menyimpan chapter untuk", komik.Slug, ":", err)
		return
	}

	log.Printf("%d chapter baru tersimpan untuk %s", len(added), komik.Slug)
}
