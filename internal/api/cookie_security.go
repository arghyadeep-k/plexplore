package api

import (
	"net/http"
	"strings"
)

const (
	cookieSecureAuto   = "auto"
	cookieSecureAlways = "always"
	cookieSecureNever  = "never"
)

type CookieSecurityPolicy struct {
	SecureMode        string
	TrustProxyHeaders bool
}

func (p CookieSecurityPolicy) CookieSecure(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(p.SecureMode)) {
	case cookieSecureAlways:
		return true
	case cookieSecureNever:
		return false
	default:
		return requestLooksHTTPS(r, p.TrustProxyHeaders)
	}
}

func requestLooksHTTPS(r *http.Request, trustProxyHeaders bool) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if !trustProxyHeaders || r == nil {
		return false
	}
	forwardedProto := strings.TrimSpace(strings.ToLower(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]))
	return forwardedProto == "https"
}
