document.addEventListener("DOMContentLoaded", () => {
  const apiKey = prompt("Please enter your API Key:");
  if (!apiKey) {
    alert("API Key is required to use the dashboard.");
    return;
  }
  window.apiKey = apiKey;

  fetchStatus();
  fetchTorrents();
  fetchDownloads();

  document
    .getElementById("add-torrent-form")
    .addEventListener("submit", addTorrent);
  document
    .getElementById("unrestrict-link-form")
    .addEventListener("submit", unrestrictLink);
});

const API_BASE_URL = "/api";

async function apiFetch(url, options = {}) {
  options.headers = {
    "Content-Type": "application/json",
    "X-API-Key": window.apiKey,
    ...options.headers,
  };
  const response = await fetch(url, options);
  if (!response.ok) {
    const errorData = await response.json();
    throw new Error(errorData.message || "An API error occurred");
  }
  return response.json();
}

function showMessage(message, isError = false) {
  const messageBox = document.getElementById("response-message");
  messageBox.textContent = message;
  messageBox.className = `message-box ${isError ? "error" : "success"}`;
  messageBox.style.display = "block";
  setTimeout(() => {
    messageBox.style.display = "none";
  }, 5000);
}

// --- Fetch Functions ---

async function fetchStatus() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/status`);
    const user = result.data;
    const container = document.getElementById("status-container");
    const expirationDate = new Date(user.expiration).toLocaleDateString();
    container.innerHTML = `
            <p><strong>User:</strong> ${user.username} | <strong>Type:</strong> ${user.type} | <strong>Expires:</strong> ${expirationDate} | <strong>Points:</strong> ${user.points}</p>
        `;
  } catch (error) {
    console.error("Error fetching status:", error);
  }
}

async function fetchTorrents() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/torrents`);
    const torrents = result.data || [];
    const list = document.getElementById("torrents-list");
    list.innerHTML = "";
    if (torrents.length === 0) {
      list.innerHTML = "<p>No active torrents found.</p>";
      return;
    }
    torrents.forEach((t) => {
      const card = document.createElement("div");
      card.className = "item-card";
      card.innerHTML = `
                <h3>${t.filename}</h3>
                <p class="id">ID: ${t.id}</p>
                <p>Status: <strong>${t.status}</strong></p>
                <p>Size: ${formatBytes(t.bytes)}</p>
                <p>Progress: ${t.progress.toFixed(1)}%</p>
                <div class="progress-bar">
                    <div class="progress-bar-inner" style="width: ${t.progress}%;"></div>
                </div>
                <p>Speed: ${formatBytes(t.speed)}/s | Seeders: ${t.seeders || 0}</p>
                <button onclick="deleteTorrent('${t.id}')">Delete</button>
            `;
      list.appendChild(card);
    });
  } catch (error) {
    showMessage(`Error fetching torrents: ${error.message}`, true);
  }
}

async function fetchDownloads() {
  try {
    const result = await apiFetch(`${API_BASE_URL}/downloads`);
    const downloads = result.data || [];
    const list = document.getElementById("downloads-list");
    list.innerHTML = "";
    if (downloads.length === 0) {
      list.innerHTML = "<p>No recent downloads found.</p>";
      return;
    }
    downloads.forEach((d) => {
      const card = document.createElement("div");
      card.className = "item-card";
      card.innerHTML = `
                <h3>${d.filename}</h3>
                <p class="id">ID: ${d.id}</p>
                <p>Size: ${formatBytes(d.filesize)}</p>
                <p>Host: ${d.host}</p>
                <p>Generated: ${new Date(d.generated).toLocaleString()}</p>
                 <button onclick="deleteDownload('${d.id}')">Delete</button>
            `;
      list.appendChild(card);
    });
  } catch (error) {
    showMessage(`Error fetching downloads: ${error.message}`, true);
  }
}

// --- Action Functions ---

async function addTorrent(event) {
  event.preventDefault();
  const magnetLink = document.getElementById("magnet-link").value;
  try {
    const result = await apiFetch(`${API_BASE_URL}/torrents`, {
      method: "POST",
      body: JSON.stringify({ magnet: magnetLink }),
    });
    showMessage(`Torrent added successfully! ID: ${result.data.id}`);
    document.getElementById("magnet-link").value = "";
    fetchTorrents();
  } catch (error) {
    showMessage(`Error adding torrent: ${error.message}`, true);
  }
}

async function unrestrictLink(event) {
  event.preventDefault();
  const hosterLink = document.getElementById("hoster-link").value;
  try {
    const result = await apiFetch(`${API_BASE_URL}/unrestrict`, {
      method: "POST",
      body: JSON.stringify({ link: hosterLink }),
    });
    showMessage(`Link unrestricted: ${result.data.filename}`);
    document.getElementById("hoster-link").value = "";
    fetchDownloads();
  } catch (error) {
    showMessage(`Error unrestricting link: ${error.message}`, true);
  }
}

async function deleteTorrent(id) {
  if (!confirm(`Are you sure you want to delete torrent ${id}?`)) return;
  try {
    await apiFetch(`${API_BASE_URL}/torrents/${id}`, { method: "DELETE" });
    showMessage(`Torrent ${id} deleted successfully.`);
    fetchTorrents();
  } catch (error) {
    showMessage(`Error deleting torrent: ${error.message}`, true);
  }
}

async function deleteDownload(id) {
  if (!confirm(`Are you sure you want to delete download ${id}?`)) return;
  try {
    await apiFetch(`${API_BASE_URL}/downloads/${id}`, { method: "DELETE" });
    showMessage(`Download ${id} deleted successfully.`);
    fetchDownloads();
  } catch (error) {
    showMessage(`Error deleting download: ${error.message}`, true);
  }
}

// --- Utility Functions ---

function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return "0 Bytes";
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + " " + sizes[i];
}
