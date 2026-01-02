document.addEventListener("DOMContentLoaded", () => {
  checkLogin();
  setupEventListeners();
});

const API_BASE_URL = "/api";
let refreshIntervals = {};
let userRole = null; // 'admin' or 'viewer'
let isAdmin = false;

// Cache for filtering
let cachedTorrents = [];
let cachedDownloads = [];

// --- Auth & Init ---

function checkLogin() {
  // First check for token in URL
  const urlParams = new URLSearchParams(window.location.search);
  const token = urlParams.get("token");

  if (token) {
    // Token auth - store and use it
    window.authToken = token;
    window.authType = "token";
    // Clean URL without reloading
    window.history.replaceState({}, document.title, window.location.pathname);
    fetchAuthInfo().then(() => showDashboard());
    return;
  }

  // Fall back to API key
  const key = localStorage.getItem("rdctl_api_key");
  if (key) {
    window.apiKey = key;
    window.authType = "api_key";
    isAdmin = true; // API key always has admin access
    userRole = "admin";
    showDashboard();
  } else {
    showLogin();
  }
}

async function fetchAuthInfo() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/auth/me`);
    userRole = result.role;
    isAdmin = result.is_admin;

    // Store session expiry for countdown
    if (result.expires_at) {
      window.sessionExpiresAt = new Date(result.expires_at);
      startSessionCountdown();
    }

    console.log("Auth info:", {
      role: userRole,
      isAdmin,
      expiresAt: result.expires_at,
    });
  } catch (error) {
    console.error("Failed to fetch auth info:", error);
    logout();
  }
}

let sessionCountdownInterval = null;

function startSessionCountdown() {
  // Clear any existing countdown
  if (sessionCountdownInterval) {
    clearInterval(sessionCountdownInterval);
  }

  // Create or update session timer display
  updateSessionTimer();

  // Update every second
  sessionCountdownInterval = setInterval(() => {
    updateSessionTimer();
  }, 1000);
}

function updateSessionTimer() {
  const expiresAt = window.sessionExpiresAt;
  if (!expiresAt) return;

  const now = new Date();
  const diff = expiresAt - now;

  // Get or create timer element
  let timerEl = document.getElementById("session-timer");
  if (!timerEl) {
    const statusContainer = document.getElementById("status-container");
    if (statusContainer) {
      timerEl = document.createElement("span");
      timerEl.id = "session-timer";
      timerEl.className = "session-timer";
      statusContainer.appendChild(timerEl);
    }
  }

  if (!timerEl) return;

  if (diff <= 0) {
    timerEl.innerHTML = `<span class="timer-expired">⏰ Session expired</span>`;
    clearInterval(sessionCountdownInterval);
    setTimeout(() => {
      showToast(
        "Session expired. Please request a new dashboard link.",
        "error",
      );
      logout();
    }, 2000);
    return;
  }

  const minutes = Math.floor(diff / 60000);
  const seconds = Math.floor((diff % 60000) / 1000);

  const urgencyClass =
    minutes < 5 ? "timer-urgent" : minutes < 15 ? "timer-warning" : "";
  timerEl.innerHTML = `<span class="timer-icon">⏱️</span><span class="${urgencyClass}">${minutes}:${seconds.toString().padStart(2, "0")}</span>`;
}

function showLogin() {
  document.getElementById("login-overlay").classList.remove("hidden");
  document.querySelector(".app-container").classList.remove("active");
  document.querySelector(".app-container").classList.add("blur-content");
  document.getElementById("api-key-input").focus();
}

function showDashboard() {
  document.getElementById("login-overlay").classList.add("hidden");
  document.querySelector(".app-container").classList.remove("blur-content");
  document.querySelector(".app-container").classList.add("active");

  fetchStatus();
  fetchTorrents();
  fetchDownloads();

  // Setup auto-refresh
  toggleAutoRefresh(
    "torrents",
    document.getElementById("auto-refresh-torrents").checked,
  );
  toggleAutoRefresh(
    "downloads",
    document.getElementById("auto-refresh-downloads").checked,
  );
}

function handleLogin(e) {
  e.preventDefault();
  const key = document.getElementById("api-key-input").value.trim();
  if (key) {
    localStorage.setItem("rdctl_api_key", key);
    window.apiKey = key;
    window.authType = "api_key";
    isAdmin = true;
    userRole = "admin";
    showDashboard();
  }
}

function logout() {
  localStorage.removeItem("rdctl_api_key");
  window.apiKey = null;
  window.authToken = null;
  window.authType = null;
  window.sessionExpiresAt = null;
  userRole = null;
  isAdmin = false;

  // Clear all intervals
  clearInterval(refreshIntervals.torrents);
  clearInterval(refreshIntervals.downloads);
  if (sessionCountdownInterval) {
    clearInterval(sessionCountdownInterval);
    sessionCountdownInterval = null;
  }

  // Clear caches
  cachedTorrents = [];
  cachedDownloads = [];

  showLogin();
}

// --- Event Listeners ---

function setupEventListeners() {
  document.getElementById("login-form").addEventListener("submit", handleLogin);
  document.getElementById("logout-btn").addEventListener("click", logout);

  document
    .getElementById("add-torrent-form")
    .addEventListener("submit", addTorrent);
  document
    .getElementById("unrestrict-link-form")
    .addEventListener("submit", unrestrictLink);

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

  // Modal listeners
  document
    .getElementById("confirm-cancel")
    .addEventListener("click", closeConfirmModal);
}

function toggleAutoRefresh(type, enabled) {
  if (refreshIntervals[type]) {
    clearInterval(refreshIntervals[type]);
    refreshIntervals[type] = null;
  }

  if (enabled) {
    // Refresh every 5 seconds
    refreshIntervals[type] = setInterval(() => {
      if (type === "torrents") fetchTorrents();
      else fetchDownloads();
    }, 5000);
  }
}

// --- API Helper ---

async function apiFetch(url, options = {}) {
  const headers = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  // Add auth based on type
  if (window.authType === "token" && window.authToken) {
    headers["X-Auth-Token"] = window.authToken;
  } else if (window.apiKey) {
    headers["X-API-Key"] = window.apiKey;
  }

  options.headers = headers;

  try {
    const response = await fetch(url, options);
    if (response.status === 401) {
      logout();
      throw new Error("Unauthorized");
    }
    if (response.status === 403) {
      throw new Error("Forbidden: Admin access required for this operation");
    }
    if (!response.ok) {
      const errorData = await response.json();
      throw new Error(
        errorData.message || errorData.error || "An API error occurred",
      );
    }
    return response.json();
  } catch (error) {
    throw error;
  }
}

function showToast(message, type = "success") {
  const toast = document.getElementById("response-message");
  toast.textContent = message;
  toast.className = `toast ${type}`;
  toast.style.display = "block"; // Fallback
  toast.classList.remove("hidden");

  setTimeout(() => {
    toast.classList.add("hidden");
  }, 3000);
}

// --- Fetch Data ---

async function fetchStatus() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/status`);
    const user = result.data;
    const container = document.getElementById("status-container");

    const typeClass =
      user.type === "premium" ? "status-downloaded" : "status-error";
    const formattedDate = new Date(user.expiration).toLocaleDateString();
    const maskedUsername = maskUsername(user.username);

    container.innerHTML = `
            <span>${maskedUsername}</span>
            <span class="status-badge ${typeClass}">${user.type}</span>
            <span style="opacity: 0.7">|</span>
            <span>Exp: ${formattedDate}</span>
            <span>(${user.points} pts)</span>
        `;
    container.className = "status-pill";
  } catch (error) {
    console.error("Status error:", error);
  }
}

function maskUsername(username) {
  if (!username || username.length <= 5) {
    return "*****";
  }
  return "*****" + username.substring(5);
}

async function fetchTorrents(loadMore = false) {
  try {
    const offset = loadMore ? cachedTorrents.length : 0;
    const result = await apiFetch(
      `${API_BASE_URL}/torrents?limit=50&offset=${offset}`,
    );
    const newTorrents = result.data || [];
    const totalCount = result.total_count || newTorrents.length;

    // Update cache
    if (loadMore) {
      cachedTorrents = [...cachedTorrents, ...newTorrents];
    } else {
      cachedTorrents = newTorrents;
    }

    // Store total for pagination
    window.torrentsTotalCount = totalCount;

    renderTorrents();
  } catch (error) {
    showToast(`Error fetching torrents: ${error.message}`, "error");
  }
}

function renderTorrents(filterText = null) {
  const list = document.getElementById("torrents-list");
  const countBadge = document.getElementById("torrents-count");
  const searchInput = document.getElementById("torrents-search");
  const filter =
    filterText !== null
      ? filterText
      : searchInput
        ? searchInput.value.toLowerCase()
        : "";

  // Filter torrents
  const filteredTorrents = filter
    ? cachedTorrents.filter(
        (t) =>
          t.filename.toLowerCase().includes(filter) ||
          t.status.toLowerCase().includes(filter),
      )
    : cachedTorrents;

  const totalCount = window.torrentsTotalCount || cachedTorrents.length;

  // Update count badge
  if (cachedTorrents.length > 0) {
    const filterInfo = filter ? ` (${filteredTorrents.length} matches)` : "";
    if (totalCount > cachedTorrents.length) {
      countBadge.textContent = `(${cachedTorrents.length} of ${totalCount})${filterInfo}`;
    } else {
      countBadge.textContent = `(${cachedTorrents.length})${filterInfo}`;
    }
  } else {
    countBadge.textContent = "";
  }

  if (filteredTorrents.length === 0) {
    list.innerHTML = `<div class="item-card"><p style="text-align:center; color: var(--text-secondary)">${filter ? "No matching torrents" : "No active torrents"}</p></div>`;
    return;
  }

  const html = filteredTorrents
    .map((t) => {
      const statusClass =
        t.status === "Downloaded"
          ? "status-downloaded"
          : t.status === "Downloading"
            ? "status-downloading"
            : t.status === "Error" || t.status === "Dead"
              ? "status-error"
              : "status-badge";

      const progressClass =
        t.progress >= 100
          ? "progress-complete"
          : t.progress > 0
            ? "progress-active"
            : "";

      const addedDate = t.added ? new Date(t.added).toLocaleDateString() : "";

      return `
          <div class="item-card" data-filename="${escapeHtml(t.filename.toLowerCase())}">
              <div class="item-header">
                  <div style="flex:1">
                      <div class="item-title" title="${escapeHtml(t.filename)}">${escapeHtml(t.filename)}</div>
                      <div class="item-meta">
                          <span>${formatBytes(t.bytes)}</span>
                          <span class="status-badge ${statusClass}">${t.status}</span>
                          ${t.seeders !== undefined && t.seeders !== null ? `<span>${t.seeders} seeds</span>` : ""}
                          ${t.speed !== undefined && t.speed !== null && t.speed > 0 ? `<span>${formatBytes(t.speed)}/s</span>` : ""}
                          ${addedDate ? `<span title="Added date">${addedDate}</span>` : ""}
                      </div>
                  </div>
                  ${isAdmin ? `<button class="delete-btn" onclick="confirmDelete('torrent', '${t.id}', '${escapeHtml(t.filename)}')" title="Delete">🗑</button>` : ""}
              </div>
              <div class="progress-container ${progressClass}">
                  <div class="progress-fill" style="width: ${t.progress}%"></div>
                  <span class="progress-text">${t.progress.toFixed(1)}%</span>
              </div>
          </div>
          `;
    })
    .join("");

  // Add Load More button if there are more
  const loadMoreHtml =
    window.torrentsTotalCount > cachedTorrents.length && !filter
      ? `<button class="btn btn-secondary btn-block load-more-btn" onclick="fetchTorrents(true)">Load More (${cachedTorrents.length}/${window.torrentsTotalCount})</button>`
      : "";

  list.innerHTML = html + loadMoreHtml;
}

function filterTorrents() {
  const searchInput = document.getElementById("torrents-search");
  renderTorrents(searchInput.value.toLowerCase());
}

async function fetchDownloads(loadMore = false) {
  try {
    const offset = loadMore ? cachedDownloads.length : 0;
    const result = await apiFetch(
      `${API_BASE_URL}/downloads?limit=50&offset=${offset}`,
    );
    const newDownloads = result.data || [];
    const totalCount = result.total_count || newDownloads.length;

    // Update cache
    if (loadMore) {
      cachedDownloads = [...cachedDownloads, ...newDownloads];
    } else {
      cachedDownloads = newDownloads;
    }

    // Store total for pagination
    window.downloadsTotalCount = totalCount;

    renderDownloads();
  } catch (error) {
    showToast(`Error fetching downloads: ${error.message}`, "error");
  }
}

function renderDownloads(filterText = null) {
  const list = document.getElementById("downloads-list");
  const countBadge = document.getElementById("downloads-count");
  const searchInput = document.getElementById("downloads-search");
  const filter =
    filterText !== null
      ? filterText
      : searchInput
        ? searchInput.value.toLowerCase()
        : "";

  // Filter downloads
  const filteredDownloads = filter
    ? cachedDownloads.filter(
        (d) =>
          d.filename.toLowerCase().includes(filter) ||
          d.host.toLowerCase().includes(filter),
      )
    : cachedDownloads;

  const totalCount = window.downloadsTotalCount || cachedDownloads.length;

  // Update count badge
  if (cachedDownloads.length > 0) {
    const filterInfo = filter ? ` (${filteredDownloads.length} matches)` : "";
    if (totalCount > cachedDownloads.length) {
      countBadge.textContent = `(${cachedDownloads.length} of ${totalCount})${filterInfo}`;
    } else {
      countBadge.textContent = `(${cachedDownloads.length})${filterInfo}`;
    }
  } else {
    countBadge.textContent = "";
  }

  if (filteredDownloads.length === 0) {
    list.innerHTML = `<div class="item-card"><p style="text-align:center; color: var(--text-secondary)">${filter ? "No matching downloads" : "No recent downloads"}</p></div>`;
    return;
  }

  const html = filteredDownloads
    .map((d) => {
      return `
          <div class="item-card" data-filename="${escapeHtml(d.filename.toLowerCase())}">
              <div class="item-header">
                   <div style="flex:1">
                      <div class="item-title"><a href="${d.download}" target="_blank" style="color:inherit;text-decoration:none">${escapeHtml(d.filename)}</a></div>
                      <div class="item-meta">
                          <span>${formatBytes(d.filesize)}</span>
                          <span>${d.host}</span>
                          <span>${new Date(d.generated).toLocaleDateString()}</span>
                      </div>
                  </div>
                  ${isAdmin ? `<button class="delete-btn" onclick="confirmDelete('download', '${d.id}', '${escapeHtml(d.filename)}')" title="Delete">🗑</button>` : ""}
              </div>
          </div>
          `;
    })
    .join("");

  // Add Load More button if there are more
  const loadMoreHtml =
    window.downloadsTotalCount > cachedDownloads.length && !filter
      ? `<button class="btn btn-secondary btn-block load-more-btn" onclick="fetchDownloads(true)">Load More (${cachedDownloads.length}/${window.downloadsTotalCount})</button>`
      : "";

  list.innerHTML = html + loadMoreHtml;
}

function filterDownloads() {
  const searchInput = document.getElementById("downloads-search");
  renderDownloads(searchInput.value.toLowerCase());
}

function clearSearch(type) {
  const input = document.getElementById(`${type}-search`);
  if (input) {
    input.value = "";
    if (type === "torrents") renderTorrents("");
    else renderDownloads("");
  }
}

// --- Action Functions ---

async function addTorrent(e) {
  e.preventDefault();
  const input = document.getElementById("magnet-link");
  const magnet = input.value.trim();
  if (!magnet) return;

  try {
    const result = await apiFetch(`${API_BASE_URL}/torrents`, {
      method: "POST",
      body: JSON.stringify({ magnet }),
    });
    showToast("Torrent added successfully!", "success");
    input.value = "";
    fetchTorrents();
  } catch (error) {
    showToast(error.message, "error");
  }
}

async function unrestrictLink(e) {
  e.preventDefault();
  const input = document.getElementById("hoster-link");
  const link = input.value.trim();
  if (!link) return;

  try {
    const result = await apiFetch(`${API_BASE_URL}/unrestrict`, {
      method: "POST",
      body: JSON.stringify({ link }),
    });
    showToast("Link unrestricted!", "success");
    input.value = "";
    fetchDownloads();

    // Optional: Open link immediately
    // window.open(result.data.download, '_blank');
  } catch (error) {
    showToast(error.message, "error");
  }
}

// --- Delete Handling ---

let itemToDelete = null;

function confirmDelete(type, id, name) {
  itemToDelete = { type, id };
  const modal = document.getElementById("confirm-modal");
  document.getElementById("confirm-title").textContent =
    type === "torrent" ? "Delete Torrent?" : "Delete Download?";
  document.getElementById("confirm-message").textContent =
    `Are you sure you want to remove "${name}"?`;

  // Quick action handler setup
  const okBtn = document.getElementById("confirm-ok");
  okBtn.onclick = performDelete;

  modal.classList.remove("hidden");
  okBtn.focus();
}

function closeConfirmModal() {
  document.getElementById("confirm-modal").classList.add("hidden");
  itemToDelete = null;
}

async function performDelete() {
  if (!itemToDelete) return;

  const { type, id } = itemToDelete;
  const endpoint = type === "torrent" ? `/torrents/${id}` : `/downloads/${id}`;

  try {
    await apiFetch(`${API_BASE_URL}${endpoint}`, { method: "DELETE" });
    showToast(
      `${type === "torrent" ? "Torrent" : "Download"} deleted`,
      "success",
    );

    if (type === "torrent") fetchTorrents();
    else fetchDownloads();
  } catch (error) {
    showToast(error.message, "error");
  }

  closeConfirmModal();
}

// --- Utils ---

function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + " " + sizes[i];
}

function escapeHtml(text) {
  if (!text) return text;
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}
