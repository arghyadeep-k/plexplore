(function () {
  const csrfToken = window.PlexploreUI.csrfTokenFromMeta();
  const ownerByID = {};
  let devicesCache = [];

  function dayStartRFC3339(dateValue) {
    return dateValue + "T00:00:00Z";
  }

  function dayEndRFC3339(dateValue) {
    return dateValue + "T23:59:59.999Z";
  }

  function setStatus(id, className, text) {
    const el = document.getElementById(id);
    if (!el) {
      return;
    }
    el.className = className;
    el.textContent = text;
  }

  function jsonOrError(res, fallback) {
    if (!res.ok) {
      return res.text().then(function (text) {
        throw new Error((text || fallback || ("HTTP " + res.status)).trim());
      });
    }
    return res.json();
  }

  function ownerLabel(userID) {
    const owner = ownerByID[userID];
    if (!owner) {
      return "#" + String(userID);
    }
    const email = window.PlexploreUI.escapeHTML(owner.email || "");
    return "#" + String(userID) + " (" + email + ")";
  }

  function renderDeviceRows(devices) {
    const body = document.getElementById("devices_admin_body");
    if (!body) {
      return;
    }
    if (!Array.isArray(devices) || devices.length === 0) {
      body.innerHTML = "<tr><td colspan='7' class='muted'>No devices</td></tr>";
      return;
    }
    body.innerHTML = devices
      .map(function (d) {
        return (
          "<tr>" +
          "<td>" +
          String(d.id || "") +
          "</td>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(d.name || "") +
          "</td>" +
          "<td>" +
          ownerLabel(d.user_id) +
          "</td>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(d.created_at || "") +
          "</td>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(d.updated_at || "") +
          "</td>" +
          "<td>" +
          window.PlexploreUI.escapeHTML(d.api_key_preview || "") +
          "</td>" +
          "<td><button type='button' class='rotate-btn' data-device-id='" +
          String(d.id || "") +
          "'>Rotate Key</button></td>" +
          "</tr>"
        );
      })
      .join("");
  }

  function syncVisitDeviceOptions(devices) {
    const select = document.getElementById("visit_device_select");
    if (!select) {
      return;
    }
    const options = ["<option value=''>All devices</option>"];
    for (const d of devices) {
      if (!d || !d.id) {
        continue;
      }
      options.push(
        "<option value='" +
          String(d.id) +
          "'>" +
          window.PlexploreUI.escapeHTML(d.name || "") +
          "</option>",
      );
    }
    select.innerHTML = options.join("");
  }

  function loadUsers() {
    const ownerSelect = document.getElementById("device_owner_select");
    return fetch("/api/v1/users", { cache: "no-store" })
      .then(function (res) {
        return jsonOrError(res, "users request failed");
      })
      .then(function (payload) {
        const users = (payload && payload.users) || [];
        users.forEach(function (u) {
          ownerByID[u.id] = u;
        });
        if (!ownerSelect) {
          return;
        }
        const options = users.map(function (u) {
          return (
            "<option value='" +
            String(u.id) +
            "'>" +
            window.PlexploreUI.escapeHTML(u.email || ("user-" + String(u.id))) +
            (u.is_admin ? " (admin)" : "") +
            "</option>"
          );
        });
        ownerSelect.innerHTML = options.join("");
      });
  }

  function loadDevices() {
    return fetch("/api/v1/devices", { cache: "no-store" })
      .then(function (res) {
        return jsonOrError(res, "devices request failed");
      })
      .then(function (payload) {
        devicesCache = (payload && payload.devices) || [];
        renderDeviceRows(devicesCache);
        syncVisitDeviceOptions(devicesCache);
      });
  }

  function showRotatedKey(key) {
    const wrap = document.getElementById("rotate_result");
    const input = document.getElementById("rotated_api_key");
    if (!wrap || !input) {
      return;
    }
    input.value = key || "";
    wrap.classList.remove("hidden");
  }

  function rotateDeviceKey(deviceID) {
    setStatus("rotate_status", "muted", "Rotating key for device #" + deviceID + "...");
    return fetch("/api/v1/devices/" + encodeURIComponent(String(deviceID)) + "/rotate-key", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrfToken,
      },
      body: "{}",
    })
      .then(function (res) {
        return jsonOrError(res, "rotate failed");
      })
      .then(function (payload) {
        showRotatedKey(payload.api_key || "");
        setStatus(
          "rotate_status",
          "status-success",
          "API key rotated for device #" +
            String(payload.id || deviceID) +
            ". Save the new key now; old key is invalid.",
        );
        return loadDevices();
      })
      .catch(function (err) {
        setStatus("rotate_status", "status-error", "Rotate failed: " + err.message);
      });
  }

  function createDevice() {
    const ownerSelect = document.getElementById("device_owner_select");
    const name = (document.getElementById("device_name").value || "").trim();
    const sourceType = (document.getElementById("device_source_type").value || "").trim();
    const userID = Number((ownerSelect && ownerSelect.value) || 0);
    if (!name || !sourceType || !Number.isFinite(userID) || userID <= 0) {
      setStatus("create_device_status", "status-error", "Owner, name, and source type are required.");
      return Promise.resolve();
    }

    setStatus("create_device_status", "muted", "Creating device...");
    return fetch("/api/v1/devices", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrfToken,
      },
      body: JSON.stringify({
        user_id: userID,
        name: name,
        source_type: sourceType,
      }),
    })
      .then(function (res) {
        return jsonOrError(res, "create failed");
      })
      .then(function (payload) {
        setStatus(
          "create_device_status",
          "status-success",
          "Device created. Save API key now; it is shown once.",
        );
        showRotatedKey(payload.api_key || "");
        document.getElementById("device_name").value = "";
        return loadDevices();
      })
      .catch(function (err) {
        setStatus("create_device_status", "status-error", "Create failed: " + err.message);
      });
  }

  function generateVisitsForDevice(deviceID, fromDate, toDate) {
    const params = new URLSearchParams();
    params.set("device_id", deviceID);
    if (fromDate) {
      params.set("from", dayStartRFC3339(fromDate));
    }
    if (toDate) {
      params.set("to", dayEndRFC3339(toDate));
    }

    return fetch("/api/v1/visits/generate?" + params.toString(), {
      method: "POST",
      headers: {
        "X-CSRF-Token": csrfToken,
      },
    }).then(function (res) {
      return jsonOrError(res, "visit generation failed for " + deviceID);
    });
  }

  function generateVisits() {
    const selectedDevice = Number((document.getElementById("visit_device_select").value || "").trim());
    const fromDate = (document.getElementById("visit_from_date").value || "").trim();
    const toDate = (document.getElementById("visit_to_date").value || "").trim();
    const deviceIDs = [];
    if (Number.isFinite(selectedDevice) && selectedDevice > 0) {
      deviceIDs.push(selectedDevice);
    } else {
      devicesCache.forEach(function (d) {
        if (d && Number.isFinite(Number(d.id)) && Number(d.id) > 0) {
          deviceIDs.push(Number(d.id));
        }
      });
    }

    if (!deviceIDs.length) {
      setStatus("generate_visits_status", "status-error", "No devices available.");
      return Promise.resolve();
    }
    if (fromDate && toDate && fromDate > toDate) {
      setStatus("generate_visits_status", "status-error", "From date must be before or equal to To date.");
      return Promise.resolve();
    }

    setStatus("generate_visits_status", "muted", "Generating visits...");
    let createdTotal = 0;
    let index = 0;
    function next() {
      if (index >= deviceIDs.length) {
        setStatus(
          "generate_visits_status",
          "status-success",
          "Visit generation completed for " +
            String(deviceIDs.length) +
            " device(s); created " +
            String(createdTotal) +
            " visit(s).",
        );
        return Promise.resolve();
      }
      const deviceID = deviceIDs[index];
      index += 1;
      return generateVisitsForDevice(deviceID, fromDate, toDate)
        .then(function (payload) {
          createdTotal += Number(payload.created_visits || 0);
        })
        .then(next);
    }

    return next().catch(function (err) {
      setStatus("generate_visits_status", "status-error", "Visit generation failed: " + err.message);
    });
  }

  function bindEvents() {
    const createBtn = document.getElementById("create_device_btn");
    if (createBtn) {
      createBtn.addEventListener("click", function () {
        createDevice();
      });
    }

    const visitsBtn = document.getElementById("generate_visits_btn");
    if (visitsBtn) {
      visitsBtn.addEventListener("click", function () {
        generateVisits();
      });
    }

    const tableBody = document.getElementById("devices_admin_body");
    if (tableBody) {
      tableBody.addEventListener("click", function (event) {
        const target = event.target;
        if (!target || !target.classList || !target.classList.contains("rotate-btn")) {
          return;
        }
        const deviceID = Number(target.getAttribute("data-device-id") || 0);
        if (!Number.isFinite(deviceID) || deviceID <= 0) {
          return;
        }
        rotateDeviceKey(deviceID);
      });
    }

    const copyBtn = document.getElementById("copy_rotated_key_btn");
    if (copyBtn) {
      copyBtn.addEventListener("click", function () {
        const input = document.getElementById("rotated_api_key");
        if (!input || !input.value) {
          return;
        }
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard
            .writeText(input.value)
            .then(function () {
              setStatus("rotate_status", "status-success", "API key copied to clipboard.");
            })
            .catch(function () {
              input.select();
              setStatus("rotate_status", "status-warning", "Copy failed. Key selected; use Ctrl+C.");
            });
          return;
        }
        input.select();
        setStatus("rotate_status", "status-warning", "Clipboard API unavailable. Key selected; use Ctrl+C.");
      });
    }
  }

  window.PlexploreUI.initThemeToggle();
  bindEvents();
  loadUsers()
    .then(loadDevices)
    .catch(function (err) {
      setStatus("create_device_status", "status-error", "Initial load failed: " + err.message);
    });
})();
