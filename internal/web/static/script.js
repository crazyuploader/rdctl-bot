// State
let cachedTorrents = [];
let cachedDownloads = [];
let keptTorrentIds = new Set();
let isAdmin = false;
let currentTab = "torrents";
let refreshIntervals = { torrents: null, downloads: null };

// Config
const API_BASE_URL = "/api";

// Initialize
document.addEventListener("DOMContentLoaded", () => {
  lucide.createIcons();
  checkLogin();
  setupEventListeners();
  setupTabs();
});

// Check if already logged in
function checkLogin() {
  const apiKey = localStorage.getItem("apiKey");
  if (apiKey) {
    window.apiKey = apiKey;
    showApp();
    fetchAllData();
    startAutoRefresh();
  }
}

// Setup event listeners
function setupEventListeners() {
  // Login form
  document.getElementById("login-form").addEventListener("submit", handleLogin);

  // Logout
  document.getElementById("logout-btn").addEventListener("click", handleLogout);

  // Add torrent
  document
    .getElementById("add-torrent-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();
      const input = document.getElementById("magnet-link");
      const link = input.value.trim();
      if (!link) return;

      try {
        showLoading("torrents", true);
        await apiFetch(`${API_BASE_URL}/torrents`, {
          method: "POST",
          body: JSON.stringify({ link }),
        });
        input.value = "";
        showToast("Torrent added", "success");
        fetchTorrents();
      } catch (error) {
        showToast(error.message, "error");
      }
    });

  // Unrestrict link
  document
    .getElementById("unrestrict-form")
    .addEventListener("submit", async (e) => {
      e.preventDefault();
      const input = document.getElementById("hoster-link");
      const link = input.value.trim();
      if (!link) return;

      try {
        await apiFetch(`${API_BASE_URL}/unrestrict`, {
          method: "POST",
          body: JSON.stringify({ link }),
        });
        input.value = "";
        showToast("Link unlocked", "success");
        fetchDownloads();
      } catch (error) {
        showToast(error.message, "error");
      }
    });

  // Auto-delete
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

  // Confirm modal
  document
    .getElementById("confirm-cancel")
    .addEventListener("click", hideConfirmModal);
}

// Tab switching
function setupTabs() {
  document.querySelectorAll(".tab").forEach((tab) => {
    tab.addEventListener("click", () => {
      const tabName = tab.dataset.tab;
      switchTab(tabName);
    });
  });
}

function switchTab(name) {
  currentTab = name;

  // Update tab styles
  document
    .querySelectorAll(".tab")
    .forEach((t) => t.classList.remove("active"));
  document.querySelector(`[data-tab="${name}"]`).classList.add("active");

  // Show/hide panels
  document
    .getElementById("torrents-panel")
    .classList.toggle("hidden", name !== "torrents");
  document
    .getElementById("downloads-panel")
    .classList.toggle("hidden", name !== "downloads");
}

// Login
async function handleLogin(e) {
  e.preventDefault();
  const apiKey = document.getElementById("api-key-input").value.trim();
  if (!apiKey) return;

  window.apiKey = apiKey;
  localStorage.setItem("apiKey", apiKey);

  try {
    showApp();
    await fetchAllData();
    startAutoRefresh();
  } catch (error) {
    localStorage.removeItem("apiKey");
    window.apiKey = null;
    showToast("Invalid API key", "error");
  }
}

function handleLogout() {
  localStorage.removeItem("apiKey");
  window.apiKey = null;
  stopAutoRefresh();
  document.getElementById("login-screen").classList.remove("hidden");
  document.getElementById("app-screen").classList.add("hidden");
}

// Show app
function showApp() {
  document.getElementById("login-screen").classList.add("hidden");
  document.getElementById("app-screen").classList.remove("hidden");
}

// Fetch all data
async function fetchAllData() {
  await fetchAuthInfo();
  await Promise.all([
    fetchStatus(),
    fetchTorrents(),
    fetchDownloads(),
    fetchKeptTorrents(),
    fetchAutoDeleteSetting(),
  ]);
}

// Auth
async function fetchAuthInfo() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/auth/me`);
    isAdmin = result.is_admin;

    // Show user greeting
    const greeting = document.getElementById("user-greeting");
    if (result.username) {
      greeting.textContent = `@${result.username}`;
    } else if (result.user_id) {
      greeting.textContent = `User #${result.user_id}`;
    }

    // Show auto-delete for admins
    if (isAdmin) {
      document.getElementById("autodelete-card").classList.remove("hidden");
    }
  } catch (error) {
    console.error("Auth error:", error);
  }
}

// Status
async function fetchStatus() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/status`);
    const badge = document.getElementById("status-badge");

    if (result.data && result.data.type === "premium") {
      badge.className = "badge badge-success";
      badge.innerHTML =
        '<i data-lucide="crown" class="w-3 h-3"></i><span>Premium</span>';
    } else {
      badge.className = "badge badge-muted";
      badge.innerHTML =
        '<i data-lucide="user" class="w-3 h-3"></i><span>Free</span>';
    }
    lucide.createIcons();
  } catch (error) {
    console.error("Status error:", error);
  }
}

// Torrents
async function fetchTorrents() {
  if (cachedTorrents.length === 0) showLoading("torrents", true);
  try {
    const result = await apiFetch(`${API_BASE_URL}/torrents?limit=100`);
    const newTorrents = result.data || [];

    // Check if we can smart-update
    if (cachedTorrents.length > 0) {
      const needsFullRender = smartUpdateTorrents(newTorrents);
      cachedTorrents = newTorrents;
      if (needsFullRender) renderTorrents();
    } else {
      cachedTorrents = newTorrents;
      renderTorrents();
    }
  } catch (error) {
    showToast("Failed to load torrents", "error");
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
    // Remove all items except empty
    Array.from(list.children).forEach((c) => {
      if (c.id !== "torrents-empty" && c.id !== "torrents-loading") {
        c.remove();
      }
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

      return `
      <div class="torrent-item ${isKept ? "kept" : ""}" data-id="${t.id}">
        <div class="flex items-start justify-between gap-4">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span class="truncate text-sm font-medium">${escapeHtml(t.filename)}</span>
              ${isKept ? '<span class="badge badge-info"><i data-lucide="shield" class="w-3 h-3"></i></span>' : ""}
            </div>
            <div class="flex items-center gap-3 text-xs text-[#71717a]">
              <span>${formatBytes(t.bytes)}</span>
              <span class="torrent-status ${statusClass}">${t.status}</span>
              <span class="torrent-seeders">${t.seeders ? `${t.seeders} seeds` : ""}</span>
            </div>
          </div>
          <div class="flex items-center gap-1">
            ${
              isAdmin
                ? `
              <button data-id="${t.id}" data-filename="${escapeHtml(t.filename)}" onclick="toggleKeep(this.dataset.id, this.dataset.filename)" class="btn btn-ghost p-2" title="${isKept ? "Unkeep" : "Keep"}">
                <i data-lucide="${isKept ? "shield-check" : "shield"}" class="w-4 h-4"></i>
              </button>
              <button data-id="${t.id}" data-filename="${escapeHtml(t.filename)}" onclick="confirmDelete('torrent', this.dataset.id, this.dataset.filename)" class="btn btn-ghost p-2 text-[#f87171]" title="Delete">
                <i data-lucide="trash-2" class="w-4 h-4"></i>
              </button>
            `
                : ""
            }
          </div>
        </div>
        <div class="mt-3">
          <div class="progress-bar">
            <div class="progress-fill ${t.progress >= 100 ? "complete" : ""}" style="width: ${t.progress}%"></div>
          </div>
          <div class="flex justify-between mt-1 text-xs text-[#71717a]">
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

  // Ignore granular update if search is active (it's simpler to do full re-render)
  const searchInput = document.getElementById("torrents-search");
  if (searchInput && searchInput.value) return true;

  for (let i = 0; i < newTorrents.length; i++) {
    const oldT = cachedTorrents[i];
    const newT = newTorrents[i];

    // Identity changed (e.g. order changed or item swapped)
    if (oldT.id !== newT.id) return true;

    // Visual update needed?
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
          statusEl.className = `torrent-status ${newT.status === "Downloaded" ? "badge-success" : newT.status === "Downloading" ? "badge-info" : newT.status === "Error" ? "badge-error" : "badge-muted"}`;
        }

        const seedersEl = card.querySelector(".torrent-seeders");
        if (seedersEl)
          seedersEl.textContent = newT.seeders ? `${newT.seeders} seeds` : "";

        const fillEl = card.querySelector(".progress-fill");
        if (fillEl) {
          fillEl.style.width = `${newT.progress}%`;
          if (newT.progress >= 100) fillEl.classList.add("complete");
          else fillEl.classList.remove("complete");
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

// Downloads
async function fetchDownloads() {
  if (cachedDownloads.length === 0) showLoading("downloads", true);
  try {
    const result = await apiFetch(`${API_BASE_URL}/downloads?limit=100`);
    const newDownloads = result.data || [];

    if (JSON.stringify(cachedDownloads) !== JSON.stringify(newDownloads)) {
      cachedDownloads = newDownloads;
      renderDownloads();
    }
  } catch (error) {
    showToast("Failed to load downloads", "error");
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
      if (c.id !== "downloads-empty" && c.id !== "downloads-loading") {
        c.remove();
      }
    });
    return;
  }

  empty.classList.add("hidden");

  const html = filtered
    .map(
      (d) => `
    <div class="torrent-item" data-id="${d.id}">
      <div class="flex items-start justify-between gap-4">
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2 mb-1">
            <a href="${d.download}" target="_blank" class="truncate text-sm font-medium hover:text-blue-400 transition-colors">${escapeHtml(d.filename)}</a>
            <span class="badge badge-muted">${d.host}</span>
          </div>
          <div class="flex items-center gap-3 text-xs text-[#71717a]">
            <span>${formatBytes(d.filesize)}</span>
            <span>${new Date(d.generated).toLocaleDateString()}</span>
          </div>
        </div>
        <div class="flex items-center gap-1">
          <a href="${d.download}" target="_blank" class="btn btn-ghost p-2" title="Download">
            <i data-lucide="download" class="w-4 h-4"></i>
          </a>
          ${
            isAdmin
              ? `
            <button data-id="${d.id}" data-filename="${escapeHtml(d.filename)}" onclick="confirmDelete('download', this.dataset.id, this.dataset.filename)" class="btn btn-ghost p-2 text-[#f87171]" title="Delete">
              <i data-lucide="trash-2" class="w-4 h-4"></i>
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

// Kept torrents
async function fetchKeptTorrents() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/kept-torrents`);
    keptTorrentIds.clear();
    (result.data || []).forEach((t) => keptTorrentIds.add(t.torrent_id));

    // Update UI
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
      btn.innerHTML = `<i data-lucide="${isKept ? "shield-check" : "shield"}" class="w-4 h-4"></i>`;
      btn.title = isKept ? "Unkeep" : "Keep";
    }

    // Add/remove badge
    const existingBadge = item.querySelector(".badge-info");
    if (isKept && !existingBadge) {
      const titleDiv = item.querySelector(".flex.items-center.gap-2.mb-1");
      if (titleDiv) {
        titleDiv.insertAdjacentHTML(
          "beforeend",
          '<span class="badge badge-info"><i data-lucide="shield" class="w-3 h-3"></i></span>',
        );
        lucide.createIcons();
      }
    } else if (!isKept && existingBadge) {
      existingBadge.remove();
    }
  });
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
    showToast(error.message, "error");
  }
}

// Auto-delete
async function fetchAutoDeleteSetting() {
  if (!isAdmin) return;

  try {
    const result = await apiFetch(`${API_BASE_URL}/settings/autodelete`);
    const value = result.data || "0";
    document.getElementById("autodelete-value").textContent =
      value === "0" ? "Disabled" : `${value} days`;
    document.getElementById("autodelete-input").value = value;
  } catch (error) {
    console.error("Failed to fetch auto-delete setting:", error);
  }
}

async function saveAutoDelete() {
  const value = document.getElementById("autodelete-input").value || "0";
  try {
    await apiFetch(`${API_BASE_URL}/settings/autodelete`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    });
    document.getElementById("autodelete-value").textContent =
      value === "0" ? "Disabled" : `${value} days`;
    showToast("Setting saved", "success");
  } catch (error) {
    showToast(error.message, "error");
  }
}

// Delete confirmation
let confirmCallback = null;

function confirmDelete(type, id, filename) {
  const modal = document.getElementById("confirm-modal");
  document.getElementById("confirm-title").textContent = `Delete ${type}?`;
  document.getElementById("confirm-message").textContent =
    `Are you sure you want to delete "${filename}"?`;

  confirmCallback = async () => {
    try {
      await apiFetch(
        `${API_BASE_URL}/${type === "torrent" ? "torrents" : "downloads"}/${id}`,
        { method: "DELETE" },
      );
      hideConfirmModal();
      showToast(
        `${type.charAt(0).toUpperCase() + type.slice(1)} deleted`,
        "success",
      );

      if (type === "torrent") {
        cachedTorrents = cachedTorrents.filter((t) => t.id !== id);
        renderTorrents();
      } else {
        cachedDownloads = cachedDownloads.filter((d) => d.id !== id);
        renderDownloads();
      }
    } catch (error) {
      showToast(error.message, "error");
    }
  };

  document.getElementById("confirm-ok").onclick = confirmCallback;
  modal.classList.add("show");
}

function hideConfirmModal() {
  document.getElementById("confirm-modal").classList.remove("show");
  confirmCallback = null;
}

// Auto-refresh
function startAutoRefresh() {
  toggleAutoRefresh(
    "torrents",
    document.getElementById("auto-refresh-torrents").checked,
  );
  toggleAutoRefresh(
    "downloads",
    document.getElementById("auto-refresh-downloads").checked,
  );
}

function toggleAutoRefresh(type, enabled) {
  if (refreshIntervals[type]) {
    clearInterval(refreshIntervals[type]);
    refreshIntervals[type] = null;
  }

  if (enabled) {
    refreshIntervals[type] = setInterval(() => {
      if (type === "torrents") fetchTorrents();
      else fetchDownloads();
    }, 10000);
  }
}

function stopAutoRefresh() {
  Object.keys(refreshIntervals).forEach((type) => {
    if (refreshIntervals[type]) {
      clearInterval(refreshIntervals[type]);
      refreshIntervals[type] = null;
    }
  });
}

// Toast
function showToast(message, type = "info") {
  const toast = document.getElementById("toast");
  toast.textContent = message;
  toast.className = `toast ${type}`;
  toast.classList.add("show");

  setTimeout(() => {
    toast.classList.remove("show");
  }, 3000);
}

// API fetch
async function apiFetch(url, options = {}) {
  const headers = {
    "Content-Type": "application/json",
    "X-API-Key": window.apiKey,
    ...options.headers,
  };

  const response = await fetch(url, { ...options, headers });

  if (response.status === 401) {
    handleLogout();
    throw new Error("Unauthorized");
  }

  if (!response.ok) {
    let errorMsg = "Request failed";
    try {
      const data = await response.json();
      errorMsg = data.message || data.error || errorMsg;
    } catch (e) {}
    throw new Error(errorMsg);
  }

  return response.json();
}

// Utilities
function escapeHtml(text) {
  if (!text) return "";
  return String(text)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}
