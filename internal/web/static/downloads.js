/* downloads.js */
(function () {
  "use strict";

  if (!App.requireAuth()) return;
  App.initTopbar();
  App.initConfirmModal();
  lucide.createIcons();

  /* ── State ── */
  var cached = [];
  var isAdmin = false;
  var refreshTimer = null;
  var REFRESH_MS = 5000;

  var dateFmt = new Intl.DateTimeFormat(navigator.language || "en", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });

  /* ── Init ── */
  fetchAuthInfo().then(function () {
    fetchDownloads();
    if (document.getElementById("auto-refresh").checked) startRefresh();
  });

  /* ── Auth info ── */
  async function fetchAuthInfo() {
    try {
      var r = await App.apiFetch("/auth/me");
      isAdmin = !!r.is_admin;
    } catch (_) {}
  }

  /* ── Fetch ── */
  async function fetchDownloads() {
    if (cached.length === 0) showLoading(true);

    try {
      var r = await App.apiFetch("/downloads?limit=100");
      var fresh = r.data || [];

      var changed =
        fresh.length !== cached.length ||
        fresh.some(function (d, i) {
          var c = cached[i];
          return (
            !c ||
            c.id !== d.id ||
            c.filename !== d.filename ||
            c.filesize !== d.filesize ||
            c.host !== d.host ||
            c.download !== d.download ||
            c.generated !== d.generated
          );
        });

      if (changed) {
        cached = fresh;
        render();
      }
    } catch (e) {
      App.showToast(e.message || "Failed to load downloads", "error");
    } finally {
      showLoading(false);
    }
  }

  /* ── Render ── */
  function render(filterOverride) {
    var filter =
      filterOverride !== undefined
        ? filterOverride
        : document.getElementById("downloads-search").value;

    var items = filter
      ? cached.filter(function (d) {
          var q = filter.toLowerCase();
          return (
            d.filename.toLowerCase().includes(q) ||
            d.host.toLowerCase().includes(q)
          );
        })
      : cached;

    var list = document.getElementById("downloads-list");
    var empty = document.getElementById("downloads-empty");

    list.querySelectorAll(".list-item").forEach(function (el) {
      el.remove();
    });

    if (items.length === 0) {
      empty.style.display = "";
      return;
    }
    empty.style.display = "none";

    var frag = document.createDocumentFragment();
    items.forEach(function (d) {
      frag.appendChild(buildItem(d));
    });
    list.appendChild(frag);
    lucide.createIcons();
  }

  function buildItem(d) {
    var div = document.createElement("div");
    div.className = "list-item";
    div.dataset.id = d.id;

    var dateStr = "";
    try {
      dateStr = dateFmt.format(new Date(d.generated));
    } catch (_) {}

    div.innerHTML =
      '<div style="display:flex;align-items:flex-start;gap:10px">' +
      '<div style="flex:1;min-width:0">' +
      '<div style="display:flex;align-items:center;gap:6px;margin-bottom:2px">' +
      '<span class="item-name">' +
      App.escHtml(d.filename) +
      "</span>" +
      '<span class="badge badge-muted" style="flex-shrink:0">' +
      App.escHtml(d.host) +
      "</span>" +
      "</div>" +
      '<div class="item-meta">' +
      '<span class="meta">' +
      App.formatBytes(d.filesize) +
      "</span>" +
      (dateStr ? '<span class="meta">' + dateStr + "</span>" : "") +
      (d.download
        ? '<a href="' +
          App.escHtml(d.download) +
          '" target="_blank" rel="noopener noreferrer" class="dl-link">' +
          '<i data-lucide="arrow-down-to-line" style="width:11px;height:11px"></i>Download</a>'
        : "") +
      "</div>" +
      "</div>" +
      '<div class="item-actions">' +
      (isAdmin
        ? '<button class="icon-btn delete-btn" data-id="' +
          d.id +
          '" data-filename="' +
          App.escHtml(d.filename) +
          '" ' +
          'aria-label="Delete ' +
          App.escHtml(d.filename) +
          '" style="color:var(--red)">' +
          '<i data-lucide="trash-2" style="width:14px;height:14px"></i>' +
          "</button>"
        : "") +
      "</div>" +
      "</div>";

    return div;
  }

  /* ── Delete ── */
  function confirmDelete(id, filename) {
    App.showConfirm(
      "Delete download?",
      '"' + filename + '" will be removed from your download history.',
      async function () {
        try {
          await App.apiFetch("/downloads/" + id, { method: "DELETE" });
          cached = cached.filter(function (d) {
            return d.id !== id;
          });
          App.showToast("Download deleted", "success");
          render();
        } catch (e) {
          App.showToast(e.message || "Failed to delete", "error");
        }
      },
    );
  }

  /* ── Auto-refresh ── */
  function startRefresh() {
    clearInterval(refreshTimer);
    refreshTimer = setInterval(fetchDownloads, REFRESH_MS);
  }
  function stopRefresh() {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }

  document
    .getElementById("auto-refresh")
    .addEventListener("change", function () {
      if (this.checked) startRefresh();
      else stopRefresh();
    });

  document
    .getElementById("refresh-btn")
    .addEventListener("click", fetchDownloads);

  /* ── Search ── */
  document
    .getElementById("downloads-search")
    .addEventListener("input", function () {
      render(this.value);
    });

  /* ── Event delegation ── */
  document
    .getElementById("downloads-list")
    .addEventListener("click", function (e) {
      var btn = e.target.closest(".delete-btn");
      if (btn) confirmDelete(btn.dataset.id, btn.dataset.filename);
    });

  /* ── Loading state ── */
  function showLoading(show) {
    var ld = document.getElementById("downloads-loading");
    var em = document.getElementById("downloads-empty");
    if (show) {
      ld.style.display = "flex";
      em.style.display = "none";
    } else {
      ld.style.display = "none";
    }
  }
})();
