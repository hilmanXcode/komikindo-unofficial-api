package controllers

import (
	"sync"
	"time"
)

// scrapeCache menyimpan hasil scraping sementara di memori, sekaligus memastikan
// satu key hanya diproses satu goroutine pada satu waktu: request lain untuk
// key yang sama menunggu lalu memakai hasilnya, bukan ikut men-scrape.
// ponytail: cache per proses; kalau backend dijalankan lebih dari satu instance, pindahkan ke Redis.
var scrapeCache = struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}{entries: map[string]*cacheEntry{}}

type cacheEntry struct {
	mu      sync.Mutex
	value   any
	expires time.Time
}

// maxCacheEntries: lewat jumlah ini, key yang sudah kedaluwarsa dibuang
// (tiap kata kunci pencarian jadi satu key) supaya memori tidak terus tumbuh.
const maxCacheEntries = 1000

// cached mengembalikan hasil fetch untuk key, memakai hasil yang tersimpan
// selama umurnya belum lewat ttl. Error tidak di-cache.
func cached[T any](key string, ttl time.Duration, fetch func() (T, error)) (T, error) {
	now := time.Now()

	scrapeCache.mu.Lock()
	entry, ok := scrapeCache.entries[key]
	if !ok {
		if len(scrapeCache.entries) >= maxCacheEntries {
			for k, e := range scrapeCache.entries {
				if now.After(e.expires) {
					delete(scrapeCache.entries, k)
				}
			}
		}
		entry = &cacheEntry{}
		scrapeCache.entries[key] = entry
	}
	scrapeCache.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if now.Before(entry.expires) {
		return entry.value.(T), nil
	}

	value, err := fetch()
	if err != nil {
		return value, err
	}

	entry.value = value
	entry.expires = time.Now().Add(ttl)
	return value, nil
}
