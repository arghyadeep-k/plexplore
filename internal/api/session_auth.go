package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"plexplore/internal/store"
)

const sessionCookieName = "plexplore_session"

const authenticatedUserKey contextKey = "authenticated_user"

func LoadCurrentUserFromSession(sessionStore SessionStore, userStore UserStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sessionStore == nil || userStore == nil {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSpace(cookie.Value)

		session, err := sessionStore.GetSession(r.Context(), token)
		if err != nil {
			if errors.Is(err, store.ErrSessionNotFound) {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		user, err := userStore.GetUserByID(r.Context(), session.UserID)
		if err != nil {
			_ = sessionStore.DeleteSession(r.Context(), token)
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), authenticatedUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CurrentUserFromContext(ctx context.Context) (store.User, bool) {
	user, ok := ctx.Value(authenticatedUserKey).(store.User)
	return user, ok
}

func RequireUserSessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUserFromContext(r.Context()); !ok {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireUserSessionAuthHTML(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUserFromContext(r.Context()); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUserFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if !user.IsAdmin {
			writeJSONError(w, http.StatusForbidden, "admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
