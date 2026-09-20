package middleware

import (
	"komikindo-scraper/helpers"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// client holds the rate limiter instance and the last time it was active
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPStore manages thread-safe access to your client mapping
type IPStore struct {
	mu      sync.Mutex
	clients map[string]*client
}

func newIPStore() *IPStore {
	store := &IPStore{clients: make(map[string]*client)}
	go store.cleanup()

	return store
}

func (s *IPStore) cleanup() {

	for {
		time.Sleep(1 * time.Minute)
		s.mu.Lock()

		for ip, client := range s.clients {
			if time.Since(client.lastSeen) > 3*time.Minute {
				delete(s.clients, ip)
			}
		}
		s.mu.Unlock()
	}

}

func (s *IPStore) limiterFor(ip string, r rate.Limit, b int) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, exists := s.clients[ip]
	if !exists {
		// rate.NewLimiter takes tokens per second (r) and burst size (b)
		val = &client{limiter: rate.NewLimiter(r, b)}
		s.clients[ip] = val
	}
	val.lastSeen = time.Now()

	return val.limiter
}

// RateLimiter enforces a request frequency cap per client IP.
// Setiap pemanggilan menghasilkan store sendiri, jadi beberapa rate limiter
// dengan batas berbeda bisa dipasang pada grup route yang berbeda tanpa saling
// menimpa.
func RateLimiter(r rate.Limit, b int) gin.HandlerFunc {
	store := newIPStore()

	return func(c *gin.Context) {
		if !store.limiterFor(c.ClientIP(), r, b).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, helpers.APIResponse(
				http.StatusTooManyRequests,
				false,
				"Too many requests.",
				nil,
			))
			return
		}

		c.Next()
	}
}
