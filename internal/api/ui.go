package api

import (
	"html"
	"io"
	"net/http"
	"strings"
)

const statusPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Status</title>
  <meta name="csrf-token" content="__CSRF_TOKEN__">
  <link rel="stylesheet" href="/ui/assets/app/app.css">
  <link rel="stylesheet" href="/ui/assets/app/status.css">
  <script defer src="/ui/assets/app/common.js"></script>
  <script defer src="/ui/assets/app/status.js"></script>
</head>
<body>
  <div class="wrap status-wrap">
    <div class="topbar">
      <h1>Plexplore Status</h1>
      <div class="top-actions">
        <span id="session_user" class="session-user">Signed in: __USER_EMAIL__</span>
        <a id="status_to_map_link" class="nav-link" href="/ui/map">Map</a>
        __ADMIN_DEVICE_LINK__
        __ADMIN_USERS_LINK__
        <form method="post" action="/logout" class="inline-form">
          <input type="hidden" name="csrf_token" value="__CSRF_TOKEN__">
          <button class="logout-btn" type="submit">Logout</button>
        </form>
        <button id="theme_toggle" class="theme-toggle" type="button" aria-label="Toggle dark mode" aria-pressed="false">🌙</button>
      </div>
    </div>
    <div id="updated" class="tiny">Loading...</div>

    <div class="grid">
      <div class="card">
        <div class="label">Service Health</div>
        <div id="health" class="value">-</div>
      </div>
      <div class="card">
        <div class="label">Buffered Points</div>
        <div id="buffer_points" class="value">-</div>
      </div>
      <div class="card">
        <div class="label">Buffered Bytes</div>
        <div id="buffer_bytes" class="value">-</div>
      </div>
      <div class="card">
        <div class="label">Oldest Buffered Age</div>
        <div id="buffer_age" class="value">-</div>
      </div>
      <div class="card">
        <div class="label">Spool Segments</div>
        <div id="spool_segments" class="value">-</div>
      </div>
      <div class="card">
        <div class="label">Checkpoint Seq</div>
        <div id="checkpoint_seq" class="value">-</div>
      </div>
      <div class="card">
        <div class="label">Last Flush</div>
        <div id="flush_status" class="value">-</div>
        <div id="flush_meta" class="tiny"></div>
      </div>
      <div class="card">
        <div class="label">Visit Scheduler</div>
        <div id="scheduler_state" class="value">-</div>
        <div id="scheduler_meta" class="tiny"></div>
      </div>
    </div>

    <div class="card mt-12">
      <div class="label">Devices</div>
      <table>
        <thead>
          <tr><th>ID</th><th>Name</th><th>Source</th><th>User</th></tr>
        </thead>
        <tbody id="devices_body">
          <tr><td colspan="4" class="muted">Loading...</td></tr>
        </tbody>
      </table>
    </div>

    <div class="card mt-12">
      <div class="label">Recent Points</div>
      <table>
        <thead>
          <tr><th>Seq</th><th>Device</th><th>Time (UTC)</th><th>Lat</th><th>Lon</th></tr>
        </thead>
        <tbody id="points_body">
          <tr><td colspan="5" class="muted">Loading...</td></tr>
        </tbody>
      </table>
    </div>
  </div>

</body>
</html>
`

const mapPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Map</title>
  <meta name="csrf-token" content="__CSRF_TOKEN__">
  <link rel="stylesheet" href="/ui/assets/leaflet/leaflet.css" />
  <link rel="stylesheet" href="/ui/assets/app/app.css">
  <link rel="stylesheet" href="/ui/assets/app/map.css">
  <script defer src="/ui/assets/leaflet/leaflet.js"></script>
  <script defer src="/ui/assets/app/common.js"></script>
  <script defer src="/ui/assets/app/map.js"></script>
</head>
<body>
  <div class="wrap map-wrap">
    <div class="topbar">
      <h1>Plexplore Map</h1>
      <div class="top-actions">
        <span id="session_user" class="session-user">Signed in: __USER_EMAIL__</span>
        <a id="map_to_status_link" class="nav-link" href="/ui/status">Status</a>
        __ADMIN_DEVICE_LINK__
        __ADMIN_USERS_LINK__
        <form method="post" action="/logout" class="inline-form">
          <input type="hidden" name="csrf_token" value="__CSRF_TOKEN__">
          <button class="logout-btn" type="submit">Logout</button>
        </form>
        <button id="theme_toggle" class="theme-toggle" type="button" aria-label="Toggle dark mode" aria-pressed="false">🌙</button>
      </div>
    </div>
    <div class="card">
      <div class="row">
        <label>Device:
          <select id="device_select">
            <option value="">All devices</option>
          </select>
        </label>
        <label>From date:
          <input id="date_from" type="date">
        </label>
        <label>To date:
          <input id="date_to" type="date">
        </label>
        <label>Limit:
          <input id="limit" value="1500" class="limit-input">
        </label>
        <button id="load_btn" type="button">Refresh</button>
      </div>
      <div id="meta" class="muted mt-8">Loading...</div>
      <div id="sampling_note" class="tiny mt-8"></div>
    </div>
    <div class="card">
      <div
        id="map"
        data-tile-mode="__MAP_TILE_MODE__"
        data-tile-url-template="__MAP_TILE_URL_TEMPLATE__"
        data-tile-attribution="__MAP_TILE_ATTRIBUTION__"></div>
    </div>
    <div class="card">
      <div class="row row-space-between">
        <strong>Visits Summary</strong>
        <span id="visits_summary_meta" class="muted">Loading...</span>
      </div>
      <table class="visits-table">
        <thead>
          <tr>
            <th>Start (UTC)</th>
            <th>End (UTC)</th>
            <th>Duration</th>
            <th>Device</th>
          </tr>
        </thead>
        <tbody id="visits_body">
          <tr><td colspan="4" class="muted">Loading...</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</body>
</html>
`

const adminUsersPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Users</title>
  <meta name="csrf-token" content="__CSRF_TOKEN__">
  <link rel="stylesheet" href="/ui/assets/app/app.css">
  <link rel="stylesheet" href="/ui/assets/app/users.css">
  <script defer src="/ui/assets/app/common.js"></script>
  <script defer src="/ui/assets/app/users.js"></script>
</head>
<body>
  <div class="wrap users-wrap">
    <div class="topbar">
      <h1>Users</h1>
      <div class="top-actions">
        <span id="session_user" class="session-user">Signed in: __USER_EMAIL__</span>
        <a class="nav-link" href="/ui/status">Status</a>
        <a class="nav-link" href="/ui/map">Map</a>
        <a id="admin_devices_link" class="nav-link" href="/ui/admin/devices">Devices</a>
        <form method="post" action="/logout" class="inline-form">
          <input type="hidden" name="csrf_token" value="__CSRF_TOKEN__">
          <button class="logout-btn" type="submit">Logout</button>
        </form>
        <button id="theme_toggle" class="theme-toggle" type="button" aria-label="Toggle dark mode" aria-pressed="false">🌙</button>
      </div>
    </div>

    <div class="card">
      <div class="row">
        <input id="email" type="email" class="wide-input" placeholder="user@example.com">
        <input id="password" type="password" class="mid-input" placeholder="password">
        <label><input id="is_admin" type="checkbox"> admin</label>
        <button id="create_btn" type="button">Create User</button>
      </div>
      <div id="create_status" class="muted mt-8"></div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr><th>ID</th><th>Email</th><th>Admin</th><th>Created</th></tr>
        </thead>
        <tbody id="users_body">
          <tr><td colspan="4" class="muted">Loading...</td></tr>
        </tbody>
      </table>
    </div>
  </div>

</body>
</html>
`

const adminDevicesPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Devices</title>
  <meta name="csrf-token" content="__CSRF_TOKEN__">
  <link rel="stylesheet" href="/ui/assets/app/app.css">
  <link rel="stylesheet" href="/ui/assets/app/devices.css">
  <script defer src="/ui/assets/app/common.js"></script>
  <script defer src="/ui/assets/app/devices.js"></script>
</head>
<body>
  <div class="wrap devices-wrap">
    <div class="topbar">
      <h1>Devices</h1>
      <div class="top-actions">
        <span id="session_user" class="session-user">Signed in: __USER_EMAIL__</span>
        <a class="nav-link" href="/ui/status">Status</a>
        <a class="nav-link" href="/ui/map">Map</a>
        <a id="admin_users_link" class="nav-link" href="/ui/admin/users">Users</a>
        <form method="post" action="/logout" class="inline-form">
          <input type="hidden" name="csrf_token" value="__CSRF_TOKEN__">
          <button class="logout-btn" type="submit">Logout</button>
        </form>
        <button id="theme_toggle" class="theme-toggle" type="button" aria-label="Toggle dark mode" aria-pressed="false">🌙</button>
      </div>
    </div>

    <div class="card">
      <div class="section-title">Create Device</div>
      <div class="row">
        <select id="device_owner_select" class="mid-input" aria-label="Device owner"></select>
        <input id="device_name" type="text" class="wide-input" placeholder="device name">
        <select id="device_source_type" class="mid-input" aria-label="Source type">
          <option value="owntracks">owntracks</option>
          <option value="overland">overland</option>
        </select>
        <button id="create_device_btn" type="button">Create Device</button>
      </div>
      <div id="create_device_status" class="muted mt-8"></div>
    </div>

    <div class="card mt-12">
      <div class="section-title">Rotate API Key</div>
      <div id="rotate_status" class="muted">Use the Rotate button from a device row.</div>
      <div id="rotate_result" class="rotate-result hidden">
        <div class="status-warning">Save this API key now. It is shown once and cannot be retrieved later.</div>
        <div class="row mt-8">
          <input id="rotated_api_key" class="wide-input key-output" type="text" readonly>
          <button id="copy_rotated_key_btn" type="button">Copy Key</button>
        </div>
      </div>
    </div>

    <div class="card mt-12">
      <div class="section-title">Generate Visits</div>
      <div class="row">
        <select id="visit_device_select" class="mid-input" aria-label="Visit device">
          <option value="">All devices</option>
        </select>
        <label>From:
          <input id="visit_from_date" type="date">
        </label>
        <label>To:
          <input id="visit_to_date" type="date">
        </label>
        <button id="generate_visits_btn" type="button">Generate Visits</button>
      </div>
      <div id="generate_visits_status" class="muted mt-8"></div>
    </div>

    <div class="card mt-12">
      <div class="section-title">Devices</div>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
            <th>Owner</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Key Preview</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody id="devices_admin_body">
          <tr><td colspan="7" class="muted">Loading...</td></tr>
        </tbody>
      </table>
      <div class="tiny mt-8">Delete/disable actions are not currently supported by backend APIs.</div>
    </div>
  </div>
</body>
</html>
`

func registerUIRoutes(mux *http.ServeMux, deps Dependencies) {
	if deps.UserStore == nil || deps.SessionStore == nil {
		panic("registerUIRoutes requires non-nil userStore and sessionStore")
	}

	protectedHTML := func(handler http.HandlerFunc) http.Handler {
		return LoadCurrentUserFromSession(
			deps.SessionStore,
			deps.UserStore,
			RequireUserSessionAuthHTML(http.HandlerFunc(handler)),
		)
	}
	mux.Handle("GET /{$}", protectedHTML(statusPageHandler(deps.CookieSecurity)))
	mux.Handle("GET /ui/status", protectedHTML(statusPageHandler(deps.CookieSecurity)))
	mux.Handle("GET /ui/map", protectedHTML(mapPageHandler(deps.CookieSecurity, deps.MapTiles)))
	mux.Handle("GET /ui/admin/users", protectedHTML(requireAdminHTML(adminUsersPageHandler(deps.CookieSecurity))))
	mux.Handle("GET /ui/admin/devices", protectedHTML(requireAdminHTML(adminDevicesPageHandler(deps.CookieSecurity))))
}

func statusPageHandler(cookiePolicy CookieSecurityPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrfToken := ensureCSRFCookie(w, r, cookiePolicy)
		setHTMLSecurityHeaders(w, MapTileConfig{Mode: "none"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, renderUIPage(statusPageHTML, r, csrfToken))
	}
}

func mapPageHandler(cookiePolicy CookieSecurityPolicy, mapTiles MapTileConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrfToken := ensureCSRFCookie(w, r, cookiePolicy)
		setHTMLSecurityHeaders(w, mapTiles)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, renderMapPage(mapPageHTML, r, csrfToken, mapTiles))
	}
}

func adminUsersPageHandler(cookiePolicy CookieSecurityPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrfToken := ensureCSRFCookie(w, r, cookiePolicy)
		setHTMLSecurityHeaders(w, MapTileConfig{Mode: "none"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, renderUIPage(adminUsersPageHTML, r, csrfToken))
	}
}

func adminDevicesPageHandler(cookiePolicy CookieSecurityPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csrfToken := ensureCSRFCookie(w, r, cookiePolicy)
		setHTMLSecurityHeaders(w, MapTileConfig{Mode: "none"})
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, renderUIPage(adminDevicesPageHTML, r, csrfToken))
	}
}

func renderUIPage(page string, r *http.Request, csrfToken string) string {
	userEmail := "local"
	adminUsersLink := ""
	adminDevicesLink := ""
	if currentUser, ok := CurrentUserFromContext(r.Context()); ok {
		if trimmed := strings.TrimSpace(currentUser.Email); trimmed != "" {
			userEmail = trimmed
		}
		if currentUser.IsAdmin {
			adminDevicesLink = `<a id="admin_devices_link" class="nav-link" href="/ui/admin/devices">Devices</a>`
			adminUsersLink = `<a id="admin_users_link" class="nav-link" href="/ui/admin/users">Users</a>`
		}
	}
	rendered := strings.ReplaceAll(page, "__USER_EMAIL__", html.EscapeString(userEmail))
	rendered = strings.ReplaceAll(rendered, "__ADMIN_DEVICE_LINK__", adminDevicesLink)
	rendered = strings.ReplaceAll(rendered, "__ADMIN_USERS_LINK__", adminUsersLink)
	return strings.ReplaceAll(rendered, "__CSRF_TOKEN__", html.EscapeString(csrfToken))
}

func renderMapPage(page string, r *http.Request, csrfToken string, mapTiles MapTileConfig) string {
	mode := strings.ToLower(strings.TrimSpace(mapTiles.Mode))
	if mode == "" {
		mode = "none"
	}
	urlTemplate := strings.TrimSpace(mapTiles.URLTemplate)
	attribution := strings.TrimSpace(mapTiles.Attribution)
	if mode == "osm" && urlTemplate == "" {
		urlTemplate = "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
	}
	if mode == "osm" && attribution == "" {
		attribution = "&copy; OpenStreetMap contributors"
	}

	rendered := renderUIPage(page, r, csrfToken)
	rendered = strings.ReplaceAll(rendered, "__MAP_TILE_MODE__", html.EscapeString(mode))
	rendered = strings.ReplaceAll(rendered, "__MAP_TILE_URL_TEMPLATE__", html.EscapeString(urlTemplate))
	return strings.ReplaceAll(rendered, "__MAP_TILE_ATTRIBUTION__", html.EscapeString(attribution))
}

func requireAdminHTML(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, ok := CurrentUserFromContext(r.Context())
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !currentUser.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
