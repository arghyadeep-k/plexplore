(function () {
  function getJSON(path) {
    return fetch(path, { cache: "no-store" }).then(function (res) {
      if (!res.ok) {
        throw new Error(path + " status " + res.status);
      }
      return res.json();
    });
  }

  function setText(id, value, cls) {
    const el = document.getElementById(id);
    if (!el) {
      return;
    }
    el.textContent = value;
    el.classList.remove("ok", "warn", "bad");
    if (cls) {
      el.classList.add(cls);
    }
  }

  function renderDevices(devices) {
    const body = document.getElementById("devices_body");
    if (!body) {
      return;
    }
    if (!devices || devices.length === 0) {
      body.innerHTML = "<tr><td colspan='4' class='muted'>No devices</td></tr>";
      return;
    }
    body.innerHTML = devices
      .map(function (d) {
        return (
          "<tr><td>" +
          d.id +
          "</td><td>" +
          window.PlexploreUI.escapeHTML(d.name) +
          "</td><td>" +
          window.PlexploreUI.escapeHTML(d.source_type) +
          "</td><td>" +
          d.user_id +
          "</td></tr>"
        );
      })
      .join("");
  }

  function renderDevicesUnavailable(message) {
    const body = document.getElementById("devices_body");
    if (!body) {
      return;
    }
    body.innerHTML =
      "<tr><td colspan='4' class='muted'>Unavailable: " +
      window.PlexploreUI.escapeHTML(message) +
      "</td></tr>";
  }

  function formatCoord(value) {
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return "";
    }
    return value.toFixed(5);
  }

  function renderPoints(points) {
    const body = document.getElementById("points_body");
    if (!body) {
      return;
    }
    if (!points || points.length === 0) {
      body.innerHTML = "<tr><td colspan='5' class='muted'>No points</td></tr>";
      return;
    }
    body.innerHTML = points
      .map(function (p) {
        return (
          "<tr><td>" +
          p.seq +
          "</td><td>" +
          window.PlexploreUI.escapeHTML(p.device_id || "") +
          "</td><td>" +
          window.PlexploreUI.escapeHTML(p.timestamp_utc || "") +
          "</td><td>" +
          formatCoord(p.lat) +
          "</td><td>" +
          formatCoord(p.lon) +
          "</td></tr>"
        );
      })
      .join("");
  }

  function renderPointsUnavailable(message) {
    const body = document.getElementById("points_body");
    if (!body) {
      return;
    }
    body.innerHTML =
      "<tr><td colspan='5' class='muted'>Unavailable: " +
      window.PlexploreUI.escapeHTML(message) +
      "</td></tr>";
  }

  function refresh() {
    const updated = document.getElementById("updated");
    return Promise.all([getJSON("/health"), getJSON("/api/v1/status")])
      .then(function (results) {
        const health = results[0];
        const status = results[1];

        setText("health", health.status === "ok" ? "OK" : "DEGRADED", health.status === "ok" ? "ok" : "warn");
        setText("buffer_points", String(status.buffer_points || 0));
        setText("buffer_bytes", String(status.buffer_bytes || 0));
        setText("buffer_age", String(status.oldest_buffered_age_seconds || 0) + "s");
        setText("spool_segments", String(status.spool_segment_count || 0));
        setText("checkpoint_seq", String(status.checkpoint_seq || 0));

        if (status.last_flush) {
          setText("flush_status", status.last_flush.success ? "Success" : "Failed", status.last_flush.success ? "ok" : "bad");
          const meta = [];
          if (status.last_flush.at_utc) {
            meta.push(status.last_flush.at_utc);
          }
          if (status.last_flush.error) {
            meta.push(status.last_flush.error);
          }
          setText("flush_meta", meta.join(" | "));
        } else {
          setText("flush_status", "Not yet", "warn");
          setText("flush_meta", "");
        }

        let deviceWarning = "";
        let pointsWarning = "";

        return getJSON("/api/v1/devices")
          .then(function (devicesResp) {
            renderDevices((devicesResp && devicesResp.devices) || []);
          })
          .catch(function (devicesErr) {
            deviceWarning = " | devices: " + devicesErr.message;
            renderDevicesUnavailable(devicesErr.message);
          })
          .then(function () {
            return getJSON("/api/v1/points/recent?limit=10")
              .then(function (pointsResp) {
                renderPoints((pointsResp && pointsResp.points) || []);
              })
              .catch(function (pointsErr) {
                pointsWarning = " | points: " + pointsErr.message;
                renderPointsUnavailable(pointsErr.message);
              });
          })
          .then(function () {
            if (updated) {
              updated.textContent = "Updated: " + new Date().toLocaleString() + deviceWarning + pointsWarning;
            }
          });
      })
      .catch(function (err) {
        setText("health", "ERROR", "bad");
        if (updated) {
          updated.textContent = "Update failed: " + err.message;
        }
      });
  }

  window.PlexploreUI.initThemeToggle();
  refresh();
  setInterval(refresh, 5000);
})();
