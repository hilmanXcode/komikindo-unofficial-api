package middleware

import (
	"fmt"
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
	mu      sync.RWMutex
	clients map[string]*client
}

var store = IPStore{
	clients: make(map[string]*client),
}

func CleanupIps() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			store.mu.Lock()

			fmt.Println("Menjalankan cleanup IP")
			for ip, client := range store.clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(store.clients, ip)
				}
			}
			store.mu.Unlock()
		}
	}()
}

// RateLimiter enforces a request frequency cap per client IP
func RateLimiter(r rate.Limit, b int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		store.mu.Lock()
		val, exists := store.clients[ip]
		if !exists {
			// rate.NewLimiter takes tokens per second (r) and burst size (b)
			val = &client{limiter: rate.NewLimiter(r, b)}
			store.clients[ip] = val
		}
		val.lastSeen = time.Now()
		store.mu.Unlock()

		// If the token bucket is empty, reject the request immediately
		if !val.limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests.",
			})
			return
		}

		c.Next()
	}
}
