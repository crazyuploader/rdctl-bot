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
  // 1. Check for exchange code in URL
  const urlParams = new URLSearchParams(window.location.search);
  const code = urlParams.get("code");

  if (code) {
    exchangeTokenID(code);
    return;
  }

  // 2. Check for token in sessionStorage
  const sessionToken = sessionStorage.getItem("rdctl_auth_token");
  if (sessionToken) {
    window.authToken = sessionToken;
    window.authType = "token";
    fetchAuthInfo().then(() => showDashboard());
    return;
  }

  // 3. Fall back to API key
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

async function exchangeTokenID(code) {
  try {
    // Clean URL immediately to hide code
    window.history.replaceState({}, document.title, window.location.pathname);

    const response = await fetch(`${API_BASE_URL}/exchange-token`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code }),
    });

    if (!response.ok) {
      throw new Error("Invalid or expired exchange code");
    }

    const result = await response.json();
    if (result.success && result.token) {
      window.authToken = result.token;
      window.authType = "token";
      sessionStorage.setItem("rdctl_auth_token", result.token);
      fetchAuthInfo().then(() => showDashboard());
    } else {
      throw new Error("Failed to exchange token");
    }
  } catch (error) {
    console.error("Exchange error:", error);
    showToast(error.message, "error");
    showLogin();
  }
}

async function fetchAuthInfo() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/auth/me`);
    userRole = result.role;
    isAdmin = result.is_admin;

    // Display greeting
    const greetingEl = document.getElementById("user-greeting");
    if (greetingEl) {
      if (result.first_name) {
        greetingEl.textContent = `Hi, ${result.first_name}!`;
      } else if (result.username) {
        greetingEl.textContent = `Hi, ${result.username}!`;
      } else {
        greetingEl.textContent = "";
      }
    }

    // Store session expiry for countdown
    if (result.expires_at) {
      window.sessionExpiresAt = new Date(result.expires_at);
      startSessionCountdown();
    }

    console.log("Auth info:", {
      role: userRole,
      isAdmin,
      first_name: result.first_name,
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
      timerEl = document.createElement("div");
      timerEl.id = "session-timer";
      timerEl.className =
        "flex items-center gap-2 px-3 py-1 rounded-full bg-blue-500/10 border border-blue-500/20 text-xs font-bold";
      statusContainer.appendChild(timerEl);
    }
  }

  if (!timerEl) return;

  if (diff <= 0) {
    timerEl.innerHTML = `<span class="text-red-400">⏰ Session expired</span>`;
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

  const colorClass =
    minutes < 5
      ? "text-red-400"
      : minutes < 15
        ? "text-yellow-400"
        : "text-blue-400";
  timerEl.innerHTML = `<span class="${colorClass}">⏱️ ${minutes}:${seconds.toString().padStart(2, "0")}</span>`;
}

function showLogin() {
  if (typeof window.originalShowLogin === "function") {
    window.originalShowLogin();
  } else {
    // Fallback
    const loginOverlay = document.getElementById("login-overlay");
    const appContainer =
      document.getElementById("app-container") ||
      document.querySelector(".app-container");

    loginOverlay.classList.remove("hidden");
    loginOverlay.style.opacity = "1";
    appContainer.classList.add("opacity-0", "pointer-events-none", "blur-sm");
    document.getElementById("api-key-input")?.focus();
  }
}

function showDashboard() {
  if (typeof window.originalShowDashboard === "function") {
    window.originalShowDashboard();
  } else {
    // Fallback
    const loginOverlay = document.getElementById("login-overlay");
    const appContainer =
      document.getElementById("app-container") ||
      document.querySelector(".app-container");

    loginOverlay.style.opacity = "0";
    setTimeout(() => {
      loginOverlay.classList.add("hidden");
    }, 300);
    appContainer.classList.remove(
      "opacity-0",
      "pointer-events-none",
      "blur-sm",
    );
  }

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
  sessionStorage.removeItem("rdctl_auth_token");
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
    headers["Authorization"] = `Bearer ${window.authToken}`;
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

  // Reset and set base classes
  toast.className = `fixed bottom-8 right-8 z-[3000] max-w-md px-6 py-4 rounded-xl glass-effect border-l-4 shadow-2xl text-white font-medium transition-all duration-300 flex items-center gap-3`;
  toast.innerHTML = ""; // Clear existing content

  // Create icon element
  const icon = document.createElement("span");
  icon.className = "text-xl";

  if (type === "error") {
    toast.classList.add("border-red-500");
    icon.classList.add("text-red-400");
    icon.textContent = "✕";
  } else {
    toast.classList.add("border-green-500");
    icon.classList.add("text-green-400");
    icon.textContent = "✓";
  }

  // Create message element (safe)
  const text = document.createElement("span");
  text.textContent = message;

  // Assembly
  toast.appendChild(icon);
  toast.appendChild(text);

  toast.classList.remove("hidden", "translate-y-20", "opacity-0");

  setTimeout(() => {
    toast.classList.add("translate-y-20", "opacity-0");
    setTimeout(() => {
      toast.classList.add("hidden");
    }, 300);
  }, 3000);
}

// --- Fetch Data ---

async function fetchStatus() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/status`);
    const user = result.data;
    const container = document.getElementById("status-container");

    const typeClass =
      user.type === "premium"
        ? "text-green-400 bg-green-500/10"
        : "text-red-400 bg-red-500/10";
    const formattedDate = new Date(user.expiration).toLocaleDateString();
    const maskedUsername = maskUsername(user.username);

    container.innerHTML = `
      <span class="font-bold text-white">${maskedUsername}</span>
      <span class="px-2 py-0.5 rounded-md text-xs font-bold uppercase ${typeClass}">${user.type}</span>
      <span class="text-slate-600">|</span>
      <span class="text-slate-400">Exp: <span class="text-slate-200">${formattedDate}</span></span>
      <span class="text-slate-400">(${user.points} pts)</span>
    `;
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
    countBadge.textContent = `${cachedTorrents.length}${totalCount > cachedTorrents.length ? ` of ${totalCount}` : ""}${filterInfo} items`;
  } else {
    countBadge.textContent = "0 items";
  }

  if (filteredTorrents.length === 0) {
    list.innerHTML = `<div class="flex flex-col items-center justify-center h-full text-slate-500">
      <svg class="w-16 h-16 mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/>
      </svg>
      <p class="text-sm">${filter ? "No matching torrents" : "No active torrents"}</p>
    </div>`;
    return;
  }

  const html = filteredTorrents
    .map((t) => {
      const statusClass =
        t.status === "Downloaded"
          ? "text-green-400 bg-green-500/10"
          : t.status === "Downloading"
            ? "text-blue-400 bg-blue-500/10"
            : t.status === "Error" || t.status === "Dead"
              ? "text-red-400 bg-red-500/10"
              : "text-slate-400 bg-slate-800/50";

      const progressColor =
        t.progress >= 100
          ? "bg-green-500"
          : t.progress > 0
            ? "bg-blue-500"
            : "bg-slate-700";

      const addedDate = t.added ? new Date(t.added).toLocaleDateString() : "";

      return `
        <div class="group relative glass-effect border border-slate-700/50 rounded-xl p-4 hover:border-blue-500/40 transition-all duration-200">
          <div class="flex justify-between items-start gap-4 mb-3">
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-white break-all mb-1" title="${escapeHtml(t.filename)}">${escapeHtml(t.filename)}</div>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-400">
                <span class="font-medium text-slate-300">${formatBytes(t.bytes)}</span>
                <span class="px-2 py-0.5 rounded-md text-xs font-bold uppercase ${statusClass}">${t.status}</span>
                ${t.seeders !== undefined && t.seeders !== null ? `<span class="flex items-center gap-1"><span class="w-1.5 h-1.5 rounded-full bg-green-500"></span>${t.seeders} seeds</span>` : ""}
                ${t.speed !== undefined && t.speed !== null && t.speed > 0 ? `<span>${formatBytes(t.speed)}/s</span>` : ""}
                ${addedDate ? `<span>${addedDate}</span>` : ""}
              </div>
            </div>
            ${
              isAdmin
                ? `<button class="p-2 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-all" onclick="confirmDelete('torrent', '${t.id}', '${escapeHtml(t.filename)}')" title="Delete">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
            </button>`
                : ""
            }
          </div>
          <div class="relative h-2 w-full bg-slate-800 rounded-full overflow-hidden">
            <div class="h-full ${progressColor} transition-all duration-500 ${t.status === "Downloading" ? "animate-pulse" : ""}" style="width: ${t.progress}%"></div>
          </div>
          <div class="mt-1 text-right text-xs font-medium text-slate-500">${t.progress.toFixed(1)}%</div>
        </div>
      `;
    })
    .join("");

  // Add Load More button if there are more
  const loadMoreHtml =
    window.torrentsTotalCount > cachedTorrents.length && !filter
      ? `<button class="w-full py-3 rounded-xl border border-dashed border-slate-700 text-slate-400 text-sm font-medium hover:border-blue-500 hover:text-blue-400 transition-all" onclick="fetchTorrents(true)">Load More (${cachedTorrents.length}/${window.torrentsTotalCount})</button>`
      : "";

  list.innerHTML = html + loadMoreHtml;
}

function filterTorrents() {
  const searchInput = document.getElementById("torrents-search");
  const clearBtn = document.getElementById("torrents-clear-btn");
  if (clearBtn) {
    if (searchInput.value) {
      clearBtn.classList.remove("opacity-0", "pointer-events-none");
    } else {
      clearBtn.classList.add("opacity-0", "pointer-events-none");
    }
  }
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
    countBadge.textContent = `${cachedDownloads.length}${totalCount > cachedDownloads.length ? ` of ${totalCount}` : ""}${filterInfo} items`;
  } else {
    countBadge.textContent = "0 items";
  }

  if (filteredDownloads.length === 0) {
    list.innerHTML = `<div class="flex flex-col items-center justify-center h-full text-slate-500">
      <svg class="w-16 h-16 mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
      </svg>
      <p class="text-sm">${filter ? "No matching downloads" : "No recent downloads"}</p>
    </div>`;
    return;
  }

  const html = filteredDownloads
    .map((d) => {
      const safeUrl = sanitizeUrl(d.download);
      return `
        <div class="group relative glass-effect border border-slate-700/50 rounded-xl p-4 hover:border-purple-500/40 transition-all duration-200">
          <div class="flex justify-between items-start gap-4">
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold text-white break-all mb-1">
                <a href="${safeUrl}" target="_blank" rel="noopener noreferrer" class="hover:text-purple-400 transition-colors">${escapeHtml(d.filename)}</a>
              </div>
              <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-400">
                <span class="font-medium text-slate-300">${formatBytes(d.filesize)}</span>
                <span class="px-2 py-0.5 rounded-md text-xs font-bold uppercase bg-slate-800/50">${d.host}</span>
                <span>${new Date(d.generated).toLocaleDateString()}</span>
              </div>
            </div>
            ${
              isAdmin
                ? `<button class="p-2 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-all" onclick="confirmDelete('download', '${d.id}', '${escapeHtml(d.filename)}')" title="Delete">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
            </button>`
                : ""
            }
          </div>
        </div>
      `;
    })
    .join("");

  // Add Load More button if there are more
  const loadMoreHtml =
    window.downloadsTotalCount > cachedDownloads.length && !filter
      ? `<button class="w-full py-3 rounded-xl border border-dashed border-slate-700 text-slate-400 text-sm font-medium hover:border-purple-500 hover:text-purple-400 transition-all" onclick="fetchDownloads(true)">Load More (${cachedDownloads.length}/${window.downloadsTotalCount})</button>`
      : "";

  list.innerHTML = html + loadMoreHtml;
}

function filterDownloads() {
  const searchInput = document.getElementById("downloads-search");
  const clearBtn = document.getElementById("downloads-clear-btn");
  if (clearBtn) {
    if (searchInput.value) {
      clearBtn.classList.remove("opacity-0", "pointer-events-none");
    } else {
      clearBtn.classList.add("opacity-0", "pointer-events-none");
    }
  }
  renderDownloads(searchInput.value.toLowerCase());
}

function clearSearch(type) {
  const input = document.getElementById(`${type}-search`);
  const clearBtn = document.getElementById(`${type}-clear-btn`);
  if (input) {
    input.value = "";
    if (clearBtn) {
      clearBtn.classList.add("opacity-0", "pointer-events-none");
    }
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

  modal.classList.remove("hidden", "opacity-0", "pointer-events-none");
  setTimeout(() => {
    modal.querySelector(".glass-effect")?.classList.remove("scale-95");
  }, 10);
  okBtn.focus();
}

function closeConfirmModal() {
  const modal = document.getElementById("confirm-modal");
  modal.classList.add("opacity-0", "pointer-events-none");
  modal.querySelector(".glass-effect")?.classList.add("scale-95");
  setTimeout(() => {
    modal.classList.add("hidden");
  }, 300);
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

function sanitizeUrl(urlString) {
  if (!urlString) return "#";
  try {
    const url = new URL(urlString, window.location.origin);
    if (url.protocol === "http:" || url.protocol === "https:") {
      return urlString;
    }
    return "#";
  } catch (e) {
    return "#";
  }
}
