package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const csrfCookieName = "plexplore_csrf"

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(csrfCookieName); err == nil {
		token := strings.TrimSpace(cookie.Value)
		if token != "" {
			return token
		}
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return ""
	}
	token := hex.EncodeToString(tokenBytes)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	return token
}

func csrfTokenFromRequest(r *http.Request) string {
	headerToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	if headerToken != "" {
		return headerToken
	}
	return strings.TrimSpace(r.FormValue("csrf_token"))
}

func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	cookieToken := strings.TrimSpace(cookie.Value)
	requestToken := csrfTokenFromRequest(r)
	return cookieToken != "" && requestToken != "" && cookieToken == requestToken
}
