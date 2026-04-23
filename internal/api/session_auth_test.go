package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"plexplore/internal/store"
)

type fakeSessionStore struct {
	sessionByToken map[string]store.Session
}

func (f *fakeSessionStore) CreateSession(_ context.Context, userID int64) (store.Session, error) {
	return store.Session{Token: "new-token", UserID: userID, ExpiresAt: time.Now().UTC().Add(time.Hour), CreatedAt: time.Now().UTC()}, nil
}

func (f *fakeSessionStore) GetSession(_ context.Context, token string) (store.Session, error) {
	session, ok := f.sessionByToken[token]
	if !ok {
		return store.Session{}, store.ErrSessionNotFound
	}
	return session, nil
}

func (f *fakeSessionStore) DeleteSession(_ context.Context, _ string) error { return nil }

type fakeUserStore struct {
	users map[int64]store.User
}

func (f *fakeUserStore) GetUserByID(_ context.Context, id int64) (store.User, error) {
	user, ok := f.users[id]
	if !ok {
		return store.User{}, store.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, _ string) (store.User, error) {
	return store.User{}, store.ErrUserNotFound
}

func (f *fakeUserStore) CreateUser(_ context.Context, _ store.CreateUserParams) (store.User, error) {
	return store.User{}, nil
}

func (f *fakeUserStore) ListUsers(_ context.Context) ([]store.User, error) { return nil, nil }

func TestLoadCurrentUserFromSession_ValidCookieLoadsUser(t *testing.T) {
	sessionStore := &fakeSessionStore{
		sessionByToken: map[string]store.Session{
			"token-1": {Token: "token-1", UserID: 9, ExpiresAt: time.Now().UTC().Add(time.Hour)},
		},
	}
	userStore := &fakeUserStore{
		users: map[int64]store.User{
			9: {ID: 9, Email: "u@example.com"},
		},
	}

	handler := LoadCurrentUserFromSession(sessionStore, userStore, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUserFromContext(r.Context())
		if !ok || user.ID != 9 {
			t.Fatalf("expected authenticated user id=9, got ok=%v user=%+v", ok, user)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token-1"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestLoadCurrentUserFromSession_InvalidCookieLeavesAnonymous(t *testing.T) {
	sessionStore := &fakeSessionStore{sessionByToken: map[string]store.Session{}}
	userStore := &fakeUserStore{users: map[int64]store.User{}}

	handler := LoadCurrentUserFromSession(sessionStore, userStore, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUserFromContext(r.Context()); ok {
			t.Fatal("expected anonymous request context for missing/invalid session")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "missing-token"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestRequireUserSessionAuth_UnauthorizedJSONWhenAnonymous(t *testing.T) {
	handler := RequireUserSessionAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous JSON route, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON error body, got %v", err)
	}
}

func TestRequireUserSessionAuthHTML_RedirectWhenAnonymous(t *testing.T) {
	handler := RequireUserSessionAuthHTML(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ui/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for anonymous HTML route, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/login" {
		t.Fatalf("expected redirect to /login, got %q", got)
	}
}
