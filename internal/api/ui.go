package api

import (
	"io"
	"net/http"
)

const statusPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Status</title>
  <style>
    :root {
      --bg: #f4f6f8;
      --card: #ffffff;
      --text: #1b1f24;
      --muted: #5a6573;
      --accent: #0b6bcb;
      --ok: #188038;
      --warn: #b06000;
      --bad: #b42318;
      --border: #d7dde5;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font: 15px/1.45 "Segoe UI", Tahoma, sans-serif;
      color: var(--text);
      background: linear-gradient(180deg, #eef3f8 0%, var(--bg) 35%, var(--bg) 100%);
    }
    .wrap {
      max-width: 860px;
      margin: 20px auto;
      padding: 0 12px 18px;
    }
    h1 {
      margin: 0 0 10px;
      font-size: 24px;
    }
    .muted { color: var(--muted); }
    .grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
      gap: 12px;
      margin-top: 10px;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 12px;
      box-shadow: 0 1px 2px rgba(9, 30, 66, 0.06);
    }
    .label {
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      color: var(--muted);
      margin-bottom: 4px;
    }
    .value {
      font-size: 22px;
      font-weight: 600;
    }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }
    .bad { color: var(--bad); }
    table {
      width: 100%;
      border-collapse: collapse;
      margin-top: 8px;
    }
    th, td {
      text-align: left;
      padding: 7px 6px;
      border-bottom: 1px solid var(--border);
      font-size: 14px;
    }
    th { color: var(--muted); font-weight: 600; }
    .tiny { font-size: 12px; color: var(--muted); }
    @media (max-width: 560px) {
      .value { font-size: 19px; }
      th, td { font-size: 13px; }
    }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Plexplore Status</h1>
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
    </div>

    <div class="card" style="margin-top:12px;">
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

    <div class="card" style="margin-top:12px;">
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

  <script>
    async function getJSON(path) {
      const res = await fetch(path, { cache: "no-store" });
      if (!res.ok) throw new Error(path + " status " + res.status);
      return res.json();
    }

    function setText(id, value, cls) {
      const el = document.getElementById(id);
      if (!el) return;
      el.textContent = value;
      el.classList.remove("ok", "warn", "bad");
      if (cls) el.classList.add(cls);
    }

    function renderDevices(devices) {
      const body = document.getElementById("devices_body");
      if (!body) return;
      if (!devices || devices.length === 0) {
        body.innerHTML = "<tr><td colspan='4' class='muted'>No devices</td></tr>";
        return;
      }
      body.innerHTML = devices.map((d) =>
        "<tr><td>" + d.id + "</td><td>" + escapeHTML(d.name) + "</td><td>" +
        escapeHTML(d.source_type) + "</td><td>" + d.user_id + "</td></tr>"
      ).join("");
    }

    function renderDevicesUnavailable(message) {
      const body = document.getElementById("devices_body");
      if (!body) return;
      body.innerHTML = "<tr><td colspan='4' class='muted'>Unavailable: " + escapeHTML(message) + "</td></tr>";
    }

    function renderPoints(points) {
      const body = document.getElementById("points_body");
      if (!body) return;
      if (!points || points.length === 0) {
        body.innerHTML = "<tr><td colspan='5' class='muted'>No points</td></tr>";
        return;
      }
      body.innerHTML = points.map((p) =>
        "<tr><td>" + p.seq + "</td><td>" + escapeHTML(p.device_id || "") + "</td><td>" +
        escapeHTML(p.timestamp_utc || "") + "</td><td>" + formatCoord(p.lat) + "</td><td>" +
        formatCoord(p.lon) + "</td></tr>"
      ).join("");
    }

    function renderPointsUnavailable(message) {
      const body = document.getElementById("points_body");
      if (!body) return;
      body.innerHTML = "<tr><td colspan='5' class='muted'>Unavailable: " + escapeHTML(message) + "</td></tr>";
    }

    function formatCoord(value) {
      if (typeof value !== "number" || !isFinite(value)) return "";
      return value.toFixed(5);
    }

    function escapeHTML(value) {
      return String(value).replace(/[&<>'"]/g, (ch) => ({
        "&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"
      }[ch]));
    }

    async function refresh() {
      const updated = document.getElementById("updated");
      try {
        const [health, status] = await Promise.all([
          getJSON("/health"),
          getJSON("/api/v1/status"),
        ]);

        setText("health", health.status === "ok" ? "OK" : "DEGRADED", health.status === "ok" ? "ok" : "warn");
        setText("buffer_points", String(status.buffer_points || 0));
        setText("buffer_bytes", String(status.buffer_bytes || 0));
        setText("buffer_age", String(status.oldest_buffered_age_seconds || 0) + "s");
        setText("spool_segments", String(status.spool_segment_count || 0));
        setText("checkpoint_seq", String(status.checkpoint_seq || 0));

        if (status.last_flush) {
          setText("flush_status", status.last_flush.success ? "Success" : "Failed", status.last_flush.success ? "ok" : "bad");
          const meta = [];
          if (status.last_flush.at_utc) meta.push(status.last_flush.at_utc);
          if (status.last_flush.error) meta.push(status.last_flush.error);
          setText("flush_meta", meta.join(" | "));
        } else {
          setText("flush_status", "Not yet", "warn");
          setText("flush_meta", "");
        }

        let deviceWarning = "";
        try {
          const devicesResp = await getJSON("/api/v1/devices");
          renderDevices((devicesResp && devicesResp.devices) || []);
        } catch (devicesErr) {
          deviceWarning = " | devices: " + devicesErr.message;
          renderDevicesUnavailable(devicesErr.message);
        }

        let pointsWarning = "";
        try {
          const pointsResp = await getJSON("/api/v1/points/recent?limit=10");
          renderPoints((pointsResp && pointsResp.points) || []);
        } catch (pointsErr) {
          pointsWarning = " | points: " + pointsErr.message;
          renderPointsUnavailable(pointsErr.message);
        }

        updated.textContent = "Updated: " + new Date().toLocaleString() + deviceWarning + pointsWarning;
      } catch (err) {
        setText("health", "ERROR", "bad");
        if (updated) updated.textContent = "Update failed: " + err.message;
      }
    }

    refresh();
    setInterval(refresh, 5000);
  </script>
</body>
</html>
`

const mapPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Plexplore Map</title>
  <link
    rel="stylesheet"
    href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"
    integrity="sha256-p4NxAoJBhIIN+hmNHrzRCf9tD/miZyoHS5obTRR9BMY="
    crossorigin=""
  />
  <style>
    :root {
      --bg: #f4f6f8;
      --text: #1b1f24;
      --muted: #5a6573;
      --card: #ffffff;
      --border: #d7dde5;
      --accent: #0b6bcb;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font: 15px/1.45 "Segoe UI", Tahoma, sans-serif;
      color: var(--text);
      background: var(--bg);
    }
    .wrap {
      max-width: 1000px;
      margin: 16px auto;
      padding: 0 12px 16px;
    }
    h1 {
      margin: 0 0 10px;
      font-size: 24px;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 10px;
      box-shadow: 0 1px 2px rgba(9, 30, 66, 0.06);
      padding: 10px;
      margin-bottom: 10px;
    }
    .row {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
    }
    input, button {
      font: inherit;
      padding: 6px 8px;
      border-radius: 6px;
      border: 1px solid var(--border);
      background: #fff;
    }
    button {
      cursor: pointer;
      color: #fff;
      background: var(--accent);
      border-color: var(--accent);
    }
    #map {
      height: 68vh;
      min-height: 340px;
      border: 1px solid var(--border);
      border-radius: 8px;
    }
    .muted { color: var(--muted); }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Plexplore Map</h1>
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
          <input id="limit" value="1500" style="width:90px;">
        </label>
        <button id="load_btn" type="button">Refresh</button>
      </div>
      <div id="meta" class="muted" style="margin-top:8px;">Loading...</div>
    </div>
    <div class="card">
      <div id="map"></div>
    </div>
    <div class="card">
      <div class="row" style="justify-content:space-between;">
        <strong>Visits Summary</strong>
        <span id="visits_summary_meta" class="muted">Loading...</span>
      </div>
      <table style="margin-top:8px; width:100%; border-collapse:collapse;">
        <thead>
          <tr>
            <th style="text-align:left; border-bottom:1px solid var(--border); padding:6px;">Start (UTC)</th>
            <th style="text-align:left; border-bottom:1px solid var(--border); padding:6px;">End (UTC)</th>
            <th style="text-align:left; border-bottom:1px solid var(--border); padding:6px;">Duration</th>
            <th style="text-align:left; border-bottom:1px solid var(--border); padding:6px;">Device</th>
          </tr>
        </thead>
        <tbody id="visits_body">
          <tr><td colspan="4" class="muted" style="padding:6px;">Loading...</td></tr>
        </tbody>
      </table>
    </div>
  </div>

  <script
    src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"
    integrity="sha256-20nQCchB9co0qIjJZRGuk2/Z9VM+kNiyxNV1lvTlZBo="
    crossorigin=""
  ></script>
  <script>
    const map = L.map("map");
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: "&copy; OpenStreetMap contributors",
    }).addTo(map);

    const trackLayer = L.layerGroup().addTo(map);
    const visitLayer = L.layerGroup().addTo(map);
    map.setView([41.88, -87.63], 10);

    function formatDateUTC(date) {
      return date.toISOString().slice(0, 10);
    }

    function setDefaultDateRangeIfEmpty() {
      const fromInput = document.getElementById("date_from");
      const toInput = document.getElementById("date_to");
      if (!fromInput || !toInput) return;
      if (fromInput.value && toInput.value) return;

      const now = new Date();
      const todayUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
      const recentFromUTC = new Date(todayUTC.getTime() - (6 * 24 * 60 * 60 * 1000));

      fromInput.value = formatDateUTC(recentFromUTC);
      toInput.value = formatDateUTC(todayUTC);
    }

    function dayStartRFC3339(dateValue) {
      return dateValue + "T00:00:00Z";
    }

    function dayEndRFC3339(dateValue) {
      return dateValue + "T23:59:59.999Z";
    }

    function buildPointsQuery() {
      const params = new URLSearchParams();
      const device = document.getElementById("device_select").value.trim();
      const fromDate = document.getElementById("date_from").value.trim();
      const toDate = document.getElementById("date_to").value.trim();
      const limit = document.getElementById("limit").value.trim();
      if (device) params.set("device_id", device);
      if (fromDate) params.set("from", dayStartRFC3339(fromDate));
      if (toDate) params.set("to", dayEndRFC3339(toDate));
      if (limit) params.set("limit", limit);
      return "/api/v1/points?" + params.toString();
    }

    function buildVisitsQuery() {
      const params = new URLSearchParams();
      const device = document.getElementById("device_select").value.trim();
      const fromDate = document.getElementById("date_from").value.trim();
      const toDate = document.getElementById("date_to").value.trim();
      if (device) params.set("device_id", device);
      if (fromDate) params.set("from", dayStartRFC3339(fromDate));
      if (toDate) params.set("to", dayEndRFC3339(toDate));
      params.set("limit", "500");
      return "/api/v1/visits?" + params.toString();
    }

    function escapeHTML(value) {
      return String(value).replace(/[&<>'"]/g, (ch) => ({
        "&":"&amp;","<":"&lt;",">":"&gt;","'":"&#39;",'"':"&quot;"
      }[ch]));
    }

    function formatDuration(startRaw, endRaw) {
      const start = new Date(startRaw);
      const end = new Date(endRaw);
      if (!(start instanceof Date) || !(end instanceof Date) || isNaN(start.getTime()) || isNaN(end.getTime())) {
        return "";
      }
      const seconds = Math.max(0, Math.floor((end.getTime() - start.getTime()) / 1000));
      const hours = Math.floor(seconds / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      if (hours > 0) {
        return hours + "h " + minutes + "m";
      }
      return minutes + "m";
    }

    function renderVisitsSummary(visits) {
      const body = document.getElementById("visits_body");
      const meta = document.getElementById("visits_summary_meta");
      if (!body || !meta) return;

      if (!Array.isArray(visits) || visits.length === 0) {
        body.innerHTML = "<tr><td colspan='4' class='muted' style='padding:6px;'>No visits for current filter</td></tr>";
        meta.textContent = "0 visits";
        return;
      }

      body.innerHTML = visits.map((v) =>
        "<tr>" +
          "<td style='border-bottom:1px solid var(--border); padding:6px;'>" + escapeHTML(v.start_at || "") + "</td>" +
          "<td style='border-bottom:1px solid var(--border); padding:6px;'>" + escapeHTML(v.end_at || "") + "</td>" +
          "<td style='border-bottom:1px solid var(--border); padding:6px;'>" + escapeHTML(formatDuration(v.start_at, v.end_at)) + "</td>" +
          "<td style='border-bottom:1px solid var(--border); padding:6px;'>" + escapeHTML(v.device_id || "") + "</td>" +
        "</tr>"
      ).join("");
      meta.textContent = visits.length + " visit(s)";
    }

    async function loadDevices() {
      const select = document.getElementById("device_select");
      if (!select) return;
      try {
        const res = await fetch("/api/v1/devices", { cache: "no-store" });
        if (!res.ok) return;
        const payload = await res.json();
        const devices = (payload && payload.devices) || [];
        if (!Array.isArray(devices) || devices.length === 0) return;

        const options = ["<option value=''>All devices</option>"];
        for (const d of devices) {
          if (!d || !d.name) continue;
          options.push("<option value='" + escapeHTML(d.name) + "'>" + escapeHTML(d.name) + "</option>");
        }
        select.innerHTML = options.join("");
      } catch (_) {
        // Device list is optional for map rendering.
      }
    }

    async function loadVisits() {
      const path = buildVisitsQuery();
      visitLayer.clearLayers();
      const res = await fetch(path, { cache: "no-store" });
      if (!res.ok) throw new Error("visits HTTP " + res.status);
      const payload = await res.json();
      const visits = (payload && payload.visits) || [];

      for (const v of visits) {
        const marker = L.circleMarker([v.centroid_lat, v.centroid_lon], {
          radius: 6,
          color: "#9c27b0",
          fillColor: "#9c27b0",
          fillOpacity: 0.5,
          weight: 1,
        });
        marker.bindPopup(
          "visit #" + v.id +
          "<br>device: " + escapeHTML(v.device_id || "") +
          "<br>place: " + escapeHTML(v.place_label || "") +
          "<br>start: " + escapeHTML(v.start_at || "") +
          "<br>end: " + escapeHTML(v.end_at || "") +
          "<br>points: " + String(v.point_count || 0)
        );
        marker.addTo(visitLayer);
      }
      renderVisitsSummary(visits);
      return visits;
    }

    async function loadPointsAndVisits() {
      const meta = document.getElementById("meta");
      const pointsPath = buildPointsQuery();
      meta.textContent = "Loading " + pointsPath;

      trackLayer.clearLayers();
      visitLayer.clearLayers();
      try {
        const res = await fetch(pointsPath, { cache: "no-store" });
        if (!res.ok) throw new Error("HTTP " + res.status);
        const payload = await res.json();
        const points = (payload && payload.points) || [];
        let visitsCount = 0;
        let visitsWarning = "";
        try {
          const visits = await loadVisits();
          visitsCount = visits.length;
        } catch (visitsErr) {
          visitsWarning = " | visits unavailable: " + visitsErr.message;
          renderVisitsSummary([]);
        }

        if (!points.length) {
          if (visitsCount > 0) {
            meta.textContent = "No points for current filter | " + visitsCount + " visits shown" + visitsWarning;
          } else {
            meta.textContent = "No points for current filter | no visits for filter" + visitsWarning;
          }
          return;
        }

        const latlngs = points.map((p) => [p.lat, p.lon]);
        const poly = L.polyline(latlngs, { color: "#0b6bcb", weight: 3 }).addTo(trackLayer);
        map.fitBounds(poly.getBounds(), { padding: [16, 16] });

        if (points.length <= 500) {
          for (const p of points) {
            L.circleMarker([p.lat, p.lon], { radius: 3, weight: 1 })
              .bindPopup("seq=" + p.seq + "<br>" + p.timestamp_utc + "<br>" + p.device_id)
              .addTo(trackLayer);
          }
        }

        if (visitsCount > 0) {
          meta.textContent = "Loaded " + points.length + " points | " + visitsCount + " visits shown" + visitsWarning;
        } else {
          meta.textContent = "Loaded " + points.length + " points | no visits for filter" + visitsWarning;
        }
      } catch (err) {
        meta.textContent = "Load failed: " + err.message;
      }
    }

    document.getElementById("load_btn").addEventListener("click", loadPointsAndVisits);
    setDefaultDateRangeIfEmpty();
    loadDevices().then(loadPointsAndVisits);
  </script>
</body>
</html>
`

func registerUIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", statusPageHandler)
	mux.HandleFunc("GET /ui/status", statusPageHandler)
	mux.HandleFunc("GET /ui/map", mapPageHandler)
}

func statusPageHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, statusPageHTML)
}

func mapPageHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, mapPageHTML)
}
