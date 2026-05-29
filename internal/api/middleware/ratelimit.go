package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────────────────────────────────────
// Rate limit configuration
// ─────────────────────────────────────────────────────────────────────────────

// RateLimitConfig holds thresholds for the various rate-limiting tiers.
type RateLimitConfig struct {
	// Global limits (all requests, regardless of user)
	GlobalRPM int // requests per minute across all clients

	// Per-IP limits (unauthenticated or fallback)
	IPRequestsPerMinute int

	// Per-user limits (authenticated requests)
	UserRequestsPerMinute int

	// Per-provider limits (sync / power operations per hypervisor per minute)
	ProviderOpsPerMinute int

	// Burst multiplier — allows short bursts above the per-minute rate
	// e.g. 2 means a client can burst to 2× the per-minute rate for one window
	BurstMultiplier int
}

// DefaultRateLimitConfig returns production-safe defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GlobalRPM:             6000, // 100 req/s across all clients
		IPRequestsPerMinute:   300,  // 5 req/s per IP
		UserRequestsPerMinute: 600,  // 10 req/s per authenticated user
		ProviderOpsPerMinute:  30,   // 1 provider op every 2s per hypervisor
		BurstMultiplier:       2,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Redis-backed sliding window rate limiter
// ─────────────────────────────────────────────────────────────────────────────

// redisRateLimiter implements a sliding-window counter using Redis INCR + EXPIRE.
// It is safe for concurrent use and works across multiple server instances.
type redisRateLimiter struct {
	client *redis.Client
}

// Allow checks whether the key is within the limit for the given window.
// Returns (allowed, current count, reset time, error).
func (r *redisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
	now := time.Now()
	windowKey := fmt.Sprintf("rl:%s:%d", key, now.Truncate(window).Unix())
	resetAt := now.Truncate(window).Add(window)

	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, windowKey)
	pipe.Expire(ctx, windowKey, window+time.Second) // +1s grace to avoid race on expiry
	if _, err := pipe.Exec(ctx); err != nil {
		// Redis unavailable — fail open (allow the request)
		return true, 0, resetAt, err
	}

	count := int(incr.Val())
	return count <= limit, count, resetAt, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// In-memory fallback rate limiter (used when Redis is unavailable)
// ─────────────────────────────────────────────────────────────────────────────

type memBucket struct {
	count    int
	resetAt  time.Time
}

type memRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*memBucket
}

func newMemRateLimiter() *memRateLimiter {
	m := &memRateLimiter{buckets: make(map[string]*memBucket)}
	// Periodic cleanup to prevent unbounded growth
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			m.cleanup()
		}
	}()
	return m
}

func (m *memRateLimiter) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, b := range m.buckets {
		if now.After(b.resetAt) {
			delete(m.buckets, k)
		}
	}
}

func (m *memRateLimiter) Allow(key string, limit int, window time.Duration) (bool, int, time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	b, ok := m.buckets[key]
	if !ok || now.After(b.resetAt) {
		b = &memBucket{count: 0, resetAt: now.Add(window)}
		m.buckets[key] = b
	}
	b.count++
	return b.count <= limit, b.count, b.resetAt
}

// ─────────────────────────────────────────────────────────────────────────────
// Middleware constructors
// ─────────────────────────────────────────────────────────────────────────────

// GlobalRateLimit applies a global request-per-minute cap across all clients.
// Uses an in-memory counter (single-instance) — for multi-instance deployments
// replace with the Redis-backed limiter.
func GlobalRateLimit(rpm int) gin.HandlerFunc {
	mem := newMemRateLimiter()
	return func(c *gin.Context) {
		allowed, count, resetAt := mem.Allow("global", rpm, time.Minute)
		c.Header("X-RateLimit-Limit", strconv.Itoa(rpm))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, rpm-count)))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(time.Until(resetAt).Seconds())+1))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "global rate limit exceeded",
				"retry_after": int(time.Until(resetAt).Seconds()) + 1,
			})
			return
		}
		c.Next()
	}
}

// IPRateLimit limits requests per IP address per minute.
func IPRateLimit(rpm int) gin.HandlerFunc {
	mem := newMemRateLimiter()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		allowed, count, resetAt := mem.Allow("ip:"+ip, rpm, time.Minute)
		c.Header("X-RateLimit-Limit", strconv.Itoa(rpm))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, rpm-count)))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		if !allowed {
			retryAfter := int(time.Until(resetAt).Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}
		c.Next()
	}
}

// UserRateLimit limits requests per authenticated user per minute.
// Must be placed after the Auth middleware so the user ID is available.
func UserRateLimit(rpm int) gin.HandlerFunc {
	mem := newMemRateLimiter()
	return func(c *gin.Context) {
		userID := GetCurrentUserID(c)
		if userID == "" {
			// Not authenticated — fall through (IPRateLimit handles unauthenticated)
			c.Next()
			return
		}
		allowed, count, resetAt := mem.Allow("user:"+userID, rpm, time.Minute)
		c.Header("X-RateLimit-Limit", strconv.Itoa(rpm))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, rpm-count)))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		if !allowed {
			retryAfter := int(time.Until(resetAt).Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "user rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}
		c.Next()
	}
}

// ProviderRateLimit limits mutating operations per hypervisor per minute.
// Apply to routes that accept a hypervisor :id path parameter and trigger
// provider API calls (sync, test-connection, etc.).
func ProviderRateLimit(rpm int) gin.HandlerFunc {
	mem := newMemRateLimiter()
	return func(c *gin.Context) {
		hypervisorID := c.Param("id")
		if hypervisorID == "" {
			c.Next()
			return
		}
		key := "provider:" + hypervisorID
		allowed, count, resetAt := mem.Allow(key, rpm, time.Minute)
		c.Header("X-RateLimit-Limit", strconv.Itoa(rpm))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, rpm-count)))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		if !allowed {
			retryAfter := int(time.Until(resetAt).Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "provider rate limit exceeded — too many operations on this hypervisor",
				"retry_after": retryAfter,
			})
			return
		}
		c.Next()
	}
}

// RedisRateLimit is a Redis-backed per-key rate limiter for multi-instance deployments.
// key is a function that extracts the rate-limit key from the request context.
func RedisRateLimit(client *redis.Client, limit int, window time.Duration, keyFn func(*gin.Context) string) gin.HandlerFunc {
	rl := &redisRateLimiter{client: client}
	mem := newMemRateLimiter() // fallback when Redis is unavailable
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			c.Next()
			return
		}

		allowed, count, resetAt, err := rl.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			// Redis unavailable — fall back to in-memory
			allowed, count, resetAt = mem.Allow(key, limit, window)
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, limit-count)))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		if !allowed {
			retryAfter := int(time.Until(resetAt).Seconds()) + 1
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": retryAfter,
			})
			return
		}
		c.Next()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Circuit breaker
// ─────────────────────────────────────────────────────────────────────────────

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // normal operation
	CircuitOpen                         // failing — reject requests
	CircuitHalfOpen                     // testing recovery
)

// CircuitBreaker is a simple per-key circuit breaker.
// It opens after maxFailures consecutive failures within the window,
// and attempts recovery after resetTimeout.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failures     int
	lastFailure  time.Time
	maxFailures  int
	resetTimeout time.Duration
}

// NewCircuitBreaker creates a circuit breaker with the given thresholds.
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
	}
}

// Allow returns true if the request should be allowed through.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return true
}

// RecordSuccess resets the circuit breaker on a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = CircuitClosed
}

// RecordFailure increments the failure counter and may open the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState { return cb.state }

// circuitBreakerRegistry holds per-hypervisor circuit breakers.
var circuitBreakerRegistry = struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}{
	breakers: make(map[string]*CircuitBreaker),
}

// GetCircuitBreaker returns (or creates) a circuit breaker for a hypervisor.
func GetCircuitBreaker(hypervisorID string) *CircuitBreaker {
	circuitBreakerRegistry.mu.RLock()
	cb, ok := circuitBreakerRegistry.breakers[hypervisorID]
	circuitBreakerRegistry.mu.RUnlock()
	if ok {
		return cb
	}
	circuitBreakerRegistry.mu.Lock()
	defer circuitBreakerRegistry.mu.Unlock()
	// Double-check after acquiring write lock
	if cb, ok = circuitBreakerRegistry.breakers[hypervisorID]; ok {
		return cb
	}
	cb = NewCircuitBreaker(5, 60*time.Second) // open after 5 failures, reset after 60s
	circuitBreakerRegistry.breakers[hypervisorID] = cb
	return cb
}

// ProviderCircuitBreaker rejects requests to a hypervisor when its circuit is open.
// Apply to routes that trigger provider API calls.
func ProviderCircuitBreaker() gin.HandlerFunc {
	return func(c *gin.Context) {
		hypervisorID := c.Param("id")
		if hypervisorID == "" {
			c.Next()
			return
		}
		cb := GetCircuitBreaker(hypervisorID)
		if !cb.Allow() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":       "provider circuit breaker open — too many recent failures, please retry later",
				"retry_after": 60,
			})
			return
		}
		c.Next()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Request deduplication
// ─────────────────────────────────────────────────────────────────────────────

// DeduplicateSync prevents duplicate inventory sync requests for the same
// hypervisor within a short window. Returns 409 Conflict if a sync is already
// in-flight for the hypervisor.
func DeduplicateSync() gin.HandlerFunc {
	type entry struct {
		at time.Time
	}
	var (
		mu      sync.Mutex
		inflight = make(map[string]entry)
	)
	const window = 10 * time.Second

	return func(c *gin.Context) {
		hypervisorID := c.Param("id")
		if hypervisorID == "" {
			c.Next()
			return
		}

		mu.Lock()
		e, exists := inflight[hypervisorID]
		if exists && time.Since(e.at) < window {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error":       "a sync is already in progress for this hypervisor — please wait before triggering another",
				"retry_after": int(window.Seconds()),
			})
			return
		}
		inflight[hypervisorID] = entry{at: time.Now()}
		mu.Unlock()

		// Remove the in-flight marker after the window expires regardless of outcome.
		defer func() {
			time.AfterFunc(window, func() {
				mu.Lock()
				delete(inflight, hypervisorID)
				mu.Unlock()
			})
		}()

		c.Next()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
