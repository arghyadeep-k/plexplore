package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieSecurityPolicy_AutoHTTPIsNotSecure(t *testing.T) {
	policy := CookieSecurityPolicy{SecureMode: cookieSecureAuto}
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	if policy.CookieSecure(req) {
		t.Fatalf("expected auto mode to be non-secure on plain HTTP")
	}
}

func TestCookieSecurityPolicy_AutoTLSIsSecure(t *testing.T) {
	policy := CookieSecurityPolicy{SecureMode: cookieSecureAuto}
	req := httptest.NewRequest(http.MethodGet, "https://example.test/login", nil)
	req.TLS = &tls.ConnectionState{}
	if !policy.CookieSecure(req) {
		t.Fatalf("expected auto mode to be secure on direct TLS requests")
	}
}

func TestCookieSecurityPolicy_AutoTrustedForwardedProtoHTTPS(t *testing.T) {
	policy := CookieSecurityPolicy{
		SecureMode:        cookieSecureAuto,
		TrustProxyHeaders: true,
	}
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if !policy.CookieSecure(req) {
		t.Fatalf("expected auto mode to trust X-Forwarded-Proto=https only when enabled")
	}
}

func TestCookieSecurityPolicy_AutoUntrustedForwardedProtoHTTPS(t *testing.T) {
	policy := CookieSecurityPolicy{
		SecureMode:        cookieSecureAuto,
		TrustProxyHeaders: false,
	}
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if policy.CookieSecure(req) {
		t.Fatalf("expected auto mode to ignore forwarded proto when trust is disabled")
	}
}

func TestCookieSecurityPolicy_AlwaysAndNeverModes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/login", nil)

	if !(CookieSecurityPolicy{SecureMode: cookieSecureAlways}).CookieSecure(req) {
		t.Fatalf("expected always mode to force secure cookies")
	}
	if (CookieSecurityPolicy{SecureMode: cookieSecureNever}).CookieSecure(req) {
		t.Fatalf("expected never mode to disable secure cookies")
	}
}
