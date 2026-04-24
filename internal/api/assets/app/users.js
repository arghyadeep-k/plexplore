(function () {
  const csrfToken = window.PlexploreUI.csrfTokenFromMeta();

  function loadUsers() {
    const body = document.getElementById("users_body");
    return fetch("/api/v1/users", { cache: "no-store" })
      .then(function (res) {
        if (!res.ok) {
          throw new Error("users HTTP " + res.status);
        }
        return res.json();
      })
      .then(function (payload) {
        const users = (payload && payload.users) || [];
        if (!users.length) {
          body.innerHTML = "<tr><td colspan='4' class='muted'>No users</td></tr>";
          return;
        }
        body.innerHTML = users
          .map(function (u) {
            return (
              "<tr><td>" +
              u.id +
              "</td><td>" +
              window.PlexploreUI.escapeHTML(u.email || "") +
              "</td><td>" +
              (u.is_admin ? "yes" : "no") +
              "</td><td>" +
              window.PlexploreUI.escapeHTML(u.created_at || "") +
              "</td></tr>"
            );
          })
          .join("");
      });
  }

  function createUser() {
    const status = document.getElementById("create_status");
    const email = document.getElementById("email").value.trim();
    const password = document.getElementById("password").value.trim();
    const isAdmin = document.getElementById("is_admin").checked;
    if (!email || !password) {
      status.className = "status-error";
      status.textContent = "Email and password are required.";
      return Promise.resolve();
    }
    status.className = "muted";
    status.textContent = "Creating...";

    return fetch("/api/v1/users", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": csrfToken,
      },
      body: JSON.stringify({ email: email, password: password, is_admin: isAdmin }),
    }).then(function (res) {
      if (!res.ok) {
        return res.text().then(function (text) {
          status.className = "status-error";
          status.textContent = "Create failed: " + text;
        });
      }
      status.className = "status-success";
      status.textContent = "User created.";
      document.getElementById("email").value = "";
      document.getElementById("password").value = "";
      document.getElementById("is_admin").checked = false;
      return loadUsers();
    });
  }

  document.getElementById("create_btn").addEventListener("click", function () {
    createUser().catch(function (err) {
      const status = document.getElementById("create_status");
      status.className = "status-error";
      status.textContent = "Create failed: " + err.message;
    });
  });

  window.PlexploreUI.initThemeToggle();
  loadUsers().catch(function (err) {
    const body = document.getElementById("users_body");
    body.innerHTML =
      "<tr><td colspan='4' class='muted'>Load failed: " +
      window.PlexploreUI.escapeHTML(err.message) +
      "</td></tr>";
  });
})();
