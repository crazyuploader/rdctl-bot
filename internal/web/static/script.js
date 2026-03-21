document.addEventListener("DOMContentLoaded", () => {
  checkLogin();
  setupEventListeners();
  setupTabs();
  // Fetch kept torrents initially and set up interval to update
  fetchKeptTorrents();
  keptTorrentsInterval = setInterval(fetchKeptTorrents, 30000); // Update every 30 seconds

  // Fetch auto-delete setting initially and set up interval to update
  fetchAutoDeleteSetting();
  autoDeleteInterval = setInterval(fetchAutoDeleteSetting, 30000); // Update every 30 seconds

  // Add event listener for auto-delete save button
  const saveBtn = document.getElementById("autodelete-save");
  if (saveBtn) {
    saveBtn.addEventListener("click", saveAutoDeleteSetting);
  }

  // Add event listener for auto-delete input (save on Enter key)
  const inputEl = document.getElementById("autodelete-input");
  if (inputEl) {
    inputEl.addEventListener("keypress", (e) => {
      if (e.key === "Enter") {
        saveAutoDeleteSetting();
      }
    });
  }
});

const API_BASE_URL = "/api";
let refreshIntervals = {};
let userRole = null; // 'admin' or 'viewer'
let isAdmin = false;
let keptTorrentsInterval = null; // Interval for updating kept torrents
let autoDeleteInterval = null; // Interval for updating auto-delete setting

// Cache for filtering
let cachedTorrents = [];
let cachedDownloads = [];
let activeTab = "all";
let keptTorrentIds = new Set(); // Set of kept torrent IDs

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
      let errorMsg = "Invalid or expired exchange code";
      try {
        const errorData = await response.json();
        errorMsg =
          errorData.message || errorData.error || errorData.message || errorMsg;
      } catch (e) {
        // Fallback to default msg if not JSON
      }
      throw new Error(errorMsg);
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
  timerEl.innerHTML = `<span class="${colorClass}">⏱️ ${minutes}:${seconds
    .toString()
    .padStart(2, "0")}</span>`;
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

  // Search Listener
  const searchInput = document.getElementById("torrents-search");
  if (searchInput) {
    searchInput.addEventListener("input", (e) => {
      renderTorrents(e.target.value.toLowerCase());
    });
  }

  const downloadsSearchInput = document.getElementById("downloads-search");
  if (downloadsSearchInput) {
    downloadsSearchInput.addEventListener("input", (e) => {
      renderDownloads(e.target.value.toLowerCase());
    });
  }

  // Modal listeners
  document
    .getElementById("confirm-cancel")
    .addEventListener("click", closeConfirmModal);
}

function setupTabs() {
  const tabs = document.querySelectorAll("#torrents-tabs button[data-tab]");
  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      // Update UI
      tabs.forEach(
        (t) =>
          (t.className =
            "px-3 py-1.5 rounded-lg bg-slate-800/50 text-slate-400 text-xs font-medium whitespace-nowrap hover:bg-slate-700/50 transition-colors"),
      );
      tab.className =
        "px-3 py-1.5 rounded-lg bg-blue-500/20 text-blue-400 text-xs font-semibold whitespace-nowrap active-tab";

      // Update State
      activeTab = tab.getAttribute("data-tab");
      renderTorrents();
    });
  });
}

function toggleAutoRefresh(type, enabled) {
  if (refreshIntervals[type]) {
    clearInterval(refreshIntervals[type]);
    refreshIntervals[type] = null;
  }

  if (enabled) {
    // Refresh every 10 seconds for better UX (avoids flicker on hover)
    refreshIntervals[type] = setInterval(() => {
      if (type === "torrents") fetchTorrents();
      else fetchDownloads();
    }, 10000); // 10000ms = 10 seconds
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
      let errorMsg = "An API error occurred";
      try {
        const errorData = await response.json();
        errorMsg = errorData.message || errorData.error || errorMsg;
      } catch (e) {
        // Handle non-JSON error responses (like 404 from proxy or server crashing)
        errorMsg = `Error ${response.status}: ${response.statusText}`;
      }
      throw new Error(errorMsg);
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

async function fetchKeptTorrents() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/kept-torrents`);
    const keptTorrents = result.data || [];

    // Update the kept torrent IDs set
    keptTorrentIds.clear();
    keptTorrents.forEach((t) => {
      keptTorrentIds.add(t.torrent_id);
    });

    // Update keep status without full re-render to avoid blinking
    updateKeepStatusOnCards();
  } catch (error) {
    console.error("Error fetching kept torrents:", error);
    // Don't show toast for this as it's a background update
  }
}

function updateKeepStatusOnCards() {
  // Only update the keep badge and icon on existing cards, don't rebuild the list
  keptTorrentIds.forEach((torrentId) => {
    const card = document.querySelector(`[data-torrent-id="${torrentId}"]`);
    if (card) {
      // Add kept badge if not present
      if (!card.querySelector('.kept-badge')) {
        const badge = document.createElement('div');
        badge.className = 'kept-badge absolute -top-2 -right-2 px-2 py-0.5 rounded-full bg-blue-500 text-white text-xs font-bold shadow-lg';
        badge.textContent = 'KEPT';
        card.style.position = 'relative';
        card.appendChild(badge);
      }
      // Update keep button icon to "kept" state
      const keepBtn = card.querySelector('button[title*="Keep"], button[title*="kept"]');
      if (keepBtn) {
        keepBtn.className = keepBtn.className.replace(/text-slate-500/g, 'text-blue-400 bg-blue-500/10');
        keepBtn.innerHTML = '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>';
        keepBtn.title = "Remove from kept";
      }
    }
  });
  
  // Also add ring to kept cards
  document.querySelectorAll('#torrents-list > div').forEach(card => {
    const torrentId = card.dataset.torrentId;
    if (torrentId && keptTorrentIds.has(torrentId)) {
      card.classList.add('ring-2', 'ring-blue-500/30');
    }
  });
}

async function fetchAutoDeleteSetting() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/settings/autodelete`);
    const value = result.data || "0";

    // Update UI
    const valueEl = document.getElementById("autodelete-value");
    const inputEl = document.getElementById("autodelete-input");
    const settingsEl = document.getElementById("autodelete-settings");

    if (valueEl) valueEl.textContent = value;
    if (inputEl) inputEl.value = value;

    // Show settings panel if user is admin
    if (settingsEl && isAdmin) {
      settingsEl.classList.remove("hidden");
    }
  } catch (error) {
    console.error("Error fetching auto-delete setting:", error);
    // Don't show toast for this as it's a background update
  }
}

async function fetchStatus() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/status`);
    const user = result.data;
    const container = document.getElementById("status-container");

    if (!container) return;

    const typeClass =
      user.type === "premium"
        ? "text-green-400 bg-green-500/10"
        : "text-red-400 bg-red-500/10";
    const formattedDate = new Date(user.expiration).toLocaleDateString();
    const maskedUsername = maskUsername(user.username);

    // Update Ring (safely)
    try {
      const expDate = new Date(user.expiration);
      const now = new Date();
      const diffTime = Math.abs(expDate - now);
      const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
      updatePremiumRing(diffDays);
    } catch (e) {
      console.error("Ring update failed:", e);
    }

    container.innerHTML = `
      <span class="font-bold text-white">${escapeHtml(maskedUsername)}</span>
      <span class="px-2 py-0.5 rounded-md text-xs font-bold uppercase ${typeClass}">${escapeHtml(
        user.type,
      )}</span>
      <span class="text-slate-600">|</span>
      <span class="text-slate-400">Exp: <span class="text-slate-200">${formattedDate}</span></span>
      <span class="text-slate-400">(${user.points} pts)</span>
    `;

    // Re-trigger timer update if it was wiped
    if (window.sessionExpiresAt) updateSessionTimer();
  } catch (error) {
    console.error("Status error:", error);
    const container = document.getElementById("status-container");
    if (container) {
      container.innerHTML = `<span class="text-red-400 text-xs">Failed to load status</span>`;
    }
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
      renderTorrents(); // Full re-render when loading more
    } else {
      // Smart update: only re-render if data actually changed
      const hasChanges = smartUpdateTorrents(newTorrents);
      
      if (hasChanges) {
        cachedTorrents = newTorrents;
        renderTorrents();
      }
    }

    // Store total for pagination
    window.torrentsTotalCount = totalCount;
  } catch (error) {
    showToast(`Error fetching torrents: ${error.message}`, "error");
  }
}

function smartUpdateTorrents(newTorrents) {
  // Check if we need to update (data changed)
  if (cachedTorrents.length !== newTorrents.length) {
    return true; // Item added/removed, need full re-render
  }
  
  // Compare each torrent for changes
  for (let i = 0; i < newTorrents.length; i++) {
    const oldT = cachedTorrents[i];
    const newT = newTorrents[i];
    
    if (oldT.id !== newT.id || 
        oldT.status !== newT.status || 
        oldT.progress !== newT.progress ||
        oldT.speed !== newT.speed ||
        oldT.seeders !== newT.seeders) {
      // Found a difference, update just that card
      updateTorrentCard(newT);
    }
  }
  
  return false; // No structural changes
}

function updateTorrentCard(torrent) {
  const card = document.querySelector(`[data-torrent-id="${torrent.id}"]`);
  if (!card) return;
  
  // Update progress bar
  const progressBar = card.querySelector('.progress-bar');
  const progressText = card.querySelector('.progress-text');
  
  if (progressBar) {
    const progressPercent = Math.round(torrent.progress);
    progressBar.style.width = `${progressPercent}%`;
    
    // Update progress color
    progressBar.className = progressBar.className.replace(/bg-\w+-\d+/g, '');
    if (torrent.progress >= 100) {
      progressBar.classList.add('bg-green-500');
    } else if (torrent.progress > 0) {
      progressBar.classList.add('bg-blue-500');
    } else {
      progressBar.classList.add('bg-slate-700');
    }
    
    // Update animation for downloading
    if (torrent.status === "Downloading") {
      progressBar.classList.add('animate-pulse');
    } else {
      progressBar.classList.remove('animate-pulse');
    }
  }
  
  if (progressText) {
    progressText.textContent = `${torrent.progress.toFixed(1)}%`;
    progressText.className = progressText.className.replace(/text-\w+-\d+/g, '');
    if (torrent.progress >= 100) {
      progressText.classList.add('text-green-400');
    } else {
      progressText.classList.add('text-blue-400');
    }
  }
  
  // Update status badge
  const statusBadge = card.querySelector('.status-badge');
  if (statusBadge) {
    const statusClass = 
      torrent.status === "Downloaded" ? "text-green-400 bg-green-500/10" :
      torrent.status === "Downloading" ? "text-blue-400 bg-blue-500/10" :
      torrent.status === "Error" || torrent.status === "Dead" ? "text-red-400 bg-red-500/10" :
      "text-slate-400 bg-slate-800/50";
    
    statusBadge.className = statusBadge.className.replace(/text-\w+-\d+|bg-\w+-\d+\/\d+/g, '');
    statusBadge.classList.add(...statusClass.split(' '));
    
    // Update status text
    statusBadge.innerHTML = statusBadge.innerHTML.replace(/>([\w\s]+)</, `>${torrent.status}<`);
  }
  
  // Update speed
  const speedSpan = card.querySelector('.speed-info');
  if (speedSpan && torrent.speed !== undefined && torrent.speed > 0) {
    speedSpan.innerHTML = `<svg class="w-3.5 h-3.5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"/>
    </svg>
    ${formatBytes(torrent.speed)}/s`;
  }
}

// Batch Selection State
let selectedTorrents = new Set();

function toggleSelection(id) {
  if (selectedTorrents.has(id)) {
    selectedTorrents.delete(id);
  } else {
    selectedTorrents.add(id);
  }
  updateBatchDeleteButton();
  renderTorrents(null, true); // Re-render to update checkbox states without full re-filter
}

function toggleSelectAll(checked) {
  const searchInput = document.getElementById("torrents-search");
  const filter = searchInput ? searchInput.value.toLowerCase() : "";

  // Duplicate filtering logic to know what to select
  let visibleTorrents = cachedTorrents;

  if (filter) {
    visibleTorrents = visibleTorrents.filter(
      (t) =>
        t.filename.toLowerCase().includes(filter) ||
        t.status.toLowerCase().includes(filter),
    );
  }

  if (activeTab === "downloading") {
    visibleTorrents = visibleTorrents.filter(
      (t) => t.status.toLowerCase() === "downloading",
    );
  } else if (activeTab === "completed") {
    visibleTorrents = visibleTorrents.filter(
      (t) => t.status.toLowerCase() === "downloaded",
    );
  } else if (activeTab === "error") {
    visibleTorrents = visibleTorrents.filter((t) => {
      const s = t.status.toLowerCase();
      return s === "error" || s === "dead";
    });
  }

  if (checked) {
    // Select only visible
    visibleTorrents.forEach((t) => selectedTorrents.add(t.id));
  } else {
    // Deselect visible (keeping others if any? Standard behavior is complicated,
    // but usually "Select All" checked -> select visible.
    // Unchecked -> Deselect visible or Clear All?
    // Given the UI is "Select All", unchecking usually means "Clear Selection" or "Deselect These".
    // For simplicity and safety in batch actions, let's clear ONLY the visible ones from the set,
    // or just clear all?
    // The previous logic was `selectedTorrents.clear()`.
    // If I have items selected in another tab, should I clear them?
    // Probably yes, to avoid accidental deletions of meaningful invisible items.
    // BUT, if I am refining "Select All" to be "Select Visible", uncheck should probably "Deselect Visible".
    // Let's go with Deselect Visible to be safe and consistent.

    // However, the user issue was "it goes back to all category".
    // That sounds like `renderTorrents` without arguments logic? No, `renderTorrents(null, true)` is used.

    // Let's stick to "Uncheck select all -> Deselect visible items".
    visibleTorrents.forEach((t) => selectedTorrents.delete(t.id));
  }
  updateBatchDeleteButton();
  renderTorrents(null, true);
}

function updateBatchDeleteButton() {
  const btn = document.getElementById("torrents-batch-delete-btn");
  const selectAllBtn = document.getElementById("select-all-btn");
  const selectAllChecked = document.getElementById("select-all-checked");
  const selectAllUnchecked = document.getElementById("select-all-unchecked");

  // Update button visibility
  if (selectedTorrents.size > 0) {
    btn.classList.remove("hidden");
    btn.innerHTML = `
            <svg class="w-5 h-5 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
            <span class="text-xs font-bold">${selectedTorrents.size}</span>
        `;
  } else {
    btn.classList.add("hidden");
  }

  // Update Select All Button State
  if (selectAllBtn && selectAllChecked && selectAllUnchecked) {
    // --- DUPLICATE FILTERING LOGIC ---
    // Ideally this should be a helper function `getVisibleTorrents()` but keeping it inline for minimal diffs
    const searchInput = document.getElementById("torrents-search");
    const filter = searchInput ? searchInput.value.toLowerCase() : "";
    let visibleTorrents = cachedTorrents;
    if (filter) {
      visibleTorrents = visibleTorrents.filter(
        (t) =>
          t.filename.toLowerCase().includes(filter) ||
          t.status.toLowerCase().includes(filter),
      );
    }
    if (activeTab === "downloading") {
      visibleTorrents = visibleTorrents.filter(
        (t) => t.status.toLowerCase() === "downloading",
      );
    } else if (activeTab === "completed") {
      visibleTorrents = visibleTorrents.filter(
        (t) => t.status.toLowerCase() === "downloaded",
      );
    } else if (activeTab === "error") {
      visibleTorrents = visibleTorrents.filter((t) => {
        const s = t.status.toLowerCase();
        return s === "error" || s === "dead";
      });
    }
    // --------------------------------

    const allSelected =
      visibleTorrents.length > 0 &&
      visibleTorrents.every((t) => selectedTorrents.has(t.id));
    const someSelected = visibleTorrents.some((t) =>
      selectedTorrents.has(t.id),
    );

    if (allSelected) {
      // Full Checked State
      selectAllBtn.classList.add("text-blue-500");
      selectAllBtn.classList.remove("text-slate-400");

      selectAllChecked.classList.remove("opacity-0", "scale-50");
      selectAllChecked.classList.add("opacity-100", "scale-100");

      selectAllUnchecked.classList.add("opacity-0", "scale-50");
      selectAllUnchecked.classList.remove("opacity-100");
    } else if (someSelected) {
      // Indeterminate State (Simulated)
      selectAllBtn.classList.remove("text-blue-500");
      selectAllBtn.classList.add("text-slate-400");

      // For indeterminate, let's just show unchecked but maybe with a different visual if we had one?
      // Since we don't have a dash icon, standard "unchecked" is fine for "some selected"
      // because "Select All" behavior usually is "Click to Select All" if mixed.

      selectAllChecked.classList.add("opacity-0", "scale-50");
      selectAllChecked.classList.remove("opacity-100", "scale-100");

      selectAllUnchecked.classList.remove("opacity-0", "scale-50");
      selectAllUnchecked.classList.add("opacity-100");
    } else {
      // None Selected
      selectAllBtn.classList.remove("text-blue-500");
      selectAllBtn.classList.add("text-slate-400");

      selectAllChecked.classList.add("opacity-0", "scale-50");
      selectAllChecked.classList.remove("opacity-100", "scale-100");

      selectAllUnchecked.classList.remove("opacity-0", "scale-50");
      selectAllUnchecked.classList.add("opacity-100");
    }
  }
}

async function deleteSelectedTorrents() {
  if (selectedTorrents.size === 0) return;

  if (
    !confirm(
      `Are you sure you want to delete ${selectedTorrents.size} torrents?`,
    )
  )
    return;

  let successCount = 0;
  const errors = [];

  // Clone set to avoid modification issues during iteration if we were removing properly
  const idsToDelete = Array.from(selectedTorrents);

  for (const id of idsToDelete) {
    try {
      await apiFetch(`${API_BASE_URL}/torrents/${id}`, { method: "DELETE" });
      successCount++;
    } catch (e) {
      errors.push(id);
      console.error(`Failed to delete ${id}:`, e);
    }
  }

  if (successCount > 0) {
    showToast(`Deleted ${successCount} torrents successfully`, "success");
    selectedTorrents.clear();
    updateBatchDeleteButton();
    fetchTorrents();
  }

  if (errors.length > 0) {
    showToast(`Failed to delete ${errors.length} torrents`, "error");
  }
}

// Toggle keep/unkeep status for a torrent
async function toggleKeep(torrentId, filename) {
  try {
    if (keptTorrentIds.has(torrentId)) {
      // Currently kept, so unkeep it
      await apiFetch(`${API_BASE_URL}/torrents/${torrentId}/keep`, {
        method: "DELETE",
      });
      keptTorrentIds.delete(torrentId);
      showToast(`Torrent "${filename}" is no longer kept`, "success");
    } else {
      // Not kept, so keep it
      await apiFetch(`${API_BASE_URL}/torrents/${torrentId}/keep`, {
        method: "POST",
      });
      keptTorrentIds.add(torrentId);
      showToast(`Torrent "${filename}" is now kept`, "success");
    }

    // Re-render to update the button
    renderTorrents();
  } catch (error) {
    showToast(error.message, "error");
  }
}

// Save auto-delete setting
async function saveAutoDeleteSetting() {
  const inputEl = document.getElementById("autodelete-input");
  if (!inputEl) return;

  const value = inputEl.value.trim();
  if (value === "") {
    showToast("Please enter a valid number of days", "error");
    return;
  }

  const days = parseInt(value);
  if (isNaN(days) || days < 0) {
    showToast("Please enter a valid number of days (0 or greater)", "error");
    return;
  }

  try {
    await apiFetch(`${API_BASE_URL}/settings/autodelete`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ value }),
    });
    showToast("Auto-delete setting saved successfully", "success");

    // Update the display
    const valueEl = document.getElementById("autodelete-value");
    if (valueEl) valueEl.textContent = value;
  } catch (error) {
    showToast(error.message, "error");
  }
}

function renderTorrents(filterText = null, preserveSelection = false) {
  const list = document.getElementById("torrents-list");
  const countBadge = document.getElementById("torrents-count");
  const searchInput = document.getElementById("torrents-search");

  // If we are just re-rendering to update checkboxes/selection UI, don't re-read search input if passed null
  // But typically renderTorrents is called with null or a value.
  // If preserveSelection is true, we assume filter hasn't ostensibly changed, but we should still respect current filter.

  const filter =
    filterText !== null
      ? filterText
      : searchInput
        ? searchInput.value.toLowerCase()
        : "";

  // Filter torrents
  let filteredTorrents = filter
    ? cachedTorrents.filter(
        (t) =>
          t.filename.toLowerCase().includes(filter) ||
          t.status.toLowerCase().includes(filter),
      )
    : cachedTorrents;

  // Apply Tab Filter
  if (activeTab === "downloading") {
    filteredTorrents = filteredTorrents.filter(
      (t) => t.status.toLowerCase() === "downloading",
    );
  } else if (activeTab === "completed") {
    filteredTorrents = filteredTorrents.filter(
      (t) => t.status.toLowerCase() === "downloaded",
    );
  } else if (activeTab === "error") {
    filteredTorrents = filteredTorrents.filter((t) => {
      const s = t.status.toLowerCase();
      return s === "error" || s === "dead";
    });
  }

  const totalCount = window.torrentsTotalCount || cachedTorrents.length;

  // Update count badge
  if (cachedTorrents.length > 0) {
    const filterInfo = filter ? ` (${filteredTorrents.length} matches)` : "";
    countBadge.textContent = `${cachedTorrents.length}${
      totalCount > cachedTorrents.length ? ` of ${totalCount}` : ""
    }${filterInfo} items`;
  } else {
    countBadge.textContent = "0 items";
  }

  // Update Select All Checkbox State based on filtered view or global cache?
  // Typically select all applies to visible. For now sticking to simple global cache logic or filtered logic.
  // Let's rely on updateBatchDeleteButton which re-checks state.
  updateBatchDeleteButton();

  if (filteredTorrents.length === 0) {
    list.innerHTML = `<div class="flex flex-col items-center justify-center h-full text-slate-500">
      <svg class="w-16 h-16 mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/>
      </svg>
      <p class="text-sm">${
        filter ? "No matching torrents" : "No active torrents"
      }</p>
    </div>`;
    return;
  }

  const html = filteredTorrents
    .map((t) => {
      const isSelected = selectedTorrents.has(t.id);
      const isKept = keptTorrentIds.has(t.id);
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
      
      const statusIcon = t.status === "Downloaded"
        ? '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>'
        : t.status === "Downloading"
          ? '<svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>'
          : t.status === "Error" || t.status === "Dead"
            ? '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/></svg>'
            : '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>';

      return `
         <div data-torrent-id="${t.id}" class="group relative glass-effect border ${
           isSelected ? "border-blue-500 bg-blue-500/5" : "border-slate-700/50"
         } ${isKept ? "ring-2 ring-blue-500/30" : ""} rounded-xl p-4 hover:border-blue-500/40 transition-all duration-200">
           ${isKept ? '<div class="kept-badge absolute -top-2 -right-2 px-2 py-0.5 rounded-full bg-blue-500 text-white text-xs font-bold shadow-lg">KEPT</div>' : ''}
           <div class="flex justify-between items-start gap-3 mb-3">
              <!-- Selection Checkbox -->
              ${
                isAdmin
                  ? `
              <div class="pt-1 cursor-pointer" onclick="event.stopPropagation(); toggleSelection('${
                t.id
              }')">
                 <div class="relative w-5 h-5 group/checkbox">
                   <!-- Unchecked Circle -->
                   <svg class="w-5 h-5 text-slate-500 transition-all duration-200 group-hover/checkbox:text-blue-400 group-hover/checkbox:scale-110 ${
                     isSelected ? "opacity-0" : "opacity-100"
                   }" 
                        fill="none" 
                        stroke="currentColor" 
                        viewBox="0 0 24 24">
                    <circle cx="12" cy="12" r="9" stroke-width="2"/>
                   </svg>
                   <!-- Checked Circle -->
                   <svg class="w-5 h-5 absolute top-0 left-0 text-blue-500 transition-all duration-200 ${
                     isSelected ? "opacity-100 scale-100" : "opacity-0 scale-50"
                   }" 
                        fill="currentColor" 
                        viewBox="0 0 24 24">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
                   </svg>
                 </div>
              </div>
              `
                  : ""
              }

            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                <div class="flex-1 text-sm font-semibold text-white truncate" title="${escapeHtml(
                  t.filename,
                )}">${escapeHtml(t.filename)}</div>
                <span class="status-badge flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-bold uppercase ${statusClass}">
                  ${statusIcon}
                  ${t.status}
                </span>
              </div>
              <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-400">
                <span class="flex items-center gap-1.5">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"/>
                  </svg>
                  <span class="text-slate-300">${formatBytes(t.bytes)}</span>
                </span>
                ${
                  t.seeders !== undefined && t.seeders !== null
                    ? `<span class="flex items-center gap-1.5">
                         <svg class="w-3.5 h-3.5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                           <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                         </svg>
                         ${t.seeders} seeds
                       </span>`
                    : ""
                }
                ${
                  t.speed !== undefined && t.speed !== null && t.speed > 0
                    ? `<span class="speed-info flex items-center gap-1.5">
                         <svg class="w-3.5 h-3.5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                           <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"/>
                         </svg>
                         ${formatBytes(t.speed)}/s
                       </span>`
                    : ""
                }
                ${addedDate ? `<span class="flex items-center gap-1.5">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
                  </svg>
                  ${addedDate}
                </span>` : ""}
              </div>
            </div>
            
            <!-- Action Buttons -->
            <div class="flex items-center gap-1">
              <!-- Keep/Unkeep Button -->
              <button class="p-2 ${isKept ? 'text-blue-400 bg-blue-500/10' : 'text-slate-500 hover:text-blue-400 hover:bg-blue-500/10'} rounded-lg transition-all opacity-0 group-hover:opacity-100 focus:opacity-100"
                      data-id="${t.id}"
                      data-filename="${escapeHtml(t.filename)}"
                      onclick="event.stopPropagation(); toggleKeep(this.dataset.id, this.dataset.filename)"
                      title="${isKept ? 'Remove from kept' : 'Keep (exempt from auto-delete)'}">
                ${isKept 
                  ? '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>'
                  : '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>'
                }
              </button>
              
              <!-- Individual Delete Action -->
              ${
                isAdmin
                  ? `<button class="p-2 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-all opacity-0 group-hover:opacity-100 focus:opacity-100" 
                      data-id="${t.id}"
                      data-filename="${escapeHtml(t.filename)}"
                      onclick="event.stopPropagation(); confirmDelete('torrent', this.dataset.id, this.dataset.filename)" 
                      title="Delete">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
            </button>`
                 : ""
             }
            </div>
          </div>
          <div class="relative h-2 w-full bg-slate-800 rounded-full overflow-hidden">
            <div class="progress-bar h-full ${progressColor} transition-all duration-500 ${
              t.status === "Downloading" ? "animate-pulse" : ""
            }" style="width: ${t.progress}%"></div>
          </div>
          <div class="mt-1 flex items-center justify-between text-xs">
            <span class="progress-text font-medium ${t.progress >= 100 ? 'text-green-400' : 'text-blue-400'}">${t.progress.toFixed(
            1,
          )}%</span>
            ${
              t.status === "Downloading" && t.speed > 0
                ? `<span class="text-slate-500">${formatBytes(t.bytes * (100 - t.progress) / 100 / t.speed)} remaining</span>`
                : ""
            }
          </div>
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
    countBadge.textContent = `${cachedDownloads.length}${
      totalCount > cachedDownloads.length ? ` of ${totalCount}` : ""
    }${filterInfo} items`;
  } else {
    countBadge.textContent = "0 items";
  }

  if (filteredDownloads.length === 0) {
    list.innerHTML = `<div class="flex flex-col items-center justify-center h-full text-slate-500 py-12">
      <div class="w-20 h-20 mb-4 rounded-full ${filter ? 'bg-slate-800/50' : 'bg-green-500/10'} flex items-center justify-center">
        <svg class="w-10 h-10 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
      </div>
      <p class="text-base font-medium ${filter ? 'text-slate-400' : 'text-slate-400'} mb-1">${
        filter ? "No matching downloads" : "No recent downloads"
      }</p>
      <p class="text-sm text-slate-500">${filter ? 'Try a different search term' : 'Unrestricted links will appear here'}</p>
    </div>`;
    return;
  }

  const html = filteredDownloads
    .map((d) => {
      const safeUrl = sanitizeUrl(d.download);
      const generatedDate = new Date(d.generated);
      const formattedDate = generatedDate.toLocaleDateString();
      const formattedTime = generatedDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      return `
        <div class="group relative glass-effect border border-slate-700/50 rounded-xl p-4 hover:border-purple-500/40 transition-all duration-200">
          <div class="flex justify-between items-start gap-4">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                <div class="flex-1 text-sm font-semibold text-white truncate">
                  <a href="${safeUrl}" target="_blank" rel="noopener noreferrer" class="hover:text-purple-400 transition-colors" title="${escapeHtml(d.filename)}">${escapeHtml(
                  d.filename,
                )}</a>
                </div>
                <span class="flex-shrink-0 px-2 py-0.5 rounded-md text-xs font-bold uppercase bg-purple-500/10 text-purple-400 border border-purple-500/20">${
                  d.host
                }</span>
              </div>
              <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-400">
                <span class="flex items-center gap-1.5">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"/>
                  </svg>
                  <span class="text-slate-300">${formatBytes(d.filesize)}</span>
                </span>
                <span class="flex items-center gap-1.5">
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
                  </svg>
                  ${formattedDate} ${formattedTime}
                </span>
              </div>
            </div>
            <div class="flex items-center gap-1">
              <a href="${safeUrl}" target="_blank" rel="noopener noreferrer" 
                 class="p-2 text-slate-500 hover:text-green-400 hover:bg-green-500/10 rounded-lg transition-all opacity-0 group-hover:opacity-100 focus:opacity-100"
                 title="Download">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/>
                </svg>
              </a>
            ${
              isAdmin
                ? `<button class="p-2 text-slate-500 hover:text-red-400 hover:bg-red-500/10 rounded-lg transition-all opacity-0 group-hover:opacity-100 focus:opacity-100" 
                    data-id="${d.id}"
                    data-filename="${escapeHtml(d.filename)}"
                    onclick="event.stopPropagation(); confirmDelete('download', this.dataset.id, this.dataset.filename)" 
                    title="Delete">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
              </svg>
            </button>`
                : ""
            }
            </div>
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
let previousFocus = null;

function confirmDelete(type, id, name) {
  itemToDelete = { type, id };
  previousFocus = document.activeElement;

  const modal = document.getElementById("confirm-modal");
  document.getElementById("confirm-title").textContent =
    type === "torrent" ? "Delete Torrent?" : "Delete Download?";
  document.getElementById("confirm-message").textContent =
    `Are you sure you want to remove "${name}"?`;

  // Quick action handler setup
  const okBtn = document.getElementById("confirm-ok");
  okBtn.onclick = performDelete;

  modal.classList.remove("hidden", "opacity-0", "pointer-events-none");
  // Ensure aria-hidden is removed if we were using it, though we rely on display:none
  modal.setAttribute("aria-hidden", "false");

  setTimeout(() => {
    modal.querySelector(".glass-effect")?.classList.remove("scale-95");
  }, 10);
  okBtn.focus();
}

function closeConfirmModal() {
  const modal = document.getElementById("confirm-modal");
  modal.classList.add("opacity-0", "pointer-events-none");
  modal.querySelector(".glass-effect")?.classList.add("scale-95");
  modal.setAttribute("aria-hidden", "true");

  setTimeout(() => {
    modal.classList.add("hidden");
    if (previousFocus) {
      previousFocus.focus();
      previousFocus = null;
    }
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

// --- Premium Ring ---

function updatePremiumRing(daysRemaining) {
  const container = document.getElementById("premium-ring-container");
  if (!container) return;

  container.classList.remove("hidden");

  // Calculate percentage (assuming 365 days is 100%)
  const maxDays = 365;
  const percentage = Math.min(100, (daysRemaining / maxDays) * 100);
  const circumference = 2 * Math.PI * 18; // radius = 18
  const offset = circumference - (percentage / 100) * circumference;

  // Color based on days remaining
  let strokeColor = "#22c55e"; // green
  if (daysRemaining < 30) {
    strokeColor = "#ef4444"; // red
  } else if (daysRemaining < 90) {
    strokeColor = "#eab308"; // yellow
  }

  container.innerHTML = `
    <svg class="w-10 h-10 -rotate-90" viewBox="0 0 40 40">
      <circle cx="20" cy="20" r="18" stroke="#334155" stroke-width="3" fill="none"/>
      <circle cx="20" cy="20" r="18" stroke="${strokeColor}" stroke-width="3" fill="none"
        stroke-dasharray="${circumference}"
        stroke-dashoffset="${offset}"
        stroke-linecap="round"/>
    </svg>
    <div class="flex flex-col">
      <span class="text-sm font-bold text-white">${daysRemaining}</span>
      <span class="text-[10px] text-slate-400 -mt-1">days</span>
    </div>
  `;
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