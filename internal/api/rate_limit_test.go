package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFixedWindowLimiter_AllowsThenLimitsThenResets(t *testing.T) {
	now := time.Date(2026, 4, 24, 3, 0, 0, 0, time.UTC)
	limiter := NewFixedWindowLimiterForTest(2, time.Minute, func() time.Time { return now })

	if allowed, _ := limiter.Allow("k1"); !allowed {
		t.Fatal("expected first request allowed")
	}
	if allowed, _ := limiter.Allow("k1"); !allowed {
		t.Fatal("expected second request allowed")
	}
	if allowed, retryAfter := limiter.Allow("k1"); allowed {
		t.Fatal("expected third request limited")
	} else if retryAfter <= 0 {
		t.Fatalf("expected positive retry-after, got %s", retryAfter)
	}

	now = now.Add(61 * time.Second)
	if allowed, _ := limiter.Allow("k1"); !allowed {
		t.Fatal("expected request allowed after window reset")
	}
}

func TestRateLimitKeyForRequest_ProxyTrustBehavior(t *testing.T) {
	reqA := httptest.NewRequest(http.MethodPost, "/login", nil)
	reqA.RemoteAddr = "10.0.0.1:12345"
	reqA.Header.Set("X-Forwarded-For", "203.0.113.10")
	reqB := httptest.NewRequest(http.MethodPost, "/login", nil)
	reqB.RemoteAddr = "10.0.0.2:12345"
	reqB.Header.Set("X-Forwarded-For", "203.0.113.10")

	keyAUntrusted := rateLimitKeyForRequest(reqA, false, "login")
	keyBUntrusted := rateLimitKeyForRequest(reqB, false, "login")
	if keyAUntrusted == keyBUntrusted {
		t.Fatalf("expected different keys without trusted proxy headers, got %q", keyAUntrusted)
	}

	keyATrusted := rateLimitKeyForRequest(reqA, true, "login")
	keyBTrusted := rateLimitKeyForRequest(reqB, true, "login")
	if keyATrusted != keyBTrusted {
		t.Fatalf("expected same key with trusted proxy headers, got %q vs %q", keyATrusted, keyBTrusted)
	}
}

func TestRateLimit_NonSensitiveHealthRouteUnaffected(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		RateLimiters: RateLimiters{
			Login:          NewFixedWindowLimiter(1, time.Minute),
			AdminSensitive: NewFixedWindowLimiter(1, time.Minute),
		},
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "198.51.100.200:1234"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected /health unaffected by limiter, got %d on iteration %d", rec.Code, i)
		}
	}
}
