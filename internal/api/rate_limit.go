package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimiters struct {
	Login             *FixedWindowLimiter
	AdminSensitive    *FixedWindowLimiter
	TrustProxyHeaders bool
}

type FixedWindowLimiter struct {
	limit  int
	window time.Duration
	nowFn  func() time.Time

	mu      sync.Mutex
	entries map[string]fixedWindowEntry
}

type fixedWindowEntry struct {
	windowStart time.Time
	count       int
}

func NewFixedWindowLimiter(limit int, window time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		limit:   limit,
		window:  window,
		nowFn:   time.Now,
		entries: make(map[string]fixedWindowEntry),
	}
}

func NewFixedWindowLimiterForTest(limit int, window time.Duration, nowFn func() time.Time) *FixedWindowLimiter {
	limiter := NewFixedWindowLimiter(limit, window)
	if nowFn != nil {
		limiter.nowFn = nowFn
	}
	return limiter
}

func (l *FixedWindowLimiter) Allow(key string) (bool, time.Duration) {
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return true, 0
	}
	normalizedKey := strings.TrimSpace(key)
	if normalizedKey == "" {
		normalizedKey = "unknown"
	}

	now := l.nowFn().UTC()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[normalizedKey]
	if !ok || now.Sub(entry.windowStart) >= l.window {
		l.entries[normalizedKey] = fixedWindowEntry{
			windowStart: now,
			count:       1,
		}
		l.maybeCleanup(now)
		return true, 0
	}

	if entry.count >= l.limit {
		retryAfter := l.window - now.Sub(entry.windowStart)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}

	entry.count++
	l.entries[normalizedKey] = entry
	return true, 0
}

func (l *FixedWindowLimiter) maybeCleanup(now time.Time) {
	if len(l.entries) < 2048 {
		return
	}
	for key, entry := range l.entries {
		if now.Sub(entry.windowStart) >= l.window*2 {
			delete(l.entries, key)
		}
	}
}

func rateLimitKeyForRequest(r *http.Request, trustProxyHeaders bool, scope string) string {
	return strings.TrimSpace(scope) + "|" + clientIPForRateLimit(r, trustProxyHeaders)
}

func clientIPForRateLimit(r *http.Request, trustProxyHeaders bool) string {
	if r == nil {
		return "unknown"
	}
	if trustProxyHeaders {
		xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
		if xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if first != "" {
				return first
			}
		}
	}

	remote := strings.TrimSpace(r.RemoteAddr)
	if remote == "" {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(remote)
	if err == nil && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host)
	}
	return remote
}

func writeRateLimitedJSON(w http.ResponseWriter, retryAfter time.Duration, message string) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSONError(w, http.StatusTooManyRequests, message)
}
