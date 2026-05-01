/* dashboard.js */
(function () {
  "use strict";

  if (!App.requireAuth()) return;
  App.initTopbar();
  lucide.createIcons();

  /* ── Init ── */
  Promise.all([fetchStatus(), fetchAuthInfo(), fetchStats()]);

  /* ── Account status ── */
  async function fetchStatus() {
    try {
      var r = await App.apiFetch("/status");
      var d = r.data || {};

      var grid = document.getElementById("status-grid");
      var type = d.type || "free";
      var isPrem = type.toLowerCase() === "premium";

      var html = "";

      /* Type */
      html += '<div class="status-item">';
      html += '<div class="status-key">Plan</div>';
      html +=
        '<div class="status-val ' +
        (isPrem ? "teal" : "") +
        '">' +
        App.escHtml(type) +
        "</div>";
      html += "</div>";

      /* Premium until */
      if (d.premium) {
        var days = Math.round(d.premium / 86400);
        html += '<div class="status-item">';
        html += '<div class="status-key">Expires in</div>';
        html +=
          '<div class="status-val ' +
          (days < 14 ? "amber" : "green") +
          '">' +
          days +
          " days</div>";
        html += "</div>";
      }

      /* Points */
      if (d.points !== undefined && d.points !== null) {
        html += '<div class="status-item">';
        html += '<div class="status-key">Points</div>';
        html += '<div class="status-val">' + d.points + "</div>";
        html += "</div>";
      }

      /* Username */
      if (d.username) {
        html += '<div class="status-item">';
        html += '<div class="status-key">Account</div>';
        html +=
          '<div class="status-val" style="font-size:14px">' +
          App.escHtml(d.username) +
          "</div>";
        html += "</div>";
      }

      grid.innerHTML = html;
    } catch (e) {
      document.getElementById("status-grid").innerHTML =
        '<span style="font-size:13px;color:var(--fg-muted)">Failed to load status</span>';
    }
  }

  /* ── Stats ── */
  async function fetchStats() {
    try {
      var r = await App.apiFetch("/stats");
      var d = r.data || {};
      var grid = document.getElementById("stats-grid");

      var html = "";

      html += '<div class="status-item">';
      html += '<div class="status-key">Torrents</div>';
      html += '<div class="status-val">' + (d.torrents_total || 0) + "</div>";
      html += "</div>";

      if (d.torrents_active !== undefined) {
        html += '<div class="status-item">';
        html += '<div class="status-key">Active</div>';
        html +=
          '<div class="status-val">' +
          d.torrents_active +
          (d.torrents_limit ? " / " + d.torrents_limit : "") +
          "</div>";
        html += "</div>";
      }

      html += '<div class="status-item">';
      html += '<div class="status-key">Downloading</div>';
      html += '<div class="status-val teal">' + (d.torrents_downloading || 0) + "</div>";
      html += "</div>";

      html += '<div class="status-item">';
      html += '<div class="status-key">Downloaded</div>';
      html += '<div class="status-val green">' + (d.torrents_downloaded || 0) + "</div>";
      html += "</div>";

      if (d.torrents_kept !== undefined) {
        html += '<div class="status-item">';
        html += '<div class="status-key">Kept</div>';
        html += '<div class="status-val amber">' + d.torrents_kept + "</div>";
        html += "</div>";
      }

      if (d.torrents_bytes) {
        html += '<div class="status-item">';
        html += '<div class="status-key">Size</div>';
        var sizeNote =
          d.torrents_sample < d.torrents_total ? "~" : "";
        html +=
          '<div class="status-val" title="' +
          (d.torrents_sample < d.torrents_total
            ? "Estimated from first " + d.torrents_sample + " torrents"
            : "Total size of all torrents") +
          '">' +
          sizeNote +
          App.formatBytes(d.torrents_bytes) +
          "</div>";
        html += "</div>";
      }

      if (d.downloads_total !== undefined) {
        html += '<div class="status-item">';
        html += '<div class="status-key">Downloads</div>';
        html += '<div class="status-val">' + d.downloads_total + "</div>";
        html += "</div>";
      }

      grid.innerHTML = html;
    } catch (_) {
      document.getElementById("stats-grid").innerHTML =
        '<span style="font-size:13px;color:var(--fg-muted)">Failed to load stats</span>';
    }
  }

  /* ── Auth info (admin check) ── */
  async function fetchAuthInfo() {
    try {
      var r = await App.apiFetch("/auth/me");
      if (r.is_admin) {
        document.getElementById("autodelete-section").style.display = "";
        fetchAutoDeleteSetting();
      }
    } catch (_) {}
  }

  /* ── Auto-Delete ── */
  async function fetchAutoDeleteSetting() {
    try {
      var r = await App.apiFetch("/settings/autodelete");
      var days = parseInt(r.data || "0", 10);
      var valEl = document.getElementById("autodelete-value");
      var inpEl = document.getElementById("autodelete-input");
      if (!isNaN(days) && days > 0) {
        valEl.textContent = days + (days === 1 ? " day" : " days");
        if (inpEl) inpEl.value = days;
      } else {
        valEl.textContent = "Not set";
        if (inpEl) inpEl.value = "";
      }
    } catch (_) {}
  }

  document
    .getElementById("autodelete-save")
    .addEventListener("click", async function () {
      var inp = document.getElementById("autodelete-input");
      var btn = this;
      var days = parseInt(inp.value, 10);
      if (isNaN(days) || days < 0) return;

      btn.disabled = true;
      btn.textContent = "Saving…";

      try {
        await App.apiFetch("/settings/autodelete", {
          method: "PUT",
          body: JSON.stringify({ value: String(days) }),
        });
        var valEl = document.getElementById("autodelete-value");
        valEl.textContent =
          days > 0 ? days + (days === 1 ? " day" : " days") : "Not set";
        App.showToast("Saved", "success");
      } catch (e) {
        App.showToast(e.message || "Failed to save", "error");
      } finally {
        btn.disabled = false;
        btn.textContent = "Save";
      }
    });

  /* ── Add Torrent ── */
  document
    .getElementById("add-torrent-form")
    .addEventListener("submit", async function (e) {
      e.preventDefault();
      var inp = document.getElementById("magnet-link");
      var btn = document.getElementById("add-torrent-btn");
      var link = inp.value.trim();
      if (!link) return;

      btn.disabled = true;
      btn.textContent = "…";

      try {
        await App.apiFetch("/torrents", {
          method: "POST",
          body: JSON.stringify({ magnet: link }),
        });
        inp.value = "";
        App.showToast("Torrent added", "success");
      } catch (e) {
        App.showToast(e.message || "Failed to add torrent", "error");
      } finally {
        btn.disabled = false;
        btn.textContent = "Add";
      }
    });

  /* ── Unrestrict Link ── */
  document
    .getElementById("unrestrict-form")
    .addEventListener("submit", async function (e) {
      e.preventDefault();
      var inp = document.getElementById("hoster-link");
      var btn = document.getElementById("unrestrict-btn");
      var link = inp.value.trim();
      if (!link) return;

      btn.disabled = true;
      btn.textContent = "…";

      try {
        await App.apiFetch("/unrestrict", {
          method: "POST",
          body: JSON.stringify({ link: link }),
        });
        inp.value = "";
        App.showToast("Link unrestricted — check Downloads", "success");
      } catch (e) {
        App.showToast(e.message || "Failed to unrestrict link", "error");
      } finally {
        btn.disabled = false;
        btn.textContent = "Unlock";
      }
    });

  /* ── Check Domain ── */
  document
    .getElementById("check-domain-form")
    .addEventListener("submit", async function (e) {
      e.preventDefault();
      var inp = document.getElementById("domain-input");
      var btn = document.getElementById("check-domain-btn");
      var res = document.getElementById("domain-result");
      var domain = inp.value.trim().toLowerCase();
      if (!domain) return;

      btn.disabled = true;
      res.style.display = "none";

      try {
        var r = await App.apiFetch(
          "/check-domain?domain=" + encodeURIComponent(domain),
        );
        var shown = r.checked_domain || domain;
        res.style.display = "";
        if (r.supported) {
          res.innerHTML =
            '<span style="color:var(--green)">✓ ' +
            App.escHtml(shown) +
            " is supported</span>";
        } else {
          res.innerHTML =
            '<span style="color:var(--red)">✗ ' +
            App.escHtml(shown) +
            " is not supported</span>";
        }
      } catch (e) {
        App.showToast(e.message || "Failed to check domain", "error");
      } finally {
        btn.disabled = false;
      }
    });
})();
