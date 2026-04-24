package api

import (
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"

	"plexplore/internal/store"
)

const loginPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Login</title>
  <link rel="stylesheet" href="/ui/assets/app/app.css">
  <link rel="stylesheet" href="/ui/assets/app/login.css">
</head>
<body class="login-body">
  <form class="card login-wrap login-card" method="post" action="/login">
    <h1>Sign In</h1>
    __ERROR_BLOCK__
    <input type="hidden" name="csrf_token" value="__CSRF_TOKEN__">
    <input type="hidden" name="next" value="__NEXT_VALUE__">
    <label>Email
      <input type="email" name="email" value="__EMAIL_VALUE__" required autocomplete="username">
    </label>
    <label>Password
      <input type="password" name="password" required autocomplete="current-password">
    </label>
    <button type="submit">Sign In</button>
    <p class="muted">Admin-created users only. No public signup.</p>
  </form>
</body>
</html>
`

func registerLoginRoutes(mux *http.ServeMux, userStore UserStore, sessionStore SessionStore, cookiePolicy CookieSecurityPolicy, rateLimiters RateLimiters) {
	mux.HandleFunc("GET /login", loginPageHandler(cookiePolicy))
	mux.HandleFunc("POST /login", loginHandler(userStore, sessionStore, cookiePolicy, rateLimiters))
	mux.HandleFunc("POST /logout", logoutHandler(sessionStore, cookiePolicy))
}

func loginPageHandler(cookiePolicy CookieSecurityPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeLoginPage(w, r, http.StatusOK, "", "", strings.TrimSpace(r.URL.Query().Get("next")), cookiePolicy)
	}
}

func loginHandler(userStore UserStore, sessionStore SessionStore, cookiePolicy CookieSecurityPolicy, rateLimiters RateLimiters) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonRequest := isJSONLoginRequest(r)
		if limiter := rateLimiters.Login; limiter != nil {
			scope := "login"
			limiterKey := rateLimitKeyForRequest(r, rateLimiters.TrustProxyHeaders, scope)
			if allowed, retryAfter := limiter.Allow(limiterKey); !allowed {
				if jsonRequest {
					writeRateLimitedJSON(w, retryAfter, "too many login attempts")
					return
				}
				retrySeconds := int(retryAfter.Seconds())
				if retrySeconds < 1 {
					retrySeconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				enteredEmail := strings.TrimSpace(r.FormValue("email"))
				writeLoginPage(w, r, http.StatusTooManyRequests, enteredEmail, "Too many login attempts. Please try again shortly.", strings.TrimSpace(r.FormValue("next")), cookiePolicy)
				return
			}
		}
		if !validateCSRF(r) {
			if jsonRequest {
				writeJSONError(w, http.StatusForbidden, "csrf token invalid")
				return
			}
			writeLoginPage(w, r, http.StatusForbidden, "", "Session expired. Please try again.", strings.TrimSpace(r.FormValue("next")), cookiePolicy)
			return
		}

		email, password, nextPath, err := parseLoginCredentials(r, jsonRequest)
		if err != nil {
			if jsonRequest {
				writeJSONError(w, http.StatusBadRequest, "invalid login form")
				return
			}
			writeLoginPage(w, r, http.StatusBadRequest, "", "Invalid login request.", strings.TrimSpace(r.URL.Query().Get("next")), cookiePolicy)
			return
		}
		if email == "" || password == "" {
			if jsonRequest {
				writeJSONError(w, http.StatusBadRequest, "email and password are required")
				return
			}
			writeLoginPage(w, r, http.StatusBadRequest, email, "Email and password are required.", nextPath, cookiePolicy)
			return
		}

		user, err := userStore.GetUserByEmail(r.Context(), email)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				if jsonRequest {
					writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
					return
				}
				writeLoginPage(w, r, http.StatusUnauthorized, email, "Invalid email or password", nextPath, cookiePolicy)
				return
			}
			if jsonRequest {
				writeJSONError(w, http.StatusInternalServerError, "user lookup failed")
				return
			}
			writeLoginPage(w, r, http.StatusInternalServerError, email, "Login failed. Please try again.", nextPath, cookiePolicy)
			return
		}
		if !VerifyPassword(user.PasswordHash, password) {
			if jsonRequest {
				writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			writeLoginPage(w, r, http.StatusUnauthorized, email, "Invalid email or password", nextPath, cookiePolicy)
			return
		}

		session, err := sessionStore.CreateSession(r.Context(), user.ID)
		if err != nil {
			if jsonRequest {
				writeJSONError(w, http.StatusInternalServerError, "session creation failed")
				return
			}
			writeLoginPage(w, r, http.StatusInternalServerError, email, "Login failed. Please try again.", nextPath, cookiePolicy)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    session.Token,
			Path:     "/",
			HttpOnly: true,
			Secure:   cookiePolicy.CookieSecure(r),
			SameSite: http.SameSiteLaxMode,
			Expires:  session.ExpiresAt.UTC(),
		})
		http.Redirect(w, r, resolvePostLoginRedirect(nextPath), http.StatusSeeOther)
	}
}

func writeLoginPage(w http.ResponseWriter, r *http.Request, status int, email, errorMessage, nextPath string, cookiePolicy CookieSecurityPolicy) {
	csrfToken := ensureCSRFCookie(w, r, cookiePolicy)
	errorBlock := ""
	if strings.TrimSpace(errorMessage) != "" {
		errorBlock = `<p class="error" role="alert">` + html.EscapeString(errorMessage) + `</p>`
	}
	page := strings.ReplaceAll(loginPageHTML, "__CSRF_TOKEN__", html.EscapeString(csrfToken))
	page = strings.ReplaceAll(page, "__EMAIL_VALUE__", html.EscapeString(strings.TrimSpace(email)))
	page = strings.ReplaceAll(page, "__NEXT_VALUE__", html.EscapeString(strings.TrimSpace(nextPath)))
	page = strings.ReplaceAll(page, "__ERROR_BLOCK__", errorBlock)
	setHTMLSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, page)
}

func parseLoginCredentials(r *http.Request, jsonRequest bool) (string, string, string, error) {
	if jsonRequest {
		var payload struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return "", "", "", err
		}
		return strings.TrimSpace(payload.Email), strings.TrimSpace(payload.Password), strings.TrimSpace(r.URL.Query().Get("next")), nil
	}

	if err := r.ParseForm(); err != nil {
		return "", "", "", err
	}
	return strings.TrimSpace(r.FormValue("email")), strings.TrimSpace(r.FormValue("password")), strings.TrimSpace(r.FormValue("next")), nil
}

func isJSONLoginRequest(r *http.Request) bool {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	return strings.Contains(contentType, "application/json") || strings.Contains(accept, "application/json")
}

func resolvePostLoginRedirect(nextPath string) string {
	candidate := strings.TrimSpace(nextPath)
	if candidate == "" {
		return "/ui/map"
	}
	if !strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "//") {
		return "/ui/map"
	}
	if strings.HasPrefix(candidate, "/login") {
		return "/ui/map"
	}
	return candidate
}

func logoutHandler(sessionStore SessionStore, cookiePolicy CookieSecurityPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid logout form")
			return
		}
		if !validateCSRF(r) {
			writeJSONError(w, http.StatusForbidden, "csrf token invalid")
			return
		}
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			_ = sessionStore.DeleteSession(r.Context(), cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cookiePolicy.CookieSecure(r),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
