package scraper

import (
	"fmt"
	model_komik "komikindo-scraper/model/komik"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestProviderFailoverWhenPrimaryDown memastikan provider cadangan dipakai saat
// provider utama tidak bisa dihubungi.
func TestProviderFailoverWhenPrimaryDown(t *testing.T) {
	t.Cleanup(func() {
		providers = nil
		ProviderURL = "https://komikindo.ch/"
		NarasininjaURL = "https://narasininja.net/"
	})

	// Provider utama disimulasikan down: server ditutup sebelum dipakai.
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	downURL := down.URL
	down.Close()

	var base string
	nara := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/komik/one-piece":
			fmt.Fprintf(w, naraComicPage, base, base, base, base)
		case "/komik/one-piece/chapter-1194":
			w.Write([]byte(naraReaderPage))
		default:
			w.Write([]byte("<html><body>kosong</body></html>"))
		}
	}))
	defer nara.Close()
	base = nara.URL

	ProviderURL = downURL + "/"
	NarasininjaURL = nara.URL + "/"
	SetProviders(NewProviders("komikindo", "narasininja")...)

	if names := strings.Join(ProviderNames(), ","); names != "komikindo,narasininja" {
		t.Fatalf("ProviderNames = %q", names)
	}
	if PrimaryProvider().Name() != "komikindo" {
		t.Fatalf("PrimaryProvider = %q", PrimaryProvider().Name())
	}

	komik, err := FetchKomik("one-piece")
	if err != nil {
		t.Fatalf("FetchKomik harus failover ke narasininja: %v", err)
	}
	if komik.Title != "One Piece" || komik.KomikChapter[0].SlugChapter != "one-piece~chapter-1194" {
		t.Errorf("hasil failover = %+v", komik)
	}

	if _, err := FetchPanels(komik.KomikChapter[0].SlugChapter); err != nil {
		t.Fatalf("FetchPanels harus memakai provider pemilik slug: %v", err)
	}

	if !CheckConnection() {
		t.Error("CheckConnection harus true karena provider cadangan hidup")
	}
}

// TestPanelsRoutedToOwningProvider memastikan panel selalu diambil dari provider
// yang format slug chapter-nya cocok, bukan provider pertama di rantai.
func TestPanelsRoutedToOwningProvider(t *testing.T) {
	t.Cleanup(func() {
		providers = nil
		ProviderURL = "https://komikindo.ch/"
		NarasininjaURL = "https://narasininja.net/"
	})

	var komikindoHits atomic.Int32
	komikindo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		komikindoHits.Add(1)
		w.Write([]byte(chapterPage))
	}))
	defer komikindo.Close()

	nara := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(naraReaderPage))
	}))
	defer nara.Close()

	ProviderURL = komikindo.URL + "/"
	NarasininjaURL = nara.URL + "/"
	SetProviders(NewProviders("komikindo", "narasininja")...)

	// Slug narasininja (mengandung "~") harus dibaca narasininja.
	if _, err := FetchPanels("one-piece~chapter-1194"); err != nil {
		t.Fatal(err)
	}
	if komikindoHits.Load() != 0 {
		t.Errorf("komikindo dikunjungi %d kali untuk slug narasininja", komikindoHits.Load())
	}

	// Slug komikindo dibaca komikindo.
	if _, err := FetchPanels("magic-emperor-chapter-910"); err != nil {
		t.Fatal(err)
	}
	if komikindoHits.Load() != 1 {
		t.Errorf("komikindo dikunjungi %d kali, mau 1", komikindoHits.Load())
	}

	// Provider pemilik chapter terdeteksi dari slug yang tersimpan.
	if p := ProviderForChapters([]model_komik.KomikChapter{{SlugChapter: "magic-emperor-chapter-910"}}); p == nil || p.Name() != "komikindo" {
		t.Errorf("ProviderForChapters(komikindo) = %v", p)
	}
	if p := ProviderForChapters([]model_komik.KomikChapter{{SlugChapter: "one-piece~chapter-1194"}}); p == nil || p.Name() != "narasininja" {
		t.Errorf("ProviderForChapters(narasininja) = %v", p)
	}
}
