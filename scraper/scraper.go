package scraper

import (
	"errors"
	"log"
	"strings"

	model_komik "komikindo-scraper/model/komik"
)

// Provider adalah sumber data manga. Setiap provider mengembalikan model yang
// sama (Komik, KomikChapter, KomikPanel), jadi menambah sumber baru tidak
// mengubah model, database, ataupun bentuk response API.
type Provider interface {
	Name() string
	Populer() ([]model_komik.Komik, error)
	Search(keyword string) ([]model_komik.Komik, error)
	Komik(slug string) (model_komik.Komik, error)
	Panels(chapter string) ([]model_komik.KomikPanel, error)
	CheckConnection() bool
	// OwnsChapter menandai apakah slug chapter berasal dari provider ini,
	// supaya panel diambil dari provider yang benar.
	OwnsChapter(chapter string) bool
}

// providers adalah rantai provider yang dipakai, terurut dari utama ke
// cadangan. Kalau provider utama gagal (down), provider berikutnya dicoba.
var providers []Provider

// SetProviders memasang rantai provider aktif. Daftar kosong diabaikan supaya
// kesalahan konfigurasi tidak mengubah perilaku aplikasi.
func SetProviders(list ...Provider) {
	if cleaned := dedupeProviders(list); len(cleaned) > 0 {
		providers = cleaned
	}
}

// dedupeProviders membuang provider nil dan nama yang duplikat, tanpa
// mengubah urutan.
func dedupeProviders(list []Provider) []Provider {
	cleaned := make([]Provider, 0, len(list))
	seen := map[string]bool{}

	for _, p := range list {
		if p == nil || seen[p.Name()] {
			continue
		}
		seen[p.Name()] = true
		cleaned = append(cleaned, p)
	}

	return cleaned
}

// SetProvider memasang satu provider aktif (tanpa cadangan).
func SetProvider(p Provider) {
	SetProviders(p)
}

// NewProvider membuat provider dari namanya. Nama yang tidak dikenal memakai
// narasininja, provider default.
func NewProvider(name string) Provider {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "komikindo":
		return komikindoProvider{}
	default:
		return narasininjaProvider{}
	}
}

// NewProviders membuat rantai provider dari daftar nama, tanpa duplikat.
func NewProviders(names ...string) []Provider {
	list := make([]Provider, 0, len(names))
	for _, name := range names {
		list = append(list, NewProvider(name))
	}

	if cleaned := dedupeProviders(list); len(cleaned) > 0 {
		return cleaned
	}
	return []Provider{NewProvider("narasininja")}
}

// allProviders mengembalikan rantai provider aktif. Kalau belum diatur (mis.
// dari test), default-nya komikindo supaya test lama tetap berjalan.
func allProviders() []Provider {
	if len(providers) == 0 {
		providers = []Provider{komikindoProvider{}}
	}
	return providers
}

// ProviderNames mengembalikan nama provider pada rantai, terurut.
func ProviderNames() []string {
	list := allProviders()
	names := make([]string, len(list))
	for i, p := range list {
		names[i] = p.Name()
	}
	return names
}

// ProviderName mengembalikan nama provider utama.
func ProviderName() string {
	return allProviders()[0].Name()
}

// PrimaryProvider mengembalikan provider utama tanpa cadangan.
func PrimaryProvider() Provider {
	return allProviders()[0]
}

// firstResult menjalankan fetch pada tiap provider berurutan dan mengembalikan
// hasil provider pertama yang berhasil. Provider yang down (error koneksi)
// maupun yang tidak punya datanya (ErrNotFound) dilewati selama masih ada
// provider lain, sehingga API tetap jalan saat provider utama bermasalah.
func firstResult[T any](order []Provider, fetch func(Provider) (T, error)) (T, error) {
	var value T
	var lastErr error

	for _, p := range order {
		result, err := fetch(p)
		if err == nil {
			return result, nil
		}
		// ErrNotFound bukan masalah provider (komiknya memang tidak ada), jadi
		// hanya kegagalan koneksi yang perlu dilaporkan.
		if !errors.Is(err, ErrNotFound) {
			log.Printf("Provider %s gagal, mencoba cadangan: %v", p.Name(), err)
		}
		lastErr = err
	}

	return value, lastErr
}

// FetchPopuler mengambil komik populer, dengan failover ke provider cadangan.
func FetchPopuler() ([]model_komik.Komik, error) {
	return firstResult(allProviders(), func(p Provider) ([]model_komik.Komik, error) {
		return p.Populer()
	})
}

// Search mencari komik, dengan failover ke provider cadangan.
func Search(keyword string) ([]model_komik.Komik, error) {
	return firstResult(allProviders(), func(p Provider) ([]model_komik.Komik, error) {
		return p.Search(keyword)
	})
}

// FetchKomik mengambil info komik beserta daftar chapter-nya, dengan failover
// ke provider cadangan.
func FetchKomik(slugKomik string) (model_komik.Komik, error) {
	return firstResult(allProviders(), func(p Provider) (model_komik.Komik, error) {
		return p.Komik(slugKomik)
	})
}

// FetchPanels mengambil semua gambar panel dari satu chapter. Provider yang
// memiliki format chapter itu didahulukan supaya Komikindo dan NarasiNinja
// tidak saling menebak URL.
func FetchPanels(chapter string) ([]model_komik.KomikPanel, error) {
	return firstResult(panelsOrder(chapter), func(p Provider) ([]model_komik.KomikPanel, error) {
		return p.Panels(chapter)
	})
}

// panelsOrder menaruh provider yang memiliki slug chapter di depan.
func panelsOrder(chapter string) []Provider {
	list := allProviders()

	var owned []Provider
	for _, p := range list {
		if p.OwnsChapter(chapter) {
			owned = append(owned, p)
		}
	}

	if len(owned) == 0 {
		return list
	}
	return owned
}

// ProviderForChapters mengembalikan provider yang cocok dengan format chapter
// yang sudah tersimpan. Dipakai routine supaya komik yang datanya berasal dari
// satu provider tidak ditimpa chapter dari provider lain saat failover.
func ProviderForChapters(chapters []model_komik.KomikChapter) Provider {
	for _, p := range allProviders() {
		for _, ch := range chapters {
			if p.OwnsChapter(ch.SlugChapter) {
				return p
			}
		}
	}
	return nil
}

// CheckConnection bernilai true kalau minimal satu provider pada rantai bisa
// dijangkau, karena failover membuat API tetap melayani selama ada cadangan.
func CheckConnection() bool {
	for _, p := range allProviders() {
		if p.CheckConnection() {
			return true
		}
	}
	return false
}
