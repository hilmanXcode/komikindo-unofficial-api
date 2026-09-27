package controllers

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCached(t *testing.T) {
	var calls atomic.Int32
	fetch := func() (int, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return 42, nil
	}

	// Request paralel untuk key yang sama hanya memicu satu fetch.
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, err := cached("test:paralel", time.Minute, fetch); v != 42 || err != nil {
				t.Errorf("cached = %v, %v", v, err)
			}
		}()
	}
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("fetch dipanggil %d kali, mau 1", calls.Load())
	}

	// Setelah kedaluwarsa, fetch dipanggil lagi.
	cached("test:ttl", time.Millisecond, fetch)
	time.Sleep(5 * time.Millisecond)
	cached("test:ttl", time.Millisecond, fetch)
	if calls.Load() != 3 {
		t.Fatalf("fetch dipanggil %d kali, mau 3", calls.Load())
	}

	// Error tidak di-cache: provider yang sempat down dicoba lagi.
	fail := true
	flaky := func() (int, error) {
		if fail {
			return 0, errors.New("down")
		}
		return 7, nil
	}
	if _, err := cached("test:error", time.Minute, flaky); err == nil {
		t.Fatal("mau error")
	}
	fail = false
	if v, err := cached("test:error", time.Minute, flaky); v != 7 || err != nil {
		t.Fatalf("setelah pulih = %v, %v", v, err)
	}
}
