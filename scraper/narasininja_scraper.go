package scraper

import (
	"log"
	"net"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	model_komik "komikindo-scraper/model/komik"

	"github.com/gocolly/colly/v2"
)

// NarasininjaURL diekspor supaya test bisa mengarahkannya ke server lokal.
var NarasininjaURL = "https://narasininja.net/"

// chapterSlugSeparator memisahkan slug komik dan slug chapter di SlugChapter.
// URL chapter narasininja berbentuk /komik/<komik>/<chapter>, sedangkan API
// hanya punya satu segmen untuk parameter chapter. Pemisah "~" dipilih karena
// tidak pernah muncul di slug manapun, jadi slug tetap unik, aman dipakai di
// URL, dan chapter URL-nya bisa dibentuk ulang.
const chapterSlugSeparator = "~"

// narasininjaProvider membungkus scraper narasininja.net sebagai salah satu
// Provider.
type narasininjaProvider struct{}

func (narasininjaProvider) Name() string { return "narasininja" }

func (narasininjaProvider) CheckConnection() bool {
	_, err := net.DialTimeout("tcp", "narasininja.net:443", time.Second)
	return err == nil
}

// OwnsChapter menandai slug chapter narasininja, yang selalu berisi pemisah
// komik dan chapter.
func (narasininjaProvider) OwnsChapter(chapter string) bool {
	_, _, ok := strings.Cut(chapter, chapterSlugSeparator)
	return ok
}

// Populer mengambil komik dari slider "Terpopuler Hari Ini" di halaman depan.
func (narasininjaProvider) Populer() ([]model_komik.Komik, error) {
	return fetchNaraCards(NarasininjaURL, ".popularslider")
}

// Search mencari komik lewat halaman pencarian provider.
func (narasininjaProvider) Search(keyword string) ([]model_komik.Komik, error) {
	return fetchNaraCards(NarasininjaURL+"search?s="+url.QueryEscape(keyword), ".listupd")
}

// fetchNaraCards membaca kartu komik (.bsx) di dalam container tertentu.
func fetchNaraCards(pageURL, container string) ([]model_komik.Komik, error) {

	var list []model_komik.Komik

	cly := base.Clone()

	cly.OnHTML(container, func(e *colly.HTMLElement) {
		e.ForEach(".bsx", func(i int, el *colly.HTMLElement) {
			link := el.ChildAttr("a", "href")

			slug, ok := slugFromURL(link)
			if !ok {
				return
			}

			list = append(list, model_komik.Komik{
				Title:  el.ChildAttr("a", "title"),
				ImgUrl: el.ChildAttr("img", "src"),
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

// Komik mengambil info komik beserta daftar chapter-nya dari provider.
func (narasininjaProvider) Komik(slugKomik string) (model_komik.Komik, error) {

	komik := model_komik.Komik{Slug: slugKomik}

	cly := base.Clone()

	cly.OnHTML(".seriestucon", func(e *colly.HTMLElement) {
		komik.Title = strings.Join(strings.Fields(e.ChildText(".seriestuheader .entry-title")), " ")
		komik.ImgUrl = e.ChildAttr(".seriestucontl .thumb img", "src")
		komik.Description = strings.Join(strings.Fields(e.ChildText(".seriestucontentr .entry-content")), " ")

		e.ForEach(".infotable tr", func(i int, tr *colly.HTMLElement) {
			if !strings.EqualFold(strings.TrimSpace(tr.ChildText("td:first-child")), "status") {
				return
			}

			if isFinishedStatus(tr.ChildText("td:last-child")) {
				komik.Status = "Tamat"
			} else {
				komik.Status = "Berjalan"
			}
		})
	})

	cly.OnHTML("div#chapterlist", func(e *colly.HTMLElement) {
		e.ForEach("li .eph-num a", func(i int, el *colly.HTMLElement) {
			// Slug diambil dari URL chapter, bukan dibuat dari judul, supaya
			// bisa dibentuk ulang saat mengambil panel.
			slugChapter, ok := naraChapterSlug(el.Attr("href"))
			if !ok {
				return
			}

			title := strings.Join(strings.Fields(el.ChildText(".chapternum")), " ")
			if title == "" {
				title = strings.Join(strings.Fields(el.Text), " ")
			}

			komik.KomikChapter = append(komik.KomikChapter, model_komik.KomikChapter{
				Title:       title,
				SlugChapter: slugChapter,
			})
		})
	})

	komikURL := NarasininjaURL + "komik/" + slugKomik
	log.Println("Scraping komik:", komikURL)

	if err := cly.Visit(komikURL); err != nil {
		return komik, err
	}

	if komik.Title == "" || len(komik.KomikChapter) == 0 {
		return komik, ErrNotFound
	}

	// Model, cache, dan routine mengharapkan chapter terbaru ada di depan.
	// Urutan di halaman provider tidak bisa dipercaya (daftarnya naik dari
	// chapter lama, tapi ujungnya kadang tidak urut), jadi diurutkan sendiri
	// berdasarkan nomor chapter di judul. Nomor diambil dari judul, bukan URL:
	// provider menulis "Chapter 1053.7" sebagai "chapter-10537", sehingga titik
	// desimalnya hilang di URL.
	sort.SliceStable(komik.KomikChapter, func(i, j int) bool {
		return chapterNumber(komik.KomikChapter[i].Title) >
			chapterNumber(komik.KomikChapter[j].Title)
	})

	markSuccess()
	return komik, nil
}

// Panels mengambil semua gambar panel dari satu chapter.
func (narasininjaProvider) Panels(chapter string) ([]model_komik.KomikPanel, error) {

	var panels []model_komik.KomikPanel

	chapterURL, ok := naraChapterURL(chapter)
	if !ok {
		return nil, ErrNotFound
	}

	cly := base.Clone()

	cly.OnHTML("div#readerarea", func(e *colly.HTMLElement) {
		e.ForEach("img.ts-main-image", func(i int, el *colly.HTMLElement) {
			src := strings.TrimSpace(el.Attr("src"))
			if src == "" {
				src = strings.TrimSpace(el.Attr("data-src"))
			}
			if src == "" {
				return
			}

			panels = append(panels, model_komik.KomikPanel{
				SlugChapter: chapter,
				PanelNumber: len(panels) + 1,
				ImgUrl:      src,
			})
		})
	})

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

// naraChapterSlug mengubah URL chapter provider menjadi SlugChapter.
// https://narasininja.net/komik/one-piece/chapter-1188/ -> one-piece~chapter-1188
func naraChapterSlug(link string) (string, bool) {
	u, err := url.Parse(link)
	if err != nil {
		return "", false
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", false
	}

	komik := parts[len(parts)-2]
	chapter := parts[len(parts)-1]
	if komik == "" || chapter == "" {
		return "", false
	}

	return komik + chapterSlugSeparator + chapter, true
}

// naraChapterURL mengembalikan URL chapter dari SlugChapter. Kebalikan dari
// naraChapterSlug.
func naraChapterURL(slugChapter string) (string, bool) {
	komik, chapter, ok := strings.Cut(slugChapter, chapterSlugSeparator)
	if !ok || komik == "" || chapter == "" {
		return "", false
	}

	return NarasininjaURL + path.Join("komik", komik, chapter), true
}

// isFinishedStatus memetakan status provider ("Ongoing", "Ended", dll) ke
// status yang dipakai model.
func isFinishedStatus(status string) bool {
	status = strings.ToLower(status)
	return strings.Contains(status, "end") ||
		strings.Contains(status, "tamat") ||
		strings.Contains(status, "complet")
}

// chapterNumber mengambil angka terakhir di judul chapter, mis.
// "Chapter 21.2" -> 21.2. Judul tanpa angka dianggap 0.
func chapterNumber(title string) float64 {
	end := -1
	for i := len(title) - 1; i >= 0; i-- {
		if title[i] >= '0' && title[i] <= '9' {
			end = i
			break
		}
	}
	if end == -1 {
		return 0
	}

	start := end
	for start > 0 && (title[start-1] == '.' || (title[start-1] >= '0' && title[start-1] <= '9')) {
		start--
	}

	number, err := strconv.ParseFloat(title[start:end+1], 64)
	if err != nil {
		return 0
	}

	return number
}
