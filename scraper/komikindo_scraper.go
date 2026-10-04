package scraper

import (
	"errors"
	model_komik "komikindo-scraper/model/komik"
	"log"
	"net"
	"net/url"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gocolly/colly/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProviderURL diekspor supaya test bisa mengarahkannya ke server lokal.
var ProviderURL = "https://komikindo.ch/"

// ErrNotFound dipakai saat halaman terbuka tapi datanya kosong: slug salah,
// halaman dihapus provider, atau struktur HTML-nya berubah.
var ErrNotFound = errors.New("data tidak ditemukan di provider")

// base menyimpan pengaturan bersama. Setiap scraping memakai base.Clone(),
// yang berbagi HTTP client dan LimitRule, jadi batas koneksi ke provider
// berlaku untuk seluruh aplikasi, bukan per request.
var base = func() *colly.Collector {
	c := colly.NewCollector(
		// User-Agent bawaan Colly gampang diblokir Cloudflare.
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"),
		// Clone() berbagi daftar URL yang sudah dikunjungi. Tanpa ini, setiap
		// URL hanya bisa di-scrape sekali selama proses hidup: populer gagal
		// setelah cache habis, dan routine gagal di putaran kedua.
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(20 * time.Second)
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 4})
	return c
}()

// lastSuccess waktu (unix detik) scraping terakhir yang berhasil membaca data,
// dilaporkan /health. Port provider yang terbuka belum berarti scraping jalan.
var lastSuccess atomic.Int64

// LastSuccess mengembalikan waktu scraping terakhir yang berhasil, atau waktu
// nol kalau belum pernah.
func LastSuccess() time.Time {
	if v := lastSuccess.Load(); v > 0 {
		return time.Unix(v, 0)
	}
	return time.Time{}
}

func markSuccess() {
	lastSuccess.Store(time.Now().Unix())
}

type ScraperKomikindo struct {
	db *gorm.DB
}

// komikindoProvider membungkus scraper komikindo.ch sebagai salah satu Provider.
type komikindoProvider struct{}

func (komikindoProvider) Name() string { return "komikindo" }

func (komikindoProvider) CheckConnection() bool {
	_, err := net.DialTimeout("tcp", "komikindo.ch:443", time.Second)
	return err == nil
}

// OwnsChapter menandai slug chapter komikindo, yaitu yang tidak memakai
// pemisah khusus seperti narasininja.
func (komikindoProvider) OwnsChapter(chapter string) bool {
	return chapter != "" && !strings.Contains(chapter, chapterSlugSeparator)
}

func NewScraperKomikindo(db *gorm.DB) *ScraperKomikindo {
	return &ScraperKomikindo{
		db: db,
	}
}

// FetchPopuler mengambil komik populer dari slider halaman depan provider.
func (komikindoProvider) Populer() ([]model_komik.Komik, error) {
	return fetchCards(ProviderURL, "div.odadingslider")
}

// Search mencari komik lewat halaman pencarian provider.
func (komikindoProvider) Search(keyword string) ([]model_komik.Komik, error) {
	return fetchCards(ProviderURL+"?s="+url.QueryEscape(keyword), ".film-list")
}

// fetchCards membaca kartu komik (.animepost) di dalam container tertentu.
func fetchCards(pageURL, container string) ([]model_komik.Komik, error) {

	var list []model_komik.Komik

	cly := base.Clone()

	cly.OnHTML(container, func(e *colly.HTMLElement) {
		e.ForEach(".animepost", func(i int, el *colly.HTMLElement) {
			link := el.ChildAttr(".animposx>a", "href")

			slug, ok := slugFromURL(link)
			if !ok {
				return
			}

			list = append(list, model_komik.Komik{
				Title:  el.ChildAttr(".animposx>a", "title"),
				ImgUrl: el.ChildAttr(".animposx>a img", "src"),
				Slug:   slug,
			})
		})
	})

	log.Println("Scraping:", pageURL)

	if err := cly.Visit(pageURL); err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, ErrNotFound
	}

	markSuccess()
	return list, nil
}

// slugFromURL mengambil segmen terakhir URL provider, mis.
// https://komikindo.ch/komik/worst/ -> worst.
func slugFromURL(link string) (string, bool) {
	u, err := url.Parse(link)
	if err != nil || strings.Trim(u.Path, "/") == "" {
		return "", false
	}
	return path.Base(u.Path), true
}

// FetchKomik mengambil info komik beserta daftar chapter-nya dari provider.
func (komikindoProvider) Komik(slugKomik string) (model_komik.Komik, error) {

	komik := model_komik.Komik{Slug: slugKomik}

	cly := base.Clone()

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
			slugChapter, ok := slugFromURL(el.ChildAttr("a", "href"))
			if !ok {
				return
			}

			komik.KomikChapter = append(komik.KomikChapter, model_komik.KomikChapter{
				Title:       el.ChildAttr("a", "title"),
				SlugChapter: slugChapter,
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

	markSuccess()
	return komik, nil
}

// FetchPanels mengambil semua gambar panel dari satu chapter.
func (komikindoProvider) Panels(chapter string) ([]model_komik.KomikPanel, error) {

	var panels []model_komik.KomikPanel

	cly := base.Clone()

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

	markSuccess()
	return panels, nil
}

// ScrapeChapterKomik menyimpan chapter baru dari komik yang sudah ada di
// database, dan menandai komik yang sudah tamat supaya tidak di-scrape lagi.
func (s *ScraperKomikindo) ScrapeChapterKomik(komik model_komik.Komik) {

	// Pakai provider yang memiliki chapter yang sudah tersimpan, bukan provider
	// utama. Kalau tidak, komik dari provider yang sedang down akan ditimpa
	// chapter dari provider cadangan yang slug-nya berbeda format.
	provider := ProviderForChapters(komik.KomikChapter)
	if provider == nil {
		provider = PrimaryProvider()
	}

	fetched, err := provider.Komik(komik.Slug)
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

	// Chapter yang hanya berganti slug (added dan stale sama banyak) bukan
	// chapter baru. Provider mengurutkan chapter dari yang terbaru.
	if len(added) > len(stale) {
		latest := fetched.KomikChapter[0]
		err = s.db.Model(&komik).Updates(map[string]interface{}{
			"last_chapter_at":    time.Now(),
			"last_chapter_slug":  latest.SlugChapter,
			"last_chapter_title": latest.Title,
		}).Error
		if err != nil {
			log.Println("Gagal menandai chapter terbaru untuk", komik.Slug, ":", err)
		}
	}
}
