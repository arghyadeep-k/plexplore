package api

import "net/http"

const defaultCSPPolicy = "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data: https: http:; connect-src 'self'; font-src 'self'"

func setCommonSecurityHeaders(w http.ResponseWriter) {
	headers := w.Header()
	// HSTS is intentionally not set in-app today.
	// Production HSTS is owned by the HTTPS reverse proxy layer.
	headers.Set("X-Frame-Options", "DENY")
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	headers.Set("Cross-Origin-Opener-Policy", "same-origin")
	headers.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
}

func setHTMLSecurityHeaders(w http.ResponseWriter) {
	setCommonSecurityHeaders(w)
	w.Header().Set("Content-Security-Policy", defaultCSPPolicy)
}
