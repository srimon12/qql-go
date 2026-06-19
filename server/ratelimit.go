package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"connectrpc.com/connect"
)

// RateLimiter implements per-key token bucket rate limiting.
// Each key (typically a JWT subject or tenant ID) gets its own bucket
// with independent refill and capacity.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens per second
	capacity int     // max burst
	enabled  bool
}

type bucket struct {
	tokens   float64
	lastFill time.Time
}

// RateLimitConfig configures the rate limiter.
type RateLimitConfig struct {
	// Rate is tokens per second per key.
	Rate float64
	// Capacity is the maximum burst size per key.
	Capacity int
	// Enabled turns on rate limiting.
	Enabled bool
}

// NewRateLimiter creates a rate limiter. If not enabled, all calls pass through.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	if !cfg.Enabled {
		return &RateLimiter{enabled: false}
	}
	if cfg.Rate <= 0 {
		cfg.Rate = 10 // default: 10 req/s
	}
	if cfg.Capacity <= 0 {
		cfg.Capacity = 20 // default: burst of 20
	}
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     cfg.Rate,
		capacity: cfg.Capacity,
		enabled:  true,
	}
	go rl.cleanup()
	return rl
}

// Allow checks if a request from the given key should be allowed.
func (rl *RateLimiter) Allow(key string) bool {
	if !rl.enabled {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:   float64(rl.capacity) - 1,
			lastFill: time.Now(),
		}
		rl.buckets[key] = b
		return true
	}

	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.capacity) {
		b.tokens = float64(rl.capacity)
	}
	b.lastFill = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// RetryAfter returns how long the key should wait before retrying.
func (rl *RateLimiter) RetryAfter(key string) time.Duration {
	if !rl.enabled {
		return 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists || b.tokens >= 1 {
		return 0
	}

	deficit := 1 - b.tokens
	seconds := deficit / rl.rate
	return time.Duration(seconds * float64(time.Second))
}

// cleanup removes stale buckets every 5 minutes.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.buckets {
			if now.Sub(b.lastFill) > 10*time.Minute {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// rateLimitInterceptor returns a connect interceptor that enforces rate limits.
func rateLimitInterceptor(rl *RateLimiter) connect.Interceptor {
	return &rateLimitInt{limiter: rl}
}

type rateLimitInt struct {
	limiter *RateLimiter
}

func (r *rateLimitInt) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		key := "anonymous"
		if claims := ExtractClaimsFromContext(ctx); claims != nil && claims.Subject != "" {
			key = claims.Subject
		}

		if !r.limiter.Allow(key) {
			retryAfter := r.limiter.RetryAfter(key)
			fmt.Fprintf(os.Stderr, "rate limit: %s blocked (retry after %v)\n", key, retryAfter)
			return nil, connect.NewError(
				connect.CodeResourceExhausted,
				fmt.Errorf("rate limit exceeded, retry after %v", retryAfter),
			)
		}

		return next(ctx, req)
	}
}

func (r *rateLimitInt) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (r *rateLimitInt) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// RateLimitHTTPMiddleware is an HTTP middleware variant for the health endpoint.
func RateLimitHTTPMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if !rl.Allow(key) {
				retryAfter := rl.RetryAfter(key)
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
