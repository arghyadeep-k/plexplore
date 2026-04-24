(function () {
  function formatDateUTC(date) {
    return date.toISOString().slice(0, 10);
  }

  function setDefaultDateRangeIfEmpty() {
    const fromInput = document.getElementById("date_from");
    const toInput = document.getElementById("date_to");
    if (!fromInput || !toInput || (fromInput.value && toInput.value)) {
      return;
    }
    const now = new Date();
    const todayUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
    const recentFromUTC = new Date(todayUTC.getTime() - 6 * 24 * 60 * 60 * 1000);
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
    const limitRaw = document.getElementById("limit").value.trim();
    const requestedMaxPoints = Math.max(100, Number(limitRaw) || 1500);
    let fetchLimit = requestedMaxPoints;
    let simplify = false;
    let simplifyMaxPoints = requestedMaxPoints;

    if (requestedMaxPoints <= 2000) {
      simplify = false;
      fetchLimit = requestedMaxPoints;
      simplifyMaxPoints = requestedMaxPoints;
    } else if (requestedMaxPoints <= 8000) {
      simplify = true;
      fetchLimit = requestedMaxPoints;
      simplifyMaxPoints = 2000;
    } else {
      simplify = true;
      fetchLimit = requestedMaxPoints;
      simplifyMaxPoints = 1000;
    }

    if (device) {
      params.set("device_id", device);
    }
    if (fromDate) {
      params.set("from", dayStartRFC3339(fromDate));
    }
    if (toDate) {
      params.set("to", dayEndRFC3339(toDate));
    }
    params.set("limit", String(fetchLimit));
    params.set("simplify", simplify ? "true" : "false");
    params.set("max_points", String(simplifyMaxPoints));
    return "/api/v1/points?" + params.toString();
  }

  function buildVisitsQuery() {
    const params = new URLSearchParams();
    const device = document.getElementById("device_select").value.trim();
    const fromDate = document.getElementById("date_from").value.trim();
    const toDate = document.getElementById("date_to").value.trim();
    if (device) {
      params.set("device_id", device);
    }
    if (fromDate) {
      params.set("from", dayStartRFC3339(fromDate));
    }
    if (toDate) {
      params.set("to", dayEndRFC3339(toDate));
    }
    params.set("limit", "500");
    return "/api/v1/visits?" + params.toString();
  }

  function formatDuration(startRaw, endRaw) {
    const start = new Date(startRaw);
    const end = new Date(endRaw);
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
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
    if (!body || !meta) {
      return;
    }
    if (!Array.isArray(visits) || visits.length === 0) {
      body.innerHTML = "<tr><td colspan='4' class='muted'>No visits for current filter</td></tr>";
      meta.textContent = "0 visits";
      return;
    }

    body.innerHTML = visits
      .map(function (v) {
        return (
          "<tr>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(v.start_at || "") +
          "</td>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(v.end_at || "") +
          "</td>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(formatDuration(v.start_at, v.end_at)) +
          "</td>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(v.device_id || "") +
          "</td>" +
          "</tr>"
        );
      })
      .join("");
    meta.textContent = visits.length + " visit(s)";
  }

  function configureMapTiles(map) {
    const mapEl = document.getElementById("map");
    const mode = (mapEl.getAttribute("data-tile-mode") || "").trim().toLowerCase();
    const template = (mapEl.getAttribute("data-tile-url-template") || "").trim();
    const attribution = (mapEl.getAttribute("data-tile-attribution") || "").trim();
    const meta = document.getElementById("meta");

    if (mode === "none") {
      if (meta) {
        meta.textContent = "Map tiles disabled (privacy mode). Track/visit overlays remain available.";
      }
      return;
    }
    if ((mode === "osm" || mode === "custom") && template !== "") {
      L.tileLayer(template, {
        maxZoom: 19,
        attribution: attribution,
      }).addTo(map);
      return;
    }
    if (meta) {
      meta.textContent = "Map tiles unavailable: invalid map tile configuration.";
    }
  }

  const map = L.map("map");
  const trackLayer = L.layerGroup().addTo(map);
  const visitLayer = L.layerGroup().addTo(map);
  map.setView([41.88, -87.63], 10);
  configureMapTiles(map);

  function loadDevices() {
    const select = document.getElementById("device_select");
    if (!select) {
      return Promise.resolve();
    }
    return fetch("/api/v1/devices", { cache: "no-store" })
      .then(function (res) {
        if (!res.ok) {
          return null;
        }
        return res.json();
      })
      .then(function (payload) {
        const devices = (payload && payload.devices) || [];
        if (!Array.isArray(devices) || devices.length === 0) {
          return;
        }
        const options = ["<option value=''>All devices</option>"];
        for (const d of devices) {
          if (!d || !d.name) {
            continue;
          }
          const escaped = window.PlexploreUI.escapeHTML(d.name);
          options.push("<option value='" + escaped + "'>" + escaped + "</option>");
        }
        select.innerHTML = options.join("");
      })
      .catch(function () {
        return undefined;
      });
  }

  function loadVisits() {
    const path = buildVisitsQuery();
    visitLayer.clearLayers();
    return fetch(path, { cache: "no-store" })
      .then(function (res) {
        if (!res.ok) {
          throw new Error("visits HTTP " + res.status);
        }
        return res.json();
      })
      .then(function (payload) {
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
            "visit #" +
              v.id +
              "<br>device: " +
              window.PlexploreUI.escapeHTML(v.device_id || "") +
              "<br>place: " +
              window.PlexploreUI.escapeHTML(v.place_label || "") +
              "<br>start: " +
              window.PlexploreUI.escapeHTML(v.start_at || "") +
              "<br>end: " +
              window.PlexploreUI.escapeHTML(v.end_at || "") +
              "<br>points: " +
              String(v.point_count || 0),
          );
          marker.addTo(visitLayer);
        }
        renderVisitsSummary(visits);
        return visits;
      });
  }

  function loadPointsAndVisits() {
    const meta = document.getElementById("meta");
    const samplingNote = document.getElementById("sampling_note");
    const pointsPath = buildPointsQuery();
    meta.textContent = "Loading " + pointsPath;
    if (samplingNote) {
      samplingNote.textContent = "";
    }

    trackLayer.clearLayers();
    visitLayer.clearLayers();

    return fetch(pointsPath, { cache: "no-store" })
      .then(function (res) {
        if (!res.ok) {
          throw new Error("HTTP " + res.status);
        }
        return res.json();
      })
      .then(function (payload) {
            const points = (payload && payload.points) || [];
            const sampled = Boolean(payload && payload.sampled);
            const sampledFrom = Number((payload && payload.sampled_from) || 0);
            let visitsCount = 0;
            let visitsWarning = "";
        return loadVisits()
          .then(function (visits) {
            visitsCount = visits.length;
          })
          .catch(function (visitsErr) {
            visitsWarning = " | visits unavailable: " + visitsErr.message;
            renderVisitsSummary([]);
          })
          .then(function () {
            if (!points.length) {
              meta.textContent =
                (visitsCount > 0
                  ? "No points for current filter | " + visitsCount + " visits shown"
                  : "No points for current filter | no visits for filter") + visitsWarning;
              return;
            }

            const latlngs = points.map(function (p) {
              return [p.lat, p.lon];
            });
            const poly = L.polyline(latlngs, { color: "#0b6bcb", weight: 3 }).addTo(trackLayer);
            map.fitBounds(poly.getBounds(), { padding: [16, 16] });

            let markerMode = "none";
            if (points.length <= 500) {
              markerMode = "full";
              for (const p of points) {
                L.circleMarker([p.lat, p.lon], { radius: 3, weight: 1 })
                  .bindPopup(
                    "seq=" +
                      String(p.seq) +
                      "<br>" +
                      window.PlexploreUI.escapeHTML(p.timestamp_utc || "") +
                      "<br>" +
                      window.PlexploreUI.escapeHTML(p.device_id || ""),
                  )
                  .addTo(trackLayer);
              }
            } else if (points.length <= 3000) {
              markerMode = "clustered";
              renderClusteredMarkers(points);
            }

            meta.textContent =
              (visitsCount > 0
                ? "Loaded " + points.length + " points | " + visitsCount + " visits shown"
                : "Loaded " + points.length + " points | no visits for filter") + visitsWarning;
            if (samplingNote) {
              const notes = [];
              if (sampled && sampledFrom > points.length) {
                notes.push("Track sampled: showing " + points.length + " of " + sampledFrom + " fetched points.");
              }
              if (markerMode === "clustered") {
                notes.push("Markers are clustered for performance.");
              } else if (markerMode === "none") {
                notes.push("Markers hidden for very large result sets; polyline still shows route.");
              }
              samplingNote.textContent = notes.join(" ");
            }
          });
      })
      .catch(function (err) {
        meta.textContent = "Load failed: " + err.message;
      });
  }

  function renderClusteredMarkers(points) {
    const zoom = Math.max(1, map.getZoom());
    const cellDeg = Math.max(0.001, (360 / (Math.pow(2, zoom) * 256)) * 36);
    const clusters = new Map();

    for (const p of points) {
      const keyLat = Math.round(p.lat / cellDeg);
      const keyLon = Math.round(p.lon / cellDeg);
      const key = keyLat + ":" + keyLon;
      let bucket = clusters.get(key);
      if (!bucket) {
        bucket = { count: 0, latSum: 0, lonSum: 0 };
        clusters.set(key, bucket);
      }
      bucket.count += 1;
      bucket.latSum += p.lat;
      bucket.lonSum += p.lon;
    }

    clusters.forEach(function (bucket) {
      const lat = bucket.latSum / bucket.count;
      const lon = bucket.lonSum / bucket.count;
      if (bucket.count <= 1) {
        L.circleMarker([lat, lon], { radius: 3, weight: 1 }).addTo(trackLayer);
        return;
      }
      const radius = Math.max(6, Math.min(15, 4 + Math.log2(bucket.count + 1) * 2));
      L.circleMarker([lat, lon], {
        radius: radius,
        color: "#0b6bcb",
        fillColor: "#0b6bcb",
        fillOpacity: 0.35,
        weight: 1,
      })
        .bindPopup("clustered points: " + String(bucket.count))
        .addTo(trackLayer);
    });
  }

  window.PlexploreUI.initThemeToggle();
  setDefaultDateRangeIfEmpty();
  document.getElementById("load_btn").addEventListener("click", function () {
    loadPointsAndVisits();
  });
  loadDevices().then(loadPointsAndVisits);
})();
