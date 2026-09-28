package scraper

import (
	"errors"
	model_komik "komikindo-scraper/model/komik"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Potongan HTML asli komikindo.ch (September 2026). Kalau test ini masih lolos
// tapi scraping di production kosong, berarti struktur HTML provider berubah:
// ambil ulang halamannya dan perbarui fixture ini.
const komikPage = `<html><body>
<div class="infoanime">
  <h1 class="entry-title" itemprop="name">Komik
    HiGH &#038; LOW: THE WORST Housen Gakuen Nikki            </h1>
  <div class="thumb"><img src="https://komikindo.ch/wp-content/uploads/cover.jpg" /></div>
  <div class="infox">
    <div class="spe">
      <span><b>Judul Alternatif:</b> HiGH & LOW THE WORST Hosen Gakuen Diary </span>
      <span><b>Status:</b>
        Tamat                </span>
    </div>
    <div class="shortcsc sht2"><p>
      Manga adalah serial komik yang bertemakan
      School Life.
    </p></div>
  </div>
</div>
<div class="bxcl scrolling" id="chapter_list"><ul>
  <li><span class="lchx">
    <a href="https://komikindo.ch/high-low-the-worst-housen-gakuen-nikki-chapter-2-end/" title="Komik HiGH &#038; LOW: THE WORST Housen Gakuen Nikki Chapter 2 End">Chapter 2 End</a></span></li>
  <li><span class="lchx">
    <a href="https://komikindo.ch/high-low-the-worst-housen-gakuen-nikki-chapter-1/" title="Komik HiGH &#038; LOW: THE WORST Housen Gakuen Nikki Chapter 1">Chapter 1</a></span></li>
</ul></div>
</body></html>`

const chapterPage = `<html><body><div id="chimg-auh">
<img src="https://img.example/1.jpeg" alt="Magic Emperor Chapter 910" onError="this.onerror=null;"/><img src="https://img.example/2.jpeg" alt="Magic Emperor Chapter 910"/>
</div></body></html>`

const card = `<div class="animepost"><div class="animposx">
  <a href="https://komikindo.ch/komik/545921-revenge-of-the-iron-blooded-sword-hound/" itemprop="url" title="Komik Revenge Of The Iron-Blooded Sword Hound" rel="bookmark">
    <div class="limit"><div class="ply"></div>
      <img src="https://komikindo.ch/wp-content/uploads/2023/04/cover-223x319.jpg" itemprop="image" /></div>
  </a>
  <div class="bigors"><div class="tt"><h3><a href="https://komikindo.ch/komik/545921-revenge-of-the-iron-blooded-sword-hound/">Revenge</a></h3></div></div>
</div></div>`

const homePage = `<html><body><div class="odadingslider">` + card + `</div></body></html>`
const searchPage = `<html><body><div class="film-list">` + card + `</div></body></html>`

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/komik/high-low":
			w.Write([]byte(komikPage))
		case "/magic-emperor-chapter-910":
			w.Write([]byte(chapterPage))
		case "/":
			if r.URL.Query().Get("s") == "revenge" {
				w.Write([]byte(searchPage))
			} else if r.URL.Query().Has("s") {
				w.Write([]byte("<html><body><div class=\"film-list\"></div></body></html>"))
			} else {
				w.Write([]byte(homePage))
			}
		default:
			w.Write([]byte("<html><body>kosong</body></html>"))
		}
	}))
	defer srv.Close()
	ProviderURL = srv.URL + "/"

	komik, err := FetchKomik("high-low")
	if err != nil {
		t.Fatal(err)
	}

	if komik.Title != "Komik HiGH & LOW: THE WORST Housen Gakuen Nikki" {
		t.Errorf("title = %q", komik.Title)
	}
	if komik.Status != "Tamat" {
		t.Errorf("status = %q, mau Tamat", komik.Status)
	}
	if komik.ImgUrl != "https://komikindo.ch/wp-content/uploads/cover.jpg" {
		t.Errorf("imgurl = %q", komik.ImgUrl)
	}
	// Slug harus sama dengan URL provider ("high-low"), bukan hasil slugify
	// judul ("high-and-low") yang 404 di provider.
	if len(komik.KomikChapter) != 2 || komik.KomikChapter[1].SlugChapter != "high-low-the-worst-housen-gakuen-nikki-chapter-1" {
		t.Errorf("chapter = %+v", komik.KomikChapter)
	}

	panels, err := FetchPanels("magic-emperor-chapter-910")
	if err != nil {
		t.Fatal(err)
	}
	if len(panels) != 2 || panels[1].PanelNumber != 2 || panels[1].ImgUrl != "https://img.example/2.jpeg" {
		t.Errorf("panel = %+v", panels)
	}

	for name, fetch := range map[string]func() ([]model_komik.Komik, error){
		"populer": FetchPopuler,
		"search":  func() ([]model_komik.Komik, error) { return Search("revenge") },
	} {
		list, err := fetch()
		if err != nil {
			t.Fatal(name, err)
		}
		if len(list) != 1 || list[0].Slug != "545921-revenge-of-the-iron-blooded-sword-hound" ||
			list[0].Title != "Komik Revenge Of The Iron-Blooded Sword Hound" ||
			list[0].ImgUrl != "https://komikindo.ch/wp-content/uploads/2023/04/cover-223x319.jpg" {
			t.Errorf("%s = %+v", name, list)
		}
	}

	// Collector dipakai bersama; URL yang sama harus bisa diambil lagi
	// (cache habis, routine putaran berikutnya).
	for i := range 2 {
		if _, err := FetchPopuler(); err != nil {
			t.Fatalf("populer ke-%d: %v", i+1, err)
		}
		if _, err := FetchKomik("high-low"); err != nil {
			t.Fatalf("komik ke-%d: %v", i+1, err)
		}
	}

	if _, err := Search("tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Errorf("pencarian kosong: err = %v, mau ErrNotFound", err)
	}
	if LastSuccess().IsZero() {
		t.Error("LastSuccess belum terisi setelah scraping berhasil")
	}

	if _, err := FetchKomik("tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Errorf("halaman kosong: err = %v, mau ErrNotFound", err)
	}
	if _, err := FetchPanels("tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Errorf("chapter kosong: err = %v, mau ErrNotFound", err)
	}
}
