// ─── State ────────────────────────────────────────────────────────────────────
let cachedTorrents = [];
let cachedDownloads = [];
let keptTorrentIds = new Set();
let isAdmin = false;
let currentTab = "torrents";
let refreshIntervals = { torrents: null, downloads: null };
let confirmCallback = null;
let lastFocusedElement = null;

// ─── Config ───────────────────────────────────────────────────────────────────
const API_BASE_URL = "/api";
const REFRESH_INTERVAL_MS = 5000;

// ─── Init ─────────────────────────────────────────────────────────────────────
document.addEventListener("DOMContentLoaded", () => {
  lucide.createIcons();
  checkLogin();
  setupEventListeners();
  setupTabs();
  setupModalKeyboard();
});

// ─── Login / Auth ─────────────────────────────────────────────────────────────
async function checkLogin() {
  // Check for authorization code in URL (from dashboard link)
  const params = new URLSearchParams(window.location.search);
  const code = params.get("code");

  if (code) {
    // Exchange code for token
    try {
      const response = await fetch(`${API_BASE_URL}/exchange-token`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code }),
      });

      if (response.ok) {
        const result = await response.json();
        if (result.token) {
          window.authToken = result.token;
          localStorage.setItem("authToken", result.token);

          // Remove code from URL
          params.delete("code");
          const newUrl = params.toString()
            ? `${window.location.pathname}?${params.toString()}`
            : window.location.pathname;
          history.replaceState(null, "", newUrl);

          try {
            await fetchAuthInfo();
            showApp();
            await fetchAllData();
            startAutoRefresh();
          } catch (error) {
            // handleLogout already called on 401
          }
          return;
        }
      }
    } catch (error) {
      console.error("Token exchange failed:", error);
    }
  }

  // Fall back to stored token or API key
  const authToken = localStorage.getItem("authToken");
  const apiKey = localStorage.getItem("apiKey");

  if (authToken) {
    window.authToken = authToken;
  } else if (apiKey) {
    window.apiKey = apiKey;
  } else {
    return;
  }

  try {
    await fetchAuthInfo();
    showApp();
    await fetchAllData();
    startAutoRefresh();
  } catch (error) {
    // handleLogout already called on 401
  }
}

async function handleLogin(e) {
  e.preventDefault();
  const input = document.getElementById("api-key-input");
  const btn = document.getElementById("login-btn");
  const apiKey = input.value.trim();
  if (!apiKey) return;

  // Show spinner, disable button
  btn.classList.add("loading");
  btn.disabled = true;

  window.apiKey = apiKey;
  localStorage.setItem("apiKey", apiKey);

  try {
    await fetchAuthInfo();
    showApp();
    await fetchAllData();
    startAutoRefresh();
  } catch (error) {
    // Revert on failure
    localStorage.removeItem("apiKey");
    window.apiKey = null;
    document.getElementById("login-screen").classList.remove("hidden");
    document.getElementById("app-screen").classList.add("hidden");
    showToast("Invalid API key — verify your key and try again", "error");
    input.focus();
  } finally {
    btn.classList.remove("loading");
    btn.disabled = false;
  }
}

function handleLogout() {
  localStorage.removeItem("apiKey");
  localStorage.removeItem("authToken");
  window.apiKey = null;
  window.authToken = null;
  stopAutoRefresh();
  cachedTorrents = [];
  cachedDownloads = [];
  keptTorrentIds.clear();
  isAdmin = false;
  document.getElementById("login-screen").classList.remove("hidden");
  document.getElementById("app-screen").classList.add("hidden");
  document.getElementById("autodelete-card").classList.add("hidden");
  document.getElementById("user-greeting").textContent = "";
  // Clear lists
  document
    .querySelectorAll(
      "#torrents-list .torrent-item, #downloads-list .torrent-item",
    )
    .forEach((el) => el.remove());
  document.getElementById("torrents-empty").classList.remove("hidden");
  document.getElementById("downloads-empty").classList.remove("hidden");
  // Focus API key input
  setTimeout(() => document.getElementById("api-key-input").focus(), 50);
}

function showApp() {
  document.getElementById("login-screen").classList.add("hidden");
  document.getElementById("app-screen").classList.remove("hidden");
}

// ─── Data Fetching ────────────────────────────────────────────────────────────
async function fetchAllData() {
  // Fetch kept torrent IDs first so renderTorrents() already has them when it runs
  await fetchKeptTorrents();
  // Stagger remaining requests to avoid exceeding rate limiter
  await Promise.all([fetchStatus(), fetchTorrents()]);
  await Promise.all([fetchDownloads(), fetchAutoDeleteSetting()]);
}

async function fetchAuthInfo() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/auth/me`);
    isAdmin = result.is_admin;

    const greeting = document.getElementById("user-greeting");
    if (result.username) {
      greeting.textContent = `@${result.username}`;
    } else if (result.user_id) {
      greeting.textContent = `User #${result.user_id}`;
    }

    if (isAdmin) {
      document.getElementById("autodelete-card").classList.remove("hidden");
    }
  } catch (error) {
    console.error("Auth error:", error);
    throw error;
  }
}

async function fetchStatus() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/status`);
    const badge = document.getElementById("status-badge");

    if (result.data && result.data.type === "premium") {
      badge.className = "badge badge-success";
      badge.innerHTML =
        '<i data-lucide="crown" class="w-3 h-3" aria-hidden="true"></i><span>Premium</span>';
    } else {
      badge.className = "badge badge-muted";
      badge.innerHTML =
        '<i data-lucide="user" class="w-3 h-3" aria-hidden="true"></i><span>Free</span>';
    }
    lucide.createIcons();
  } catch (error) {
    console.error("Status error:", error);
  }
}

// ─── Torrents ─────────────────────────────────────────────────────────────────
async function fetchTorrents() {
  if (cachedTorrents.length === 0) showLoading("torrents", true);
  try {
    const result = await apiFetch(`${API_BASE_URL}/torrents?limit=100`);
    const newTorrents = result.data || [];

    if (cachedTorrents.length > 0) {
      const needsFullRender = smartUpdateTorrents(newTorrents);
      cachedTorrents = newTorrents;
      if (needsFullRender) renderTorrents();
      // Always sync keep icons even if we didn't do a full render
      updateKeepStatus();
    } else {
      cachedTorrents = newTorrents;
      renderTorrents();
    }
  } catch (error) {
    showToast("Failed to load torrents — check your connection", "error");
  } finally {
    showLoading("torrents", false);
  }
}

function renderTorrents(filterStr) {
  const list = document.getElementById("torrents-list");
  const empty = document.getElementById("torrents-empty");
  const searchInput = document.getElementById("torrents-search");
  const filter =
    filterStr !== undefined ? filterStr : searchInput ? searchInput.value : "";

  const filtered = filter
    ? cachedTorrents.filter((t) =>
        t.filename.toLowerCase().includes(filter.toLowerCase()),
      )
    : cachedTorrents;

  if (filtered.length === 0) {
    empty.classList.remove("hidden");
    Array.from(list.children).forEach((c) => {
      if (c.id !== "torrents-empty" && c.id !== "torrents-loading") c.remove();
    });
    return;
  }

  empty.classList.add("hidden");

  const html = filtered
    .map((t) => {
      const isKept = keptTorrentIds.has(t.id);
      const statusClass =
        t.status === "Downloaded"
          ? "badge-success"
          : t.status === "Downloading"
            ? "badge-info"
            : t.status === "Error"
              ? "badge-error"
              : "badge-muted";

      const seedLabel =
        t.seeders === 1
          ? "1\u00a0seed"
          : t.seeders > 1
            ? `${t.seeders}\u00a0seeds`
            : "";

      return `
        <div class="torrent-item ${isKept ? "kept" : ""}" data-id="${t.id}">
          <div class="flex items-start justify-between gap-4">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                <span class="truncate text-sm font-medium">${escapeHtml(t.filename)}</span>
                ${isKept ? '<span class="badge badge-info badge-kept" data-badge="kept" title="Kept — protected from auto-delete"><i data-lucide="shield" class="w-3 h-3" aria-hidden="true"></i></span>' : ""}
              </div>
              <div class="flex items-center gap-3 text-xs text-[#71717a]" style="font-variant-numeric: tabular-nums">
                <span>${formatBytes(t.bytes)}</span>
                <span class="torrent-status ${statusClass}">${t.status}</span>
                <span class="torrent-seeders">${seedLabel}</span>
              </div>
            </div>
            <div class="flex items-center gap-1">
              <button
                data-id="${t.id}"
                data-filename="${escapeHtml(t.filename)}"
                onclick="toggleKeep(this.dataset.id, this.dataset.filename)"
                class="btn btn-ghost p-2"
                aria-label="${isKept ? "Unkeep" : "Keep"} ${escapeHtml(t.filename)}"
              >
                <i data-lucide="${isKept ? "shield-check" : "shield"}" class="w-4 h-4" aria-hidden="true"></i>
              </button>
              ${
                isAdmin
                  ? `
                <button
                  data-id="${t.id}"
                  data-filename="${escapeHtml(t.filename)}"
                  onclick="confirmDelete('torrent', this.dataset.id, this.dataset.filename)"
                  class="btn btn-ghost p-2 text-[#f87171]"
                  aria-label="Delete ${escapeHtml(t.filename)}"
                >
                  <i data-lucide="trash-2" class="w-4 h-4" aria-hidden="true"></i>
                </button>
              `
                  : ""
              }
            </div>
          </div>
          <div class="mt-3">
            <div class="progress-bar" role="progressbar" aria-valuenow="${t.progress.toFixed(0)}" aria-valuemin="0" aria-valuemax="100" aria-label="${t.progress.toFixed(1)}% downloaded">
              <div class="progress-fill ${t.progress >= 100 ? "complete" : ""}" style="width: ${t.progress}%"></div>
            </div>
            <div class="flex justify-between mt-1 text-xs text-[#71717a]" style="font-variant-numeric: tabular-nums">
              <span class="torrent-progress-text">${t.progress.toFixed(1)}%</span>
              <span class="torrent-speed">${t.speed > 0 ? `${formatBytes(t.speed)}/s` : ""}</span>
            </div>
          </div>
        </div>
      `;
    })
    .join("");

  document
    .querySelectorAll("#torrents-list .torrent-item")
    .forEach((el) => el.remove());
  list.insertAdjacentHTML("beforeend", html);
  lucide.createIcons();
}

function smartUpdateTorrents(newTorrents) {
  if (cachedTorrents.length !== newTorrents.length) return true;

  const searchInput = document.getElementById("torrents-search");
  if (searchInput && searchInput.value) return true;

  for (let i = 0; i < newTorrents.length; i++) {
    const oldT = cachedTorrents[i];
    const newT = newTorrents[i];

    if (oldT.id !== newT.id) return true;

    if (
      oldT.status !== newT.status ||
      oldT.progress !== newT.progress ||
      oldT.speed !== newT.speed ||
      oldT.seeders !== newT.seeders
    ) {
      const card = document.querySelector(
        `.torrent-item[data-id="${newT.id}"]`,
      );
      if (card) {
        const statusEl = card.querySelector(".torrent-status");
        if (statusEl) {
          statusEl.textContent = newT.status;
          statusEl.className = `torrent-status ${
            newT.status === "Downloaded"
              ? "badge-success"
              : newT.status === "Downloading"
                ? "badge-info"
                : newT.status === "Error"
                  ? "badge-error"
                  : "badge-muted"
          }`;
        }

        const seedersEl = card.querySelector(".torrent-seeders");
        if (seedersEl) {
          seedersEl.textContent =
            newT.seeders === 1
              ? "1\u00a0seed"
              : newT.seeders > 1
                ? `${newT.seeders}\u00a0seeds`
                : "";
        }

        const fillEl = card.querySelector(".progress-fill");
        if (fillEl) {
          fillEl.style.width = `${newT.progress}%`;
          fillEl.classList.toggle("complete", newT.progress >= 100);
          const pb = fillEl.closest(".progress-bar");
          if (pb) {
            pb.setAttribute("aria-valuenow", newT.progress.toFixed(0));
            pb.setAttribute(
              "aria-label",
              `${newT.progress.toFixed(1)}% downloaded`,
            );
          }
        }

        const progText = card.querySelector(".torrent-progress-text");
        if (progText) progText.textContent = `${newT.progress.toFixed(1)}%`;

        const speedEl = card.querySelector(".torrent-speed");
        if (speedEl)
          speedEl.textContent =
            newT.speed > 0 ? `${formatBytes(newT.speed)}/s` : "";
      }
    }
  }
  return false;
}

// ─── Downloads ────────────────────────────────────────────────────────────────
async function fetchDownloads() {
  if (cachedDownloads.length === 0) showLoading("downloads", true);
  try {
    const result = await apiFetch(`${API_BASE_URL}/downloads?limit=100`);
    const newDownloads = result.data || [];

    // Field-level diff instead of JSON.stringify
    const changed =
      newDownloads.length !== cachedDownloads.length ||
      newDownloads.some((d, i) => {
        const old = cachedDownloads[i];
        return (
          !old ||
          old.id !== d.id ||
          old.filename !== d.filename ||
          old.filesize !== d.filesize ||
          old.host !== d.host ||
          old.download !== d.download ||
          old.generated !== d.generated
        );
      });

    if (changed) {
      cachedDownloads = newDownloads;
      renderDownloads();
    }
  } catch (error) {
    showToast("Failed to load downloads — check your connection", "error");
  } finally {
    showLoading("downloads", false);
  }
}

function renderDownloads(filterStr) {
  const list = document.getElementById("downloads-list");
  const empty = document.getElementById("downloads-empty");
  const searchInput = document.getElementById("downloads-search");
  const filter =
    filterStr !== undefined ? filterStr : searchInput ? searchInput.value : "";

  const filtered = filter
    ? cachedDownloads.filter(
        (d) =>
          d.filename.toLowerCase().includes(filter.toLowerCase()) ||
          d.host.toLowerCase().includes(filter.toLowerCase()),
      )
    : cachedDownloads;

  if (filtered.length === 0) {
    empty.classList.remove("hidden");
    Array.from(list.children).forEach((c) => {
      if (c.id !== "downloads-empty" && c.id !== "downloads-loading")
        c.remove();
    });
    return;
  }

  empty.classList.add("hidden");

  const fmt = new Intl.DateTimeFormat(navigator.language || "en", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });

  const html = filtered
    .map(
      (d) => `
      <div class="torrent-item" data-id="${d.id}">
        <div class="flex items-start justify-between gap-4">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span
                class="truncate text-sm font-medium"
              >${escapeHtml(d.filename)}</span>
              <span class="badge badge-muted">${escapeHtml(d.host)}</span>
            </div>
            <div class="flex items-center gap-3 text-xs text-[#71717a]" style="font-variant-numeric: tabular-nums">
              <span>${formatBytes(d.filesize)}</span>
              <span>${fmt.format(new Date(d.generated))}</span>
            </div>
          </div>
          <div class="flex items-center gap-1">
            ${
              isAdmin
                ? `
              <button
                data-id="${d.id}"
                data-filename="${escapeHtml(d.filename)}"
                onclick="confirmDelete('download', this.dataset.id, this.dataset.filename)"
                class="btn btn-ghost p-2 text-[#f87171]"
                aria-label="Delete ${escapeHtml(d.filename)}"
              >
                <i data-lucide="trash-2" class="w-4 h-4" aria-hidden="true"></i>
              </button>
            `
                : ""
            }
          </div>
        </div>
      </div>
    `,
    )
    .join("");

  document
    .querySelectorAll("#downloads-list .torrent-item")
    .forEach((el) => el.remove());
  list.insertAdjacentHTML("beforeend", html);
  lucide.createIcons();
}

// ─── Kept Torrents ────────────────────────────────────────────────────────────
async function fetchKeptTorrents() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/kept-torrents`);
    keptTorrentIds.clear();
    (result.data || []).forEach((t) => keptTorrentIds.add(t.TorrentID));
    updateKeepStatus();
  } catch (error) {
    console.error("Failed to fetch kept torrents:", error);
  }
}

function updateKeepStatus() {
  document.querySelectorAll(".torrent-item").forEach((item) => {
    const id = item.dataset.id;
    const isKept = keptTorrentIds.has(id);
    item.classList.toggle("kept", isKept);

    const btn = item.querySelector('button[onclick*="toggleKeep"]');
    if (btn) {
      const filename = btn.dataset.filename || "";
      btn.innerHTML = `<i data-lucide="${isKept ? "shield-check" : "shield"}" class="w-4 h-4" aria-hidden="true"></i>`;
      btn.setAttribute(
        "aria-label",
        `${isKept ? "Unkeep" : "Keep"} ${filename}`,
      );
    }

    const existingBadge = item.querySelector('[data-badge="kept"]');
    if (isKept && !existingBadge) {
      const titleDiv = item.querySelector(".flex.items-center.gap-2.mb-1");
      if (titleDiv) {
        titleDiv.insertAdjacentHTML(
          "beforeend",
          '<span class="badge badge-info badge-kept" data-badge="kept" title="Kept — protected from auto-delete"><i data-lucide="shield" class="w-3 h-3" aria-hidden="true"></i></span>',
        );
        lucide.createIcons();
      }
    } else if (!isKept && existingBadge) {
      existingBadge.remove();
    }
  });
  // Re-render all lucide icon placeholders into actual SVGs
  lucide.createIcons();
}

async function toggleKeep(id, filename) {
  try {
    if (keptTorrentIds.has(id)) {
      await apiFetch(`${API_BASE_URL}/torrents/${id}/keep`, {
        method: "DELETE",
      });
      keptTorrentIds.delete(id);
      showToast("Removed from kept", "success");
    } else {
      await apiFetch(`${API_BASE_URL}/torrents/${id}/keep`, { method: "POST" });
      keptTorrentIds.add(id);
      showToast("Added to kept", "success");
    }
    updateKeepStatus();
  } catch (error) {
    showToast(error.message || "Failed to update keep status", "error");
  }
}

// ─── Auto-Delete ──────────────────────────────────────────────────────────────
async function fetchAutoDeleteSetting() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/settings/autodelete`);
    const daysStr = result.data || "0";
    const days = parseInt(daysStr, 10);
    const valueEl = document.getElementById("autodelete-value");
    const inputEl = document.getElementById("autodelete-input");

    if (!isNaN(days) && days > 0) {
      valueEl.textContent = `${days}\u00a0${days === 1 ? "day" : "days"}`;
      if (inputEl) inputEl.value = days;
    } else {
      valueEl.textContent = "Not set";
      if (inputEl) inputEl.value = 0;
    }
  } catch (error) {
    console.error("Auto-delete fetch error:", error);
  }
}

async function saveAutoDelete() {
  const input = document.getElementById("autodelete-input");
  const btn = document.getElementById("autodelete-save");
  const days = parseInt(input.value, 10);
  if (isNaN(days) || days < 0) return;

  btn.textContent = "Saving…";
  btn.disabled = true;

  try {
    await apiFetch(`${API_BASE_URL}/settings/autodelete`, {
      method: "PUT",
      body: JSON.stringify({ value: days.toString() }),
    });
    const valueEl = document.getElementById("autodelete-value");
    valueEl.textContent =
      days > 0 ? `${days}\u00a0${days === 1 ? "day" : "days"}` : "Not set";
    showToast("Auto-Delete saved", "success");
  } catch (error) {
    showToast(error.message || "Failed to save — try again", "error");
  } finally {
    btn.textContent = "Save";
    btn.disabled = false;
  }
}

// ─── Delete Confirmation ──────────────────────────────────────────────────────
function confirmDelete(type, id, filename) {
  lastFocusedElement = document.activeElement;

  const label = type === "torrent" ? "Torrent" : "Download";
  document.getElementById("confirm-title").textContent = `Delete ${label}?`;
  document.getElementById("confirm-message").textContent =
    `"${filename}" will be permanently deleted. This cannot be undone.`;

  // Scope callback to this invocation
  confirmCallback = async () => {
    try {
      await apiFetch(`${API_BASE_URL}/${type}s/${id}`, { method: "DELETE" });
      showToast(`${label} deleted`, "success");
      if (type === "torrent") {
        cachedTorrents = cachedTorrents.filter((t) => t.id !== id);
        renderTorrents();
      } else {
        cachedDownloads = cachedDownloads.filter((d) => d.id !== id);
        renderDownloads();
      }
    } catch (error) {
      showToast(
        error.message || `Failed to delete ${label.toLowerCase()} — try again`,
        "error",
      );
    }
    hideConfirmModal();
  };

  showConfirmModal();
}

function showConfirmModal() {
  const overlay = document.getElementById("confirm-modal");
  overlay.classList.add("show");
  overlay.removeAttribute("hidden");
  overlay.setAttribute("aria-hidden", "false");
  overlay.removeAttribute("inert");
  // Focus first focusable element in modal
  requestAnimationFrame(() => {
    const focusable = overlay.querySelectorAll(
      'button, [href], input, [tabindex]:not([tabindex="-1"])',
    );
    if (focusable.length) focusable[0].focus();
  });
}

function hideConfirmModal() {
  const overlay = document.getElementById("confirm-modal");
  overlay.classList.remove("show");
  overlay.setAttribute("hidden", "true");
  overlay.setAttribute("aria-hidden", "true");
  overlay.setAttribute("inert", "true");
  confirmCallback = null;
  // Restore focus to triggering element
  if (lastFocusedElement) {
    lastFocusedElement.focus();
    lastFocusedElement = null;
  }
}

// ─── Modal Keyboard Handling ──────────────────────────────────────────────────
function setupModalKeyboard() {
  const overlay = document.getElementById("confirm-modal");

  // Escape to close
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && overlay.classList.contains("show")) {
      hideConfirmModal();
    }
  });

  // Focus trap
  overlay.addEventListener("keydown", (e) => {
    if (e.key !== "Tab") return;
    const focusable = Array.from(
      overlay.querySelectorAll(
        'button:not([disabled]), [href], input:not([disabled]), [tabindex]:not([tabindex="-1"])',
      ),
    );
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    if (e.shiftKey) {
      if (document.activeElement === first) {
        e.preventDefault();
        last.focus();
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  });

  // Click confirm-ok
  document.getElementById("confirm-ok").addEventListener("click", () => {
    if (confirmCallback) confirmCallback();
  });
}

// ─── Tabs ─────────────────────────────────────────────────────────────────────
// ─── Tabs ─────────────────────────────────────────────────────────────────────
function setupTabs() {
  const tabs = document.querySelectorAll(".tab");
  tabs.forEach((tab) => {
    tab.addEventListener("click", () => switchTab(tab.dataset.tab));
  });

  // Roving tabindex arrow key navigation
  const tablist = document.querySelector('[role="tablist"]');
  if (tablist) {
    tablist.addEventListener("keydown", (e) => {
      const tabsArr = Array.from(tabs);
      const activeIdx = tabsArr.findIndex((t) =>
        t.classList.contains("active"),
      );

      let nextIdx = activeIdx;
      if (e.key === "ArrowRight" || e.key === "ArrowDown") {
        nextIdx = (activeIdx + 1) % tabsArr.length;
        e.preventDefault();
      } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
        nextIdx = (activeIdx - 1 + tabsArr.length) % tabsArr.length;
        e.preventDefault();
      } else {
        return;
      }

      const targetTab = tabsArr[nextIdx];
      switchTab(targetTab.dataset.tab);
      targetTab.focus();
    });
  }

  // Sync tab from URL on load
  const params = new URLSearchParams(window.location.search);
  const tabParam = params.get("tab");
  if (tabParam === "downloads") switchTab("downloads");
}

function switchTab(name) {
  currentTab = name;

  // Update URL without reload
  const params = new URLSearchParams(window.location.search);
  if (name === "torrents") {
    params.delete("tab");
  } else {
    params.set("tab", name);
  }
  const newUrl = params.toString()
    ? `${window.location.pathname}?${params.toString()}`
    : window.location.pathname;
  history.replaceState(null, "", newUrl);

  // Update tab styles + ARIA + tabindex
  document.querySelectorAll(".tab").forEach((t) => {
    const isActive = t.dataset.tab === name;
    t.classList.toggle("active", isActive);
    t.setAttribute("aria-selected", isActive ? "true" : "false");
    t.setAttribute("tabindex", isActive ? "0" : "-1");
  });

  // Show/hide panels
  document
    .getElementById("torrents-panel")
    .classList.toggle("hidden", name !== "torrents");
  document
    .getElementById("downloads-panel")
    .classList.toggle("hidden", name !== "downloads");
}

// ─── Loading States ───────────────────────────────────────────────────────────
function showLoading(type, show) {
  const empty = document.getElementById(`${type}-empty`);
  const loading = document.getElementById(`${type}-loading`);
  if (show) {
    empty.classList.add("hidden");
    loading.classList.remove("hidden");
  } else {
    loading.classList.add("hidden");
  }
}

// ─── Auto Refresh ─────────────────────────────────────────────────────────────
function startAutoRefresh() {
  const torrentsCheckbox = document.getElementById("auto-refresh-torrents");
  const downloadsCheckbox = document.getElementById("auto-refresh-downloads");
  if (torrentsCheckbox?.checked) toggleAutoRefresh("torrents", true);
  if (downloadsCheckbox?.checked) toggleAutoRefresh("downloads", true);
}

function stopAutoRefresh() {
  clearInterval(refreshIntervals.torrents);
  clearInterval(refreshIntervals.downloads);
  refreshIntervals.torrents = null;
  refreshIntervals.downloads = null;
}

function toggleAutoRefresh(type, enabled) {
  clearInterval(refreshIntervals[type]);
  refreshIntervals[type] = null;
  if (enabled) {
    refreshIntervals[type] = setInterval(() => {
      if (type === "torrents") {
        fetchKeptTorrents();
        fetchTorrents();
      } else {
        fetchDownloads();
      }
    }, REFRESH_INTERVAL_MS);
  }
}

// ─── Event Listeners ──────────────────────────────────────────────────────────
function setupEventListeners() {
  document.getElementById("login-form").addEventListener("submit", handleLogin);
  document.getElementById("logout-btn").addEventListener("click", handleLogout);

  // Add torrent
  document
    .getElementById("add-torrent-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();
      const input = document.getElementById("magnet-link");
      const btn = document.getElementById("add-torrent-btn");
      const link = input.value.trim();
      if (!link) return;

      btn.disabled = true;
      try {
        await apiFetch(`${API_BASE_URL}/torrents`, {
          method: "POST",
          body: JSON.stringify({ magnet: link }),
        });
        input.value = "";
        showToast("Torrent added", "success");
        fetchTorrents();
      } catch (error) {
        showToast(
          error.message || "Failed to add torrent — check the magnet link",
          "error",
        );
      } finally {
        btn.disabled = false;
      }
    });

  // Unrestrict link
  document
    .getElementById("unrestrict-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();
      const input = document.getElementById("hoster-link");
      const btn = document.getElementById("unrestrict-btn");
      const link = input.value.trim();
      if (!link) return;

      btn.disabled = true;
      try {
        await apiFetch(`${API_BASE_URL}/unrestrict`, {
          method: "POST",
          body: JSON.stringify({ link }),
        });
        input.value = "";
        showToast("Link unlocked", "success");
        fetchDownloads();
      } catch (error) {
        showToast(
          error.message || "Failed to unrestrict link — check the URL",
          "error",
        );
      } finally {
        btn.disabled = false;
      }
    });

  // Auto-delete save
  document
    .getElementById("autodelete-save")
    .addEventListener("click", saveAutoDelete);

  // Refresh buttons
  document
    .getElementById("refresh-torrents")
    .addEventListener("click", () => fetchTorrents());
  document
    .getElementById("refresh-downloads")
    .addEventListener("click", () => fetchDownloads());

  // Auto-refresh toggles
  document
    .getElementById("auto-refresh-torrents")
    .addEventListener("change", (e) => {
      toggleAutoRefresh("torrents", e.target.checked);
    });
  document
    .getElementById("auto-refresh-downloads")
    .addEventListener("change", (e) => {
      toggleAutoRefresh("downloads", e.target.checked);
    });

  // Search
  document.getElementById("torrents-search").addEventListener("input", (e) => {
    renderTorrents(e.target.value);
  });
  document.getElementById("downloads-search").addEventListener("input", (e) => {
    renderDownloads(e.target.value);
  });

  // Cancel modal button
  document
    .getElementById("confirm-cancel")
    .addEventListener("click", hideConfirmModal);

  // Close modal on backdrop click
  document.getElementById("confirm-modal").addEventListener("click", (e) => {
    if (e.target === e.currentTarget) hideConfirmModal();
  });
}

// ─── API ──────────────────────────────────────────────────────────────────────
async function apiFetch(url, options = {}) {
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };

  // Prefer token-based auth if available, fall back to API key
  if (window.authToken) {
    headers["Authorization"] = `Bearer ${window.authToken}`;
  } else if (window.apiKey) {
    headers["X-API-Key"] = window.apiKey;
  }

  const res = await fetch(url, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    handleLogout();
    throw new Error("Session expired — please sign in again");
  }

  if (!res.ok) {
    let message = `Request failed (${res.status})`;
    try {
      const data = await res.json();
      if (data?.message) message = data.message;
      else if (data?.error) message = data.error;
    } catch (_) {}
    throw new Error(message);
  }

  return res.json();
}

// ─── Toast ────────────────────────────────────────────────────────────────────
let toastTimer = null;
function showToast(message, type = "info") {
  const toast = document.getElementById("toast");
  clearTimeout(toastTimer);

  toast.textContent = message;
  toast.className = `toast${type ? ` ${type}` : ""}`;

  // Force reflow to restart transition
  void toast.offsetWidth;
  toast.classList.add("show");

  toastTimer = setTimeout(() => {
    toast.classList.remove("show");
  }, 3500);
}

// ─── Utilities ────────────────────────────────────────────────────────────────
function formatBytes(bytes) {
  if (!bytes || bytes === 0) return "0\u00a0B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const value = (bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1);
  return `${value}\u00a0${units[i]}`;
}

function escapeHtml(str) {
  if (!str) return "";
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}
