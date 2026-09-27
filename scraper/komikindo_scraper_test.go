package scraper

import (
	"errors"
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

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/komik/high-low":
			w.Write([]byte(komikPage))
		case "/magic-emperor-chapter-910":
			w.Write([]byte(chapterPage))
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

	if _, err := FetchKomik("tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Errorf("halaman kosong: err = %v, mau ErrNotFound", err)
	}
	if _, err := FetchPanels("tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Errorf("chapter kosong: err = %v, mau ErrNotFound", err)
	}
}
