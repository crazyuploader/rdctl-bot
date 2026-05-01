/* app.js — shared utilities for rdctl-bot */
(function (global) {
  "use strict";

  const THEME_KEY = "rdctl-theme";
  const API = "/api";

  /* ── SVG icons ── */
  const moonSVG =
    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>';
  const sunSVG =
    '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>';

  /* ── Theme ── */
  function _getTheme() {
    const s = localStorage.getItem(THEME_KEY);
    if (s === "dark" || s === "light") return s;
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }

  function _applyTheme(t) {
    document.documentElement.setAttribute("data-theme", t);
    document.documentElement.style.colorScheme = t;
    const btn = document.getElementById("theme-btn");
    if (btn) {
      btn.innerHTML = t === "dark" ? moonSVG : sunSVG;
      btn.setAttribute(
        "aria-label",
        "Switch to " + (t === "dark" ? "light" : "dark") + " mode",
      );
    }
  }

  function initTheme() {
    const t = _getTheme();
    _applyTheme(t);
    const btn = document.getElementById("theme-btn");
    if (btn) {
      btn.addEventListener("click", () => {
        const next =
          document.documentElement.getAttribute("data-theme") === "dark"
            ? "light"
            : "dark";
        try {
          localStorage.setItem(THEME_KEY, next);
        } catch (_) {}
        _applyTheme(next);
      });
    }
  }

  /* ── Auth ── */
  function _getAuthHeaders() {
    const tok = localStorage.getItem("authToken");
    const key = localStorage.getItem("apiKey");
    if (tok) return { Authorization: "Bearer " + tok };
    if (key) return { "X-API-Key": key };
    return {};
  }

  function requireAuth() {
    if (!localStorage.getItem("authToken") && !localStorage.getItem("apiKey")) {
      window.location.href = "/";
      return false;
    }
    return true;
  }

  function logout() {
    localStorage.removeItem("authToken");
    localStorage.removeItem("apiKey");
    window.location.href = "/";
  }

  /* ── apiFetch ── */
  async function apiFetch(path, opts) {
    opts = opts || {};
    const headers = Object.assign(
      { "Content-Type": "application/json" },
      _getAuthHeaders(),
      opts.headers || {},
    );
    const res = await fetch(API + path, Object.assign({}, opts, { headers }));

    if (res.status === 401) {
      logout();
      throw new Error("Session expired");
    }

    if (!res.ok) {
      let msg = "Request failed (" + res.status + ")";
      try {
        const d = await res.json();
        if (d && d.error) msg = d.error;
        else if (d && d.message) msg = d.message;
      } catch (_) {}
      throw new Error(msg);
    }

    return res.json();
  }

  /* ── Toast ── */
  let _toastTimer = null;
  function showToast(msg, type) {
    const el = document.getElementById("toast");
    if (!el) return;
    clearTimeout(_toastTimer);
    el.textContent = msg;
    el.className = "toast" + (type ? " " + type : "");
    void el.offsetWidth;
    el.classList.add("show");
    _toastTimer = setTimeout(() => el.classList.remove("show"), 3500);
  }

  /* ── Formatters ── */
  function formatBytes(b) {
    if (!b || b === 0) return "0 B";
    const u = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(b) / Math.log(1024));
    return (b / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1) + " " + u[i];
  }

  function escHtml(s) {
    if (!s) return "";
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  /* ── Topbar init (called on authenticated pages) ── */
  function initTopbar() {
    initTheme();
    const logoutBtn = document.getElementById("logout-btn");
    if (logoutBtn) logoutBtn.addEventListener("click", logout);

    /* Highlight active nav link */
    const path = window.location.pathname;
    document.querySelectorAll(".topbar-nav a").forEach(function (a) {
      if (a.getAttribute("href") === path) a.classList.add("active");
    });
  }

  /* ── Delete confirm modal ── */
  let _confirmCb = null;
  let _lastFocus = null;

  function initConfirmModal() {
    const overlay = document.getElementById("confirm-modal");
    if (!overlay) return;

    document
      .getElementById("confirm-cancel")
      .addEventListener("click", _hideConfirm);
    document
      .getElementById("confirm-ok")
      .addEventListener("click", function () {
        if (_confirmCb) _confirmCb();
        _hideConfirm();
      });

    overlay.addEventListener("click", function (e) {
      if (e.target === overlay) _hideConfirm();
    });

    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && overlay.classList.contains("show"))
        _hideConfirm();
    });

    /* Focus trap */
    overlay.addEventListener("keydown", function (e) {
      if (e.key !== "Tab") return;
      const els = Array.from(
        overlay.querySelectorAll(
          'button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])',
        ),
      );
      if (!els.length) return;
      if (e.shiftKey) {
        if (document.activeElement === els[0]) {
          e.preventDefault();
          els[els.length - 1].focus();
        }
      } else {
        if (document.activeElement === els[els.length - 1]) {
          e.preventDefault();
          els[0].focus();
        }
      }
    });
  }

  function showConfirm(title, msg, cb) {
    _lastFocus = document.activeElement;
    _confirmCb = cb;
    document.getElementById("confirm-title").textContent = title;
    document.getElementById("confirm-message").textContent = msg;
    const overlay = document.getElementById("confirm-modal");
    overlay.classList.add("show");
    requestAnimationFrame(function () {
      const first = overlay.querySelector("button");
      if (first) first.focus();
    });
  }

  function _hideConfirm() {
    _confirmCb = null;
    document.getElementById("confirm-modal").classList.remove("show");
    if (_lastFocus) {
      _lastFocus.focus();
      _lastFocus = null;
    }
  }

  /* ── Export ── */
  global.App = {
    initTheme,
    initTopbar,
    requireAuth,
    logout,
    apiFetch,
    showToast,
    formatBytes,
    escHtml,
    initConfirmModal,
    showConfirm,
  };
})(window);
