package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"plexplore/internal/api"
	"plexplore/internal/store"
)

type authIntegrationEnv struct {
	t      *testing.T
	dbPath string
	rt     *integrationRuntime
	mux    *http.ServeMux
}

type webSession struct {
	sessionToken string
	csrfToken    string
}

func newAuthIntegrationEnv(t *testing.T) *authIntegrationEnv {
	t.Helper()

	baseDir := t.TempDir()
	spoolDir := filepath.Join(baseDir, "spool")
	dbPath := filepath.Join(baseDir, "tracker.db")
	if err := applyTestSchema(dbPath); err != nil {
		t.Fatalf("apply test schema: %v", err)
	}

	rt := openIntegrationRuntime(t, spoolDir, dbPath, 1024*1024, 64, nil)
	env := &authIntegrationEnv{
		t:      t,
		dbPath: dbPath,
		rt:     rt,
	}

	mux := http.NewServeMux()
	api.RegisterRoutesWithDependencies(mux, api.Dependencies{
		DeviceStore:  rt.sqliteStore,
		Spool:        rt.spoolManager,
		Buffer:       rt.bufferManager,
		Flusher:      rt.batchFlusher,
		PointStore:   rt.sqliteStore,
		UserStore:    rt.sqliteStore,
		SessionStore: rt.sqliteStore,
	})
	env.mux = mux

	t.Cleanup(func() {
		if env.rt != nil {
			env.rt.close()
		}
	})
	return env
}

func TestIntegration_MultiUserAuthorizationIsolation(t *testing.T) {
	env := newAuthIntegrationEnv(t)

	adminPassword := "adminpass"
	adminHash, err := api.HashPassword(adminPassword)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	_, err = env.rt.sqliteStore.CreateUser(context.Background(), store.CreateUserParams{
		Name:         "Admin",
		Email:        "admin@example.com",
		PasswordHash: adminHash,
		IsAdmin:      true,
	})
	if err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	adminSession := env.login("admin@example.com", adminPassword)
	user2ID := env.createUserAsAdmin(adminSession, "user2@example.com", "user2pass")
	user3ID := env.createUserAsAdmin(adminSession, "user3@example.com", "user3pass")
	if user2ID == user3ID || user2ID <= 0 || user3ID <= 0 {
		t.Fatalf("unexpected created user ids: user2=%d user3=%d", user2ID, user3ID)
	}

	user2Session := env.login("user2@example.com", "user2pass")
	user3Session := env.login("user3@example.com", "user3pass")

	device2 := env.createDevice(user2Session, "phone-main", "u2-key")
	device3 := env.createDevice(user3Session, "phone-main", "u3-key")

	ingest2 := env.postJSON("/api/v1/owntracks", "u2-key", ownTracksPayload(41.80100, -87.80100, 1713777600), webSession{})
	if ingest2.Code != http.StatusOK {
		t.Fatalf("user2 ingest expected 200, got %d body=%s", ingest2.Code, ingest2.Body.String())
	}
	ingest3 := env.postJSON("/api/v1/owntracks", "u3-key", ownTracksPayload(41.80200, -87.80200, 1713777660), webSession{})
	if ingest3.Code != http.StatusOK {
		t.Fatalf("user3 ingest expected 200, got %d body=%s", ingest3.Code, ingest3.Body.String())
	}
	if err := env.rt.batchFlusher.FlushNow(); err != nil {
		t.Fatalf("FlushNow failed: %v", err)
	}

	rotateDenied := env.postJSON("/api/v1/devices/"+strconv.FormatInt(device3.id, 10)+"/rotate-key", "", `{"api_key":"hijack-key"}`, user2Session)
	if rotateDenied.Code != http.StatusForbidden {
		t.Fatalf("non-owner rotate expected 403, got %d body=%s", rotateDenied.Code, rotateDenied.Body.String())
	}

	devicesU2 := env.getJSON("/api/v1/devices", user2Session)
	if devicesU2.Code != http.StatusOK {
		t.Fatalf("user2 list devices expected 200, got %d body=%s", devicesU2.Code, devicesU2.Body.String())
	}
	var list2 struct {
		Devices []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(devicesU2.Body.Bytes(), &list2); err != nil {
		t.Fatalf("unmarshal user2 devices failed: %v", err)
	}
	if len(list2.Devices) != 1 || list2.Devices[0].ID != device2.id {
		t.Fatalf("expected user2 to see only own device id=%d, got %+v", device2.id, list2.Devices)
	}

	pointsU2 := env.getJSON("/api/v1/points?limit=20", user2Session)
	if pointsU2.Code != http.StatusOK {
		t.Fatalf("user2 points expected 200, got %d body=%s", pointsU2.Code, pointsU2.Body.String())
	}
	var pointsResp struct {
		Points []struct {
			DeviceID string `json:"device_id"`
		} `json:"points"`
	}
	if err := json.Unmarshal(pointsU2.Body.Bytes(), &pointsResp); err != nil {
		t.Fatalf("unmarshal user2 points failed: %v", err)
	}
	if len(pointsResp.Points) != 1 || pointsResp.Points[0].DeviceID != device2.name {
		t.Fatalf("expected user2 to see only own points, got %+v", pointsResp.Points)
	}

	exportU3 := env.getJSON("/api/v1/exports/geojson", user3Session)
	if exportU3.Code != http.StatusOK {
		t.Fatalf("user3 export expected 200, got %d body=%s", exportU3.Code, exportU3.Body.String())
	}
	var geo struct {
		Features []struct {
			Properties map[string]interface{} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(exportU3.Body.Bytes(), &geo); err != nil {
		t.Fatalf("unmarshal user3 geojson failed: %v", err)
	}
	if len(geo.Features) != 1 {
		t.Fatalf("expected one feature in user3 export, got %d", len(geo.Features))
	}
	if got := geo.Features[0].Properties["device_id"]; got != device3.name {
		t.Fatalf("expected user3 export device_id=%q, got %+v", device3.name, got)
	}

	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points WHERE device_id = ?;`, device2.id); got != 1 {
		t.Fatalf("expected one raw point for user2 device, got %d", got)
	}
	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points WHERE device_id = ?;`, device3.id); got != 1 {
		t.Fatalf("expected one raw point for user3 device, got %d", got)
	}
	if got := queryInt(t, env.dbPath, `SELECT COUNT(*) FROM raw_points rp JOIN devices d ON rp.device_id = d.id WHERE rp.user_id != d.user_id;`); got != 0 {
		t.Fatalf("expected no raw_points/device ownership mismatches, got %d", got)
	}
}

func TestIntegration_DeviceAPIKeyStoredHashedAtRest(t *testing.T) {
	env := newAuthIntegrationEnv(t)

	adminHash, err := api.HashPassword("adminpass")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if _, err := env.rt.sqliteStore.CreateUser(context.Background(), store.CreateUserParams{
		Name:         "Admin",
		Email:        "admin@example.com",
		PasswordHash: adminHash,
		IsAdmin:      true,
	}); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	adminSession := env.login("admin@example.com", "adminpass")
	device := env.createDevice(adminSession, "phone-main", "plain-created-key")

	ingest := env.postJSON("/api/v1/owntracks", "plain-created-key", ownTracksPayload(41.80100, -87.80100, 1713777600), webSession{})
	if ingest.Code != http.StatusOK {
		t.Fatalf("ingest with created key expected 200, got %d body=%s", ingest.Code, ingest.Body.String())
	}

	storedRaw := queryString(t, env.dbPath, fmt.Sprintf(`SELECT COALESCE(api_key, '') FROM devices WHERE id = %d;`, device.id))
	storedHash := queryString(t, env.dbPath, fmt.Sprintf(`SELECT COALESCE(api_key_hash, '') FROM devices WHERE id = %d;`, device.id))
	storedPreview := queryString(t, env.dbPath, fmt.Sprintf(`SELECT COALESCE(api_key_preview, '') FROM devices WHERE id = %d;`, device.id))
	if strings.TrimSpace(storedHash) == "" {
		t.Fatalf("expected non-empty api_key_hash")
	}
	if strings.TrimSpace(storedPreview) == "" {
		t.Fatalf("expected non-empty api_key_preview")
	}
	if strings.TrimSpace(storedRaw) == "plain-created-key" {
		t.Fatalf("expected plaintext api key not stored at rest")
	}
}

type deviceCreationResult struct {
	id   int64
	name string
}

func (e *authIntegrationEnv) login(email, password string) webSession {
	e.t.Helper()

	csrfReq := httptest.NewRequest(http.MethodGet, "/login", nil)
	csrfRec := httptest.NewRecorder()
	e.mux.ServeHTTP(csrfRec, csrfReq)
	if csrfRec.Code != http.StatusOK {
		e.t.Fatalf("login page for %s expected 200, got %d", email, csrfRec.Code)
	}
	var csrfToken string
	for _, c := range csrfRec.Result().Cookies() {
		if c.Name == "plexplore_csrf" {
			csrfToken = c.Value
			break
		}
	}
	if csrfToken == "" {
		e.t.Fatalf("login page for %s did not set csrf cookie", email)
	}

	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)
	form.Set("csrf_token", csrfToken)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "plexplore_csrf", Value: csrfToken})
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		e.t.Fatalf("login %s expected 303, got %d body=%s", email, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "plexplore_session" && c.Value != "" {
			return webSession{
				sessionToken: c.Value,
				csrfToken:    csrfToken,
			}
		}
	}
	e.t.Fatalf("login %s did not set plexplore_session cookie", email)
	return webSession{}
}

func (e *authIntegrationEnv) createUserAsAdmin(adminSession webSession, email, password string) int64 {
	e.t.Helper()

	body := `{"email":"` + email + `","password":"` + password + `","is_admin":false}`
	rec := e.postJSONWithCSRF("/api/v1/users", "", body, adminSession)
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("admin create user %s expected 201, got %d body=%s", email, rec.Code, rec.Body.String())
	}
	var out struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		e.t.Fatalf("unmarshal created user failed: %v", err)
	}
	return out.ID
}

func (e *authIntegrationEnv) createDevice(session webSession, name, apiKey string) deviceCreationResult {
	e.t.Helper()

	body := `{"name":"` + name + `","source_type":"owntracks","api_key":"` + apiKey + `"}`
	rec := e.postJSON("/api/v1/devices", "", body, session)
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("create device expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		e.t.Fatalf("unmarshal created device failed: %v", err)
	}
	return deviceCreationResult{
		id:   out.ID,
		name: out.Name,
	}
}

func (e *authIntegrationEnv) postJSON(path, apiKey, body string, session webSession) *httptest.ResponseRecorder {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	if session.sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: "plexplore_session", Value: session.sessionToken})
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func (e *authIntegrationEnv) postJSONWithCSRF(path, apiKey, body string, session webSession) *httptest.ResponseRecorder {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", session.csrfToken)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	if session.sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: "plexplore_session", Value: session.sessionToken})
	}
	if session.csrfToken != "" {
		req.AddCookie(&http.Cookie{Name: "plexplore_csrf", Value: session.csrfToken})
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func (e *authIntegrationEnv) getJSON(path string, session webSession) *httptest.ResponseRecorder {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if session.sessionToken != "" {
		req.AddCookie(&http.Cookie{Name: "plexplore_session", Value: session.sessionToken})
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}
