/* torrents.js */
(function () {
  "use strict";

  if (!App.requireAuth()) return;
  App.initTopbar();
  App.initConfirmModal();
  lucide.createIcons();

  /* ── State ── */
  var cached = [];
  var keptIds = new Set();
  var isAdmin = false;
  var refreshTimer = null;
  var page = {
    items: [],
    offset: 0,
    limit: 50,
    hasMore: true,
    loading: false,
    filter: "",
  };

  var REFRESH_MS = 5000;

  /* ── Init ── */
  fetchAuthInfo().then(function () {
    fetchKeptTorrents().then(function () {
      fetchTorrents(true);
    });
    if (document.getElementById("auto-refresh").checked) startRefresh();
  });

  /* ── Auth info ── */
  async function fetchAuthInfo() {
    try {
      var r = await App.apiFetch("/auth/me");
      isAdmin = !!r.is_admin;
    } catch (_) {}
  }

  /* ── Kept torrents ── */
  async function fetchKeptTorrents() {
    try {
      var r = await App.apiFetch("/kept-torrents");
      keptIds.clear();
      (r.data || []).forEach(function (t) {
        keptIds.add(t.TorrentID);
      });
    } catch (_) {}
  }

  /* ── Fetch torrents ── */
  async function fetchTorrents(reset) {
    if (reset) {
      page = {
        items: [],
        offset: 0,
        limit: 50,
        hasMore: true,
        loading: false,
        filter: page.filter,
      };
    }
    if (page.loading) return;
    page.loading = true;

    if (page.offset === 0) showLoading(true);

    try {
      var r = await App.apiFetch(
        "/torrents?limit=" + page.limit + "&offset=" + page.offset,
      );
      var newItems = r.data || [];
      var total = r.total_count || 0;

      page.hasMore = page.offset + newItems.length < total;
      page.items = page.items.concat(newItems);
      page.offset += newItems.length;
      cached = page.items;

      render();
    } catch (e) {
      App.showToast(e.message || "Failed to load torrents", "error");
    } finally {
      showLoading(false);
      page.loading = false;
      updateLoadMore();
    }
  }

  /* ── Render ── */
  function render(filterOverride) {
    var filter = filterOverride !== undefined ? filterOverride : page.filter;
    page.filter = filter;

    var items = filter
      ? cached.filter(function (t) {
          return t.filename.toLowerCase().includes(filter.toLowerCase());
        })
      : cached;

    var list = document.getElementById("torrents-list");
    var empty = document.getElementById("torrents-empty");

    /* Remove existing items (not skeleton/empty) */
    list.querySelectorAll(".list-item").forEach(function (el) {
      el.remove();
    });

    if (items.length === 0) {
      empty.style.display = "";
      return;
    }
    empty.style.display = "none";

    var frag = document.createDocumentFragment();
    items.forEach(function (t) {
      frag.appendChild(buildItem(t));
    });
    list.appendChild(frag);
    lucide.createIcons();
  }

  function buildItem(t) {
    var isKept = keptIds.has(t.id);
    var div = document.createElement("div");
    div.className = "list-item" + (isKept ? " kept" : "");
    div.dataset.id = t.id;

    var statusClass = statusBadgeClass(t.status);
    var seedText =
      t.seeders === 1 ? "1 seed" : t.seeders > 1 ? t.seeders + " seeds" : "";

    div.innerHTML =
      '<div style="display:flex;align-items:flex-start;gap:10px">' +
      '<div style="flex:1;min-width:0">' +
      '<div style="display:flex;align-items:center;gap:6px;margin-bottom:2px">' +
      '<span class="item-name">' +
      App.escHtml(t.filename) +
      "</span>" +
      (isKept
        ? '<span class="badge badge-amber" title="Kept — protected from auto-delete" style="flex-shrink:0"><i data-lucide="shield" style="width:10px;height:10px"></i></span>'
        : "") +
      "</div>" +
      '<div class="item-meta">' +
      '<span class="meta">' +
      App.formatBytes(t.bytes) +
      "</span>" +
      '<span class="badge ' +
      statusClass +
      ' item-status">' +
      App.escHtml(t.status) +
      "</span>" +
      (seedText
        ? '<span class="meta item-seeders">' + seedText + "</span>"
        : "") +
      "</div>" +
      "</div>" +
      '<div class="item-actions">' +
      buildKeepBtn(t.id, t.filename, isKept) +
      (isAdmin ? buildDeleteBtn(t.id, t.filename, "torrent") : "") +
      "</div>" +
      "</div>" +
      '<div class="progress" role="progressbar" aria-valuenow="' +
      Math.round(t.progress) +
      '" aria-valuemin="0" aria-valuemax="100">' +
      '<div class="progress-fill item-progress' +
      (t.progress >= 100 ? " done" : "") +
      '" style="width:' +
      t.progress +
      '%"></div>' +
      "</div>" +
      '<div style="display:flex;justify-content:space-between;margin-top:3px">' +
      '<span class="meta item-pct">' +
      t.progress.toFixed(1) +
      "%</span>" +
      '<span class="meta item-speed">' +
      (t.speed > 0 ? App.formatBytes(t.speed) + "/s" : "") +
      "</span>" +
      "</div>";

    return div;
  }

  function buildKeepBtn(id, filename, isKept) {
    return (
      '<button class="icon-btn keep-btn" data-id="' +
      id +
      '" data-filename="' +
      App.escHtml(filename) +
      '" ' +
      'aria-label="' +
      (isKept ? "Unkeep" : "Keep") +
      " " +
      App.escHtml(filename) +
      '">' +
      '<i data-lucide="' +
      (isKept ? "shield-check" : "shield") +
      '" style="width:14px;height:14px;color:' +
      (isKept ? "var(--amber)" : "var(--fg-muted)") +
      '"></i>' +
      "</button>"
    );
  }

  function buildDeleteBtn(id, filename, type) {
    return (
      '<button class="icon-btn delete-btn" data-id="' +
      id +
      '" data-filename="' +
      App.escHtml(filename) +
      '" data-type="' +
      type +
      '" ' +
      'aria-label="Delete ' +
      App.escHtml(filename) +
      '" style="color:var(--red)">' +
      '<i data-lucide="trash-2" style="width:14px;height:14px"></i>' +
      "</button>"
    );
  }

  function statusBadgeClass(s) {
    if (s === "Downloaded") return "badge-green";
    if (s === "Downloading") return "badge-teal";
    if (s === "Error") return "badge-red";
    return "badge-muted";
  }

  /* ── Smart update (auto-refresh: patch in-place when possible) ── */
  async function smartRefresh() {
    try {
      await fetchKeptTorrents();
      var r = await App.apiFetch("/torrents?limit=" + page.limit + "&offset=0");
      var fresh = r.data || [];

      if (document.getElementById("torrents-search").value) {
        cached = fresh;
        page.items = fresh;
        render();
        return;
      }

      var changed = fresh.length !== cached.length;
      if (!changed) {
        for (var i = 0; i < fresh.length; i++) {
          var f = fresh[i],
            c = cached[i];
          if (
            f.id !== c.id ||
            f.status !== c.status ||
            f.progress !== c.progress ||
            f.speed !== c.speed
          ) {
            changed = true;
            break;
          }
        }
      }

      if (changed) {
        cached = fresh;
        page.items = fresh;
        page.offset = fresh.length;
        page.hasMore = fresh.length < (r.total_count || 0);
        render();
        updateLoadMore();
      } else {
        /* Just patch dynamic fields in-place */
        fresh.forEach(function (t) {
          var el = document.querySelector('.list-item[data-id="' + t.id + '"]');
          if (!el) return;
          var s = el.querySelector(".item-status");
          if (s) {
            s.textContent = t.status;
            s.className =
              "badge " + statusBadgeClass(t.status) + " item-status";
          }
          var pf = el.querySelector(".item-progress");
          if (pf) {
            pf.style.width = t.progress + "%";
            pf.classList.toggle("done", t.progress >= 100);
          }
          var pct = el.querySelector(".item-pct");
          if (pct) pct.textContent = t.progress.toFixed(1) + "%";
          var spd = el.querySelector(".item-speed");
          if (spd)
            spd.textContent =
              t.speed > 0 ? App.formatBytes(t.speed) + "/s" : "";
          var sd = el.querySelector(".item-seeders");
          if (sd)
            sd.textContent =
              t.seeders === 1
                ? "1 seed"
                : t.seeders > 1
                  ? t.seeders + " seeds"
                  : "";
        });
      }
    } catch (_) {}
  }

  /* ── Keep / Unkeep ── */
  async function toggleKeep(id, filename) {
    var kept = keptIds.has(id);
    try {
      if (kept) {
        await App.apiFetch("/torrents/" + id + "/keep", { method: "DELETE" });
        keptIds.delete(id);
        App.showToast("Removed from kept", "success");
      } else {
        await App.apiFetch("/torrents/" + id + "/keep", { method: "POST" });
        keptIds.add(id);
        App.showToast("Added to kept", "success");
      }
      var el = document.querySelector('.list-item[data-id="' + id + '"]');
      if (el) {
        var newKept = keptIds.has(id);
        el.classList.toggle("kept", newKept);

        /* Replace keep button */
        var oldBtn = el.querySelector(".keep-btn");
        if (oldBtn) {
          var newBtn = document.createElement("button");
          newBtn.className = "icon-btn keep-btn";
          newBtn.dataset.id = id;
          newBtn.dataset.filename = filename;
          newBtn.setAttribute(
            "aria-label",
            (newKept ? "Unkeep" : "Keep") + " " + filename,
          );
          newBtn.innerHTML =
            '<i data-lucide="' +
            (newKept ? "shield-check" : "shield") +
            '" style="width:14px;height:14px;color:' +
            (newKept ? "var(--amber)" : "var(--fg-muted)") +
            '"></i>';
          oldBtn.replaceWith(newBtn);
          lucide.createIcons();
        }

        /* Update / remove amber badge */
        var badge = el.querySelector(
          ".item-name + .badge, .item-name ~ .badge",
        );
        /* simpler: check inside the row */
        var nameRow = el.querySelector("[data-id]");
        var existingBadge = el.querySelector(".badge-amber[title]");
        if (newKept && !existingBadge) {
          var nameDiv = el.querySelector('[style*="align-items:center"]');
          if (nameDiv) {
            var badgeEl = document.createElement("span");
            badgeEl.className = "badge badge-amber";
            badgeEl.title = "Kept — protected from auto-delete";
            badgeEl.style.flexShrink = "0";
            badgeEl.innerHTML =
              '<i data-lucide="shield" style="width:10px;height:10px"></i>';
            nameDiv.appendChild(badgeEl);
            lucide.createIcons();
          }
        } else if (!newKept && existingBadge) {
          existingBadge.remove();
        }
      }
    } catch (e) {
      App.showToast(e.message || "Failed to update keep status", "error");
    }
  }

  /* ── Delete ── */
  function confirmDelete(id, filename) {
    App.showConfirm(
      "Delete torrent?",
      '"' +
        filename +
        '" will be permanently deleted from Real-Debrid. This cannot be undone.',
      async function () {
        try {
          await App.apiFetch("/torrents/" + id, { method: "DELETE" });
          cached = cached.filter(function (t) {
            return t.id !== id;
          });
          page.items = cached;
          App.showToast("Torrent deleted", "success");
          render();
        } catch (e) {
          App.showToast(e.message || "Failed to delete", "error");
        }
      },
    );
  }

  /* ── Load more ── */
  document
    .getElementById("load-more-btn")
    .addEventListener("click", function () {
      if (!page.hasMore || page.loading) return;
      fetchTorrents(false);
    });

  function updateLoadMore() {
    var el = document.getElementById("load-more");
    var btn = document.getElementById("load-more-btn");
    if (page.hasMore) {
      el.style.display = "";
      btn.textContent = "Load more";
    } else if (cached.length > 0) {
      el.style.display = "";
      btn.textContent = "All " + cached.length + " loaded";
      btn.disabled = true;
    } else {
      el.style.display = "none";
    }
  }

  /* ── Infinite scroll ── */
  document
    .getElementById("torrents-list")
    .addEventListener("scroll", function () {
      var el = this;
      if (el.scrollHeight - el.scrollTop - el.clientHeight < 120) {
        if (page.hasMore && !page.loading) fetchTorrents(false);
      }
    });

  /* ── Auto-refresh ── */
  function startRefresh() {
    clearInterval(refreshTimer);
    refreshTimer = setInterval(smartRefresh, REFRESH_MS);
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

  document.getElementById("refresh-btn").addEventListener("click", function () {
    fetchKeptTorrents().then(function () {
      fetchTorrents(true);
    });
  });

  /* ── Search ── */
  document
    .getElementById("torrents-search")
    .addEventListener("input", function () {
      render(this.value);
    });

  /* ── Event delegation for keep/delete buttons ── */
  document
    .getElementById("torrents-list")
    .addEventListener("click", function (e) {
      var keepBtn = e.target.closest(".keep-btn");
      if (keepBtn) {
        toggleKeep(keepBtn.dataset.id, keepBtn.dataset.filename);
        return;
      }

      var delBtn = e.target.closest(".delete-btn");
      if (delBtn) {
        confirmDelete(delBtn.dataset.id, delBtn.dataset.filename);
      }
    });

  /* ── Loading state ── */
  function showLoading(show) {
    var ld = document.getElementById("torrents-loading");
    var em = document.getElementById("torrents-empty");
    if (show) {
      ld.style.display = "flex";
      em.style.display = "none";
    } else {
      ld.style.display = "none";
    }
  }
})();
