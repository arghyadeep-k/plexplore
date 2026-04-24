(function () {
  const themeKey = "plexplore.theme";

  function preferredTheme() {
    const stored = localStorage.getItem(themeKey);
    if (stored === "light" || stored === "dark") {
      return stored;
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }

  function applyTheme(theme) {
    const root = document.documentElement;
    root.setAttribute("data-theme", theme);
    const btn = document.getElementById("theme_toggle");
    if (!btn) {
      return;
    }
    const isDark = theme === "dark";
    btn.setAttribute("aria-pressed", isDark ? "true" : "false");
    btn.textContent = isDark ? "☀" : "🌙";
    btn.title = isDark ? "Switch to light mode" : "Switch to dark mode";
  }

  function initThemeToggle() {
    applyTheme(preferredTheme());
    const btn = document.getElementById("theme_toggle");
    if (!btn) {
      return;
    }
    btn.addEventListener("click", function () {
      const current = document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
      const next = current === "dark" ? "light" : "dark";
      localStorage.setItem(themeKey, next);
      applyTheme(next);
    });
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>'"]/g, function (ch) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" }[ch];
    });
  }

  function csrfTokenFromMeta() {
    const meta = document.querySelector('meta[name="csrf-token"]');
    if (!meta) {
      return "";
    }
    return (meta.getAttribute("content") || "").trim();
  }

  window.PlexploreUI = {
    escapeHTML: escapeHTML,
    initThemeToggle: initThemeToggle,
    csrfTokenFromMeta: csrfTokenFromMeta,
  };
})();
