package scraper

import (
	"errors"
	"fmt"
	model_komik "komikindo-scraper/model/komik"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Fixture potongan HTML narasininja.net. Kalau provider mengubah strukturnya,
// ambil ulang halamannya dan perbarui fixture ini.
const naraCard = `<div class="bsx">
  <a href="%s/komik/one-piece" title="One Piece">
    <div class="limit"><img src="%s/storage/comic/image/one-piece.jpg"></div>
  </a>
</div>`

const naraComicPage = `<html><body>
<div class="seriestucon">
  <div class="seriestuheader"><h1 class="entry-title">One Piece</h1></div>
  <div class="seriestucont">
    <div class="seriestucontl"><div class="thumb"><img src="%s/storage/comic/image/one-piece.jpg"></div></div>
    <div class="seriestucontr">
      <div class="seriestucontentr"><div class="entry-content"><p>
        Seorang bajak laut mencari harta karun legendaris.
      </p></div></div>
      <table class="infotable"><tbody>
        <tr><td>Status</td><td>Ongoing</td></tr>
        <tr><td>Type</td><td>Manga</td></tr>
      </tbody></table>
    </div>
  </div>
</div>
<div class="eplister" id="chapterlist"><ul class="clstyle">
  <li data-num="1"><div class="chbox"><div class="eph-num">
    <a href="%s/komik/one-piece/chapter-1/"><span class="chapternum">Chapter 1</span></a>
    <span class="chapterdate">July 13, 2026</span>
  </div></div></li>
  <li data-num="1194"><div class="chbox"><div class="eph-num">
    <a href="%s/komik/one-piece/chapter-1194/"><span class="chapternum">Chapter 1194</span></a>
    <span class="chapterdate">July 13, 2026</span>
  </div></div></li>
  <li data-num="1188"><div class="chbox"><div class="eph-num">
    <a href="%s/komik/one-piece/chapter-1188/"><span class="chapternum">Chapter 1188</span></a>
    <span class="chapterdate">July 13, 2026</span>
  </div></div></li>
</ul></div>
</body></html>`

const naraReaderPage = `<html><body><div id="readerarea">
<div class="container">
<img class="ts-main-image" src="https://yuucdn.com/img/one-piece/1188/1.jpg">
<img class="ts-main-image" src="https://yuucdn.com/img/one-piece/1188/2.jpg">
</div></div></body></html>`

func TestNarasininjaFetch(t *testing.T) {
	SetProvider(NewProvider("narasininja"))
	t.Cleanup(func() { SetProvider(NewProvider("komikindo")) })

	var base string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/komik/one-piece":
			fmt.Fprintf(w, naraComicPage, base, base, base, base)
		case "/komik/one-piece/chapter-1194":
			w.Write([]byte(naraReaderPage))
		case "/komik/one-piece/chapter-1188":
			w.Write([]byte(naraReaderPage))
		case "/search":
			if r.URL.Query().Get("s") == "one" {
				fmt.Fprintf(w, `<html><body><div class="listupd">`+naraCard+`</div></body></html>`, base, base)
			} else {
				w.Write([]byte(`<html><body><div class="listupd"></div></body></html>`))
			}
		case "/":
			fmt.Fprintf(w, `<html><body><div class="popularslider">`+naraCard+`</div></body></html>`, base, base)
		default:
			w.Write([]byte("<html><body>kosong</body></html>"))
		}
	}))
	defer srv.Close()

	base = srv.URL
	NarasininjaURL = srv.URL + "/"
	t.Cleanup(func() { NarasininjaURL = "https://narasininja.net/" })

	komik, err := FetchKomik("one-piece")
	if err != nil {
		t.Fatal(err)
	}
	if komik.Title != "One Piece" {
		t.Errorf("title = %q", komik.Title)
	}
	if komik.Status != "Berjalan" {
		t.Errorf("status = %q, mau Berjalan", komik.Status)
	}
	if komik.ImgUrl != base+"/storage/comic/image/one-piece.jpg" {
		t.Errorf("imgurl = %q", komik.ImgUrl)
	}
	// Chapter terbaru harus di depan walau urutan provider tidak beraturan, dan
	// slug-nya harus menyimpan komik + chapter supaya panelnya bisa dibuka
	// lewat parameter chapter API.
	if len(komik.KomikChapter) != 3 {
		t.Fatalf("chapter = %+v", komik.KomikChapter)
	}
	if komik.KomikChapter[0].SlugChapter != "one-piece~chapter-1194" ||
		komik.KomikChapter[2].SlugChapter != "one-piece~chapter-1" {
		t.Errorf("slug chapter terbaru = %q", komik.KomikChapter[0].SlugChapter)
	}

	panels, err := FetchPanels(komik.KomikChapter[0].SlugChapter)
	if err != nil {
		t.Fatal(err)
	}
	if len(panels) != 2 || panels[1].PanelNumber != 2 ||
		panels[1].ImgUrl != "https://yuucdn.com/img/one-piece/1188/2.jpg" {
		t.Errorf("panel = %+v", panels)
	}

	for name, fetch := range map[string]func() ([]model_komik.Komik, error){
		"populer": FetchPopuler,
		"search":  func() ([]model_komik.Komik, error) { return Search("one") },
	} {
		list, err := fetch()
		if err != nil {
			t.Fatal(name, err)
		}
		if len(list) != 1 || list[0].Slug != "one-piece" ||
			list[0].Title != "One Piece" ||
			list[0].ImgUrl != base+"/storage/comic/image/one-piece.jpg" {
			t.Errorf("%s = %+v", name, list)
		}
	}

	if _, err := Search("tidak-ada"); !errors.Is(err, ErrNotFound) {
		t.Errorf("pencarian kosong: err = %v, mau ErrNotFound", err)
	}
}

func TestChapterNumber(t *testing.T) {
	cases := map[string]float64{
		"Chapter 1194":    1194,
		"Chapter 1053.7":  1053.7,
		"Chapter 2 End":   2,
		"Chapter Spesial": 0,
	}

	for title, want := range cases {
		if got := chapterNumber(title); got != want {
			t.Errorf("chapterNumber(%q) = %v, mau %v", title, got, want)
		}
	}
}
