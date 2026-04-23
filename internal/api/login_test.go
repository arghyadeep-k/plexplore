package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"plexplore/internal/store"
)

type fakeAuthUserStore struct {
	byEmail map[string]store.User
}

func (f *fakeAuthUserStore) GetUserByID(_ context.Context, id int64) (store.User, error) {
	for _, user := range f.byEmail {
		if user.ID == id {
			return user, nil
		}
	}
	return store.User{}, store.ErrUserNotFound
}

func (f *fakeAuthUserStore) GetUserByEmail(_ context.Context, email string) (store.User, error) {
	user, ok := f.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return store.User{}, store.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeAuthUserStore) CreateUser(_ context.Context, _ store.CreateUserParams) (store.User, error) {
	return store.User{}, nil
}

func (f *fakeAuthUserStore) ListUsers(_ context.Context) ([]store.User, error) { return nil, nil }

type fakeAuthSessionStore struct {
	created []store.Session
	deleted []string
}

func (f *fakeAuthSessionStore) CreateSession(_ context.Context, userID int64) (store.Session, error) {
	session := store.Session{
		Token:     "sess-token-1",
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	f.created = append(f.created, session)
	return session, nil
}

func (f *fakeAuthSessionStore) GetSession(_ context.Context, _ string) (store.Session, error) {
	return store.Session{}, store.ErrSessionNotFound
}

func (f *fakeAuthSessionStore) DeleteSession(_ context.Context, token string) error {
	f.deleted = append(f.deleted, token)
	return nil
}

func TestLoginPageServed(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    &fakeAuthUserStore{},
		SessionStore: &fakeAuthSessionStore{},
	})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Sign In") {
		t.Fatalf("expected login page body, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `name="csrf_token"`) {
		t.Fatalf("expected csrf token field in login page, got %q", rec.Body.String())
	}
}

func TestLoginSuccessSetsSessionCookie(t *testing.T) {
	hash, err := HashPassword("test-pass")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	userStore := &fakeAuthUserStore{
		byEmail: map[string]store.User{
			"admin@example.com": {
				ID:           7,
				Email:        "admin@example.com",
				PasswordHash: hash,
			},
		},
	}
	sessionStore := &fakeAuthSessionStore{}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	csrfToken, csrfCookie := fetchLoginCSRF(t, mux)

	form := url.Values{}
	form.Set("email", "admin@example.com")
	form.Set("password", "test-pass")
	form.Set("csrf_token", csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(sessionStore.created) != 1 {
		t.Fatalf("expected one created session, got %d", len(sessionStore.created))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName || cookies[0].Value == "" {
		t.Fatalf("expected session cookie, got %+v", cookies)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	hash, err := HashPassword("correct-pass")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	userStore := &fakeAuthUserStore{
		byEmail: map[string]store.User{
			"user@example.com": {
				ID:           8,
				Email:        "user@example.com",
				PasswordHash: hash,
			},
		},
	}
	sessionStore := &fakeAuthSessionStore{}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	csrfToken, csrfCookie := fetchLoginCSRF(t, mux)

	form := url.Values{}
	form.Set("email", "user@example.com")
	form.Set("password", "wrong-pass")
	form.Set("csrf_token", csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Sign In") {
		t.Fatalf("expected login page to be rendered, got %q", body)
	}
	if !strings.Contains(body, "Invalid email or password") {
		t.Fatalf("expected inline invalid credentials message, got %q", body)
	}
	if !strings.Contains(body, `class="error"`) || !strings.Contains(body, `role="alert"`) {
		t.Fatalf("expected visible/accessible error styling state, got %q", body)
	}
	if !strings.Contains(body, `name="email" value="user@example.com"`) {
		t.Fatalf("expected email field to retain entered value, got %q", body)
	}
	if strings.Contains(body, "wrong-pass") {
		t.Fatalf("did not expect password value to be preserved, got %q", body)
	}
}

func TestLoginInvalidCredentials_JSONStillReturnsJSON(t *testing.T) {
	hash, err := HashPassword("correct-pass")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	userStore := &fakeAuthUserStore{
		byEmail: map[string]store.User{
			"user@example.com": {
				ID:           8,
				Email:        "user@example.com",
				PasswordHash: hash,
			},
		},
	}
	sessionStore := &fakeAuthSessionStore{}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	csrfToken, csrfCookie := fetchLoginCSRF(t, mux)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"user@example.com","password":"wrong-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(csrfCookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected JSON content type, got %q", rec.Header().Get("Content-Type"))
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON error response, got %v", err)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	sessionStore := &fakeAuthSessionStore{}
	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    &fakeAuthUserStore{},
		SessionStore: sessionStore,
	})

	csrfToken, csrfCookie := fetchLoginCSRF(t, mux)

	form := url.Values{}
	form.Set("csrf_token", csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookie)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "sess-token-1"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if len(sessionStore.deleted) != 1 || sessionStore.deleted[0] != "sess-token-1" {
		t.Fatalf("expected session deletion call, got %+v", sessionStore.deleted)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != sessionCookieName {
		t.Fatalf("expected cleared session cookie, got %+v", cookies)
	}
}

func TestLoginRejectsMissingCSRFToken(t *testing.T) {
	hash, err := HashPassword("test-pass")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	userStore := &fakeAuthUserStore{
		byEmail: map[string]store.User{
			"admin@example.com": {
				ID:           7,
				Email:        "admin@example.com",
				PasswordHash: hash,
			},
		},
	}
	sessionStore := &fakeAuthSessionStore{}

	mux := http.NewServeMux()
	RegisterRoutesWithDependencies(mux, Dependencies{
		UserStore:    userStore,
		SessionStore: sessionStore,
	})

	form := url.Values{}
	form.Set("email", "admin@example.com")
	form.Set("password", "test-pass")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func fetchLoginCSRF(t *testing.T, mux *http.ServeMux) (string, *http.Cookie) {
	t.Helper()

	getReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /login expected 200, got %d", getRec.Code)
	}

	var csrfCookie *http.Cookie
	for _, c := range getRec.Result().Cookies() {
		if c.Name == csrfCookieName {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil || strings.TrimSpace(csrfCookie.Value) == "" {
		t.Fatal("expected csrf cookie from GET /login")
	}

	re := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	matches := re.FindStringSubmatch(getRec.Body.String())
	if len(matches) != 2 || strings.TrimSpace(matches[1]) == "" {
		t.Fatalf("expected csrf token field in login page body, got %q", getRec.Body.String())
	}
	return matches[1], csrfCookie
}
