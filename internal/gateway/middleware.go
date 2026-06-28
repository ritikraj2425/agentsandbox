package gateway

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// AuthMiddleware enforces token-based authentication.
func AuthMiddleware(validToken string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Unauthorized: Invalid Authorization format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if token != validToken {
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// RateLimiter manages request quotas per client IP.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientLimiter
	r       rate.Limit
	b       int
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates an IP-based token bucket rate limiter.
// r is the allowed events per second, b is the burst size.
func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*clientLimiter),
		r:       r,
		b:       b,
	}

	// Start background cleanup for stale clients
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			rl.mu.Lock()
			for ip, client := range rl.clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

// getLimiter returns the token bucket for a specific IP.
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.r, rl.b)
		rl.clients[ip] = &clientLimiter{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	client.lastSeen = time.Now()
	return client.limiter
}

// Middleware returns an HTTP handler that enforces the rate limit per IP.
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract IP
		ip := r.RemoteAddr
		if colon := strings.LastIndex(ip, ":"); colon != -1 {
			ip = ip[:colon]
		}
		// Trust X-Forwarded-For if behind a proxy
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = strings.Split(xff, ",")[0]
		}

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	}
}
