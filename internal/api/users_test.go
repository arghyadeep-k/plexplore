package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plexplore/internal/store"
)

type fakeAdminUserStore struct {
	users  []store.User
	nextID int64
}

func (f *fakeAdminUserStore) CreateUser(_ context.Context, params store.CreateUserParams) (store.User, error) {
	if f.nextID == 0 {
		f.nextID = 1
	}
	user := store.User{
		ID:           f.nextID,
		Name:         params.Name,
		Email:        strings.ToLower(strings.TrimSpace(params.Email)),
		PasswordHash: params.PasswordHash,
		IsAdmin:      params.IsAdmin,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	f.nextID++
	f.users = append(f.users, user)
	return user, nil
}

func (f *fakeAdminUserStore) GetUserByEmail(_ context.Context, email string) (store.User, error) {
	target := strings.ToLower(strings.TrimSpace(email))
	for _, user := range f.users {
		if strings.ToLower(strings.TrimSpace(user.Email)) == target {
			return user, nil
		}
	}
	return store.User{}, store.ErrUserNotFound
}

func (f *fakeAdminUserStore) GetUserByID(_ context.Context, id int64) (store.User, error) {
	for _, user := range f.users {
		if user.ID == id {
			return user, nil
		}
	}
	return store.User{}, store.ErrUserNotFound
}

func (f *fakeAdminUserStore) ListUsers(_ context.Context) ([]store.User, error) {
	out := make([]store.User, len(f.users))
	copy(out, f.users)
	return out, nil
}

type fakeAuthzSessionStore struct {
	sessions map[string]store.Session
}

func (f *fakeAuthzSessionStore) CreateSession(_ context.Context, userID int64) (store.Session, error) {
	session := store.Session{
		Token:     "token-created",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	return session, nil
}

func (f *fakeAuthzSessionStore) GetSession(_ context.Context, token string) (store.Session, error) {
	session, ok := f.sessions[token]
	if !ok {
		return store.Session{}, store.ErrSessionNotFound
	}
	return session, nil
}

func (f *fakeAuthzSessionStore) DeleteSession(_ context.Context, _ string) error { return nil }

func TestUsers_AdminCanCreateUser(t *testing.T) {
	userStore := &fakeAdminUserStore{
		nextID: 2,
		users: []store.User{
			{ID: 1, Email: "admin@example.com", IsAdmin: true, PasswordHash: "hash", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	sessionStore := &fakeAuthzSessionStore{
		sessions: map[string]store.Session{
			"admin-token": {Token: "admin-token", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	body := bytes.NewBufferString(`{"email":"new@example.com","password":"testpass","is_admin":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf-token-1")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if _, ok := payload["password_hash"]; ok {
		t.Fatalf("response leaked password_hash: %v", payload)
	}
}

func TestUsers_NonAdminCannotCreateUser(t *testing.T) {
	userStore := &fakeAdminUserStore{
		users: []store.User{
			{ID: 1, Email: "user@example.com", IsAdmin: false, PasswordHash: "hash", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	sessionStore := &fakeAuthzSessionStore{
		sessions: map[string]store.Session{
			"user-token": {Token: "user-token", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(`{"email":"new@example.com","password":"testpass"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "user-token"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUsers_AdminCreateRequiresCSRF(t *testing.T) {
	userStore := &fakeAdminUserStore{
		nextID: 2,
		users: []store.User{
			{ID: 1, Email: "admin@example.com", IsAdmin: true, PasswordHash: "hash", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	sessionStore := &fakeAuthzSessionStore{
		sessions: map[string]store.Session{
			"admin-token": {Token: "admin-token", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewBufferString(`{"email":"new@example.com","password":"testpass"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf token, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUsers_UnauthenticatedDenied(t *testing.T) {
	userStore := &fakeAdminUserStore{}
	sessionStore := &fakeAuthzSessionStore{sessions: map[string]store.Session{}}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUsers_ListDoesNotExposePasswordHash(t *testing.T) {
	userStore := &fakeAdminUserStore{
		users: []store.User{
			{ID: 1, Email: "admin@example.com", IsAdmin: true, PasswordHash: "hash-admin", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			{ID: 2, Email: "user@example.com", IsAdmin: false, PasswordHash: "hash-user", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	sessionStore := &fakeAuthzSessionStore{
		sessions: map[string]store.Session{
			"admin-token": {Token: "admin-token", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password_hash") {
		t.Fatalf("list users response leaked password_hash: %s", rec.Body.String())
	}
}

func TestUsers_AdminRoutesRateLimited(t *testing.T) {
	userStore := &fakeAdminUserStore{
		users: []store.User{
			{ID: 1, Email: "admin@example.com", IsAdmin: true, PasswordHash: "hash-admin", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	sessionStore := &fakeAuthzSessionStore{
		sessions: map[string]store.Session{
			"admin-token": {Token: "admin-token", UserID: 1, ExpiresAt: time.Now().UTC().Add(time.Hour)},
		},
	}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
		RateLimiters: RateLimiters{
			AdminSensitive: NewFixedWindowLimiter(1, time.Minute),
		},
	})

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req1.RemoteAddr = "198.51.100.22:1000"
	req1.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first admin users request=200, got %d body=%s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req2.RemoteAddr = "198.51.100.22:1000"
	req2.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second admin users request=429, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}
