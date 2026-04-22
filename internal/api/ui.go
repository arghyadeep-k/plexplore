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

func registerUIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", statusPageHandler)
	mux.HandleFunc("GET /ui/status", statusPageHandler)
}

func statusPageHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, statusPageHTML)
}
