document.addEventListener("DOMContentLoaded", () => {
  checkLogin();
  setupEventListeners();
});

const API_BASE_URL = "/api";
let refreshIntervals = {};

// --- Auth & Init ---

function checkLogin() {
  const key = localStorage.getItem("rdctl_api_key");
  if (key) {
    window.apiKey = key;
    showDashboard();
  } else {
    showLogin();
  }
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
    showDashboard();
  }
}

function logout() {
  localStorage.removeItem("rdctl_api_key");
  window.apiKey = null;
  clearInterval(refreshIntervals.torrents);
  clearInterval(refreshIntervals.downloads);
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
  options.headers = {
    "Content-Type": "application/json",
    "X-API-Key": window.apiKey,
    ...options.headers,
  };

  try {
    const response = await fetch(url, options);
    if (response.status === 401) {
      logout();
      throw new Error("Unauthorized");
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

async function fetchTorrents() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/torrents?limit=50`);
    const torrents = result.data || [];
    const totalCount = result.total_count || torrents.length;
    const list = document.getElementById("torrents-list");
    const countBadge = document.getElementById("torrents-count");

    // Update count badge - show "X of Y" if there are more items
    if (torrents.length > 0) {
      if (totalCount > torrents.length) {
        countBadge.textContent = `(${torrents.length} of ${totalCount})`;
      } else {
        countBadge.textContent = `(${torrents.length})`;
      }
    } else {
      countBadge.textContent = "";
    }

    if (torrents.length === 0) {
      list.innerHTML = `<div class="item-card"><p style="text-align:center; color: var(--text-secondary)">No active torrents</p></div>`;
      return;
    }

    const html = torrents
      .map((t) => {
        const statusClass =
          t.status === "Downloaded"
            ? "status-downloaded"
            : t.status === "Downloading"
              ? "status-downloading"
              : "status-badge";
        
        const addedDate = t.added ? new Date(t.added).toLocaleDateString() : '';

        return `
            <div class="item-card">
                <div class="item-header">
                    <div style="flex:1">
                        <div class="item-title" title="${t.filename}">${t.filename}</div>
                        <div class="item-meta">
                            <span>${formatBytes(t.bytes)}</span>
                            <span class="status-badge ${statusClass}">${t.status}</span>
                            ${t.seeders !== undefined && t.seeders !== null ? `<span>${t.seeders} seeds</span>` : ''}
                            ${t.speed !== undefined && t.speed !== null && t.speed > 0 ? `<span>${formatBytes(t.speed)}/s</span>` : ''}
                            ${addedDate ? `<span title="Added date">${addedDate}</span>` : ''}
                        </div>
                    </div>
                    <button class="delete-btn" onclick="confirmDelete('torrent', '${t.id}', '${escapeHtml(t.filename)}')" title="Delete">
                        🗑
                    </button>
                </div>
                <div class="progress-container">
                    <div class="progress-fill" style="width: ${t.progress}%"></div>
                </div>
            </div>
            `;
      })
      .join("");

    if (list.innerHTML !== html) {
      list.innerHTML = html;
    }
  } catch (error) {
    showToast(`Error fetching torrents: ${error.message}`, "error");
  }
}

async function fetchDownloads() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/downloads?limit=50`);
    const downloads = result.data || [];
    const totalCount = result.total_count || downloads.length;
    const list = document.getElementById("downloads-list");
    const countBadge = document.getElementById("downloads-count");

    // Update count badge - show "X of Y" if there are more items
    if (downloads.length > 0) {
      if (totalCount > downloads.length) {
        countBadge.textContent = `(${downloads.length} of ${totalCount})`;
      } else {
        countBadge.textContent = `(${downloads.length})`;
      }
    } else {
      countBadge.textContent = "";
    }

    if (downloads.length === 0) {
      list.innerHTML = `<div class="item-card"><p style="text-align:center; color: var(--text-secondary)">No recent downloads</p></div>`;
      return;
    }

    const html = downloads
      .map((d) => {
        return `
            <div class="item-card">
                <div class="item-header">
                     <div style="flex:1">
                        <div class="item-title"><a href="${d.download}" target="_blank" style="color:inherit;text-decoration:none">${d.filename}</a></div>
                        <div class="item-meta">
                            <span>${formatBytes(d.filesize)}</span>
                            <span>${d.host}</span>
                            <span>${new Date(d.generated).toLocaleDateString()}</span>
                        </div>
                    </div>
                    <button class="delete-btn" onclick="confirmDelete('download', '${d.id}', '${escapeHtml(d.filename)}')" title="Delete">
                        🗑
                    </button>
                </div>
            </div>
            `;
      })
      .join("");

    if (list.innerHTML !== html) {
      list.innerHTML = html;
    }
  } catch (error) {
    showToast(`Error fetching downloads: ${error.message}`, "error");
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
