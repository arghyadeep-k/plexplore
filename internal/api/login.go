package api

import (
	"errors"
	"html"
	"io"
	"net/http"
	"strings"

	"plexplore/internal/store"
)

const loginPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Login</title>
  <style>
    :root {
      --bg: #f4f6f8;
      --text: #1b1f24;
      --card: #ffffff;
      --border: #d7dde5;
      --accent: #0b6bcb;
      --muted: #5a6573;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 15px/1.45 "Segoe UI", Tahoma, sans-serif;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 16px;
    }
    .card {
      width: 100%;
      max-width: 360px;
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 14px;
    }
    h1 {
      margin: 0 0 10px;
      font-size: 22px;
    }
    label { display: block; margin-top: 8px; }
    input, button {
      width: 100%;
      margin-top: 4px;
      padding: 8px 10px;
      border: 1px solid var(--border);
      border-radius: 8px;
      font: inherit;
    }
    button {
      margin-top: 12px;
      border-color: var(--accent);
      background: var(--accent);
      color: #fff;
      cursor: pointer;
    }
    .muted { color: var(--muted); font-size: 13px; }
  </style>
</head>
<body>
  <form class="card" method="post" action="/login">
    <h1>Sign In</h1>
    <input type="hidden" name="csrf_token" value="__CSRF_TOKEN__">
    <label>Email
      <input type="email" name="email" required autocomplete="username">
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

func registerLoginRoutes(mux *http.ServeMux, userStore UserStore, sessionStore SessionStore) {
	mux.HandleFunc("GET /login", loginPageHandler)
	mux.HandleFunc("POST /login", loginHandler(userStore, sessionStore))
	mux.HandleFunc("POST /logout", logoutHandler(sessionStore))
}

func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	csrfToken := ensureCSRFCookie(w, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := strings.ReplaceAll(loginPageHTML, "__CSRF_TOKEN__", html.EscapeString(csrfToken))
	_, _ = io.WriteString(w, page)
}

func loginHandler(userStore UserStore, sessionStore SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid login form")
			return
		}
		if !validateCSRF(r) {
			writeJSONError(w, http.StatusForbidden, "csrf token invalid")
			return
		}
		email := strings.TrimSpace(r.FormValue("email"))
		password := strings.TrimSpace(r.FormValue("password"))
		if email == "" || password == "" {
			writeJSONError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		user, err := userStore.GetUserByEmail(r.Context(), email)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "user lookup failed")
			return
		}
		if !VerifyPassword(user.PasswordHash, password) {
			writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		session, err := sessionStore.CreateSession(r.Context(), user.ID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "session creation failed")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    session.Token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  session.ExpiresAt.UTC(),
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func logoutHandler(sessionStore SessionStore) http.HandlerFunc {
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
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
