# Telegram Real-Debrid Bot

A powerful Telegram bot for managing Real-Debrid torrents and hoster links on the go, written in Go.

## Configuration

The bot is configured using a `config.yaml` file. An example configuration can be found in `example-config.yaml`.

Key configuration options include:

- `telegram.bot_token`: Your Telegram bot token.
- `telegram.allowed_chat_ids`: A list of Telegram chat IDs that are allowed to interact with the bot.
- `telegram.super_admin_ids`: A list of Telegram chat IDs that have super admin privileges.
- `realdebrid.api_token`: Your Real-Debrid API token.
- `realdebrid.base_url`: The base URL for the Real-Debrid API (default: `https://api.real-debrid.com/rest/1.0`).
- `realdebrid.timeout`: The timeout for Real-Debrid API requests in seconds (default: `30`).
- `realdebrid.proxy`: (Optional) An HTTP or SOCKS5 proxy URL to use for Real-Debrid API requests (e.g., `http://localhost:8080` or `socks5://user:pass@host:port`).
- `realdebrid.ip_test_url`: (Optional) A URL to use for IP testing when a proxy is configured. If not provided, `https://api.ipify.org?format=json` will be used.
- `realdebrid.ip_verify_url`: (Optional) A second URL to use for IP verification. This is particularly useful for Media Flow Proxy setups. If provided, the IP obtained from `ip_test_url` (or the default `https://api.ipify.org?format=json`) must match the IP from this URL. If the IPs do not match, the bot will exit, ensuring that both the bot and the Media Flow Proxy are operating from the same external IP address.
- `app.log_level`: The logging level (e.g., `debug`, `info`, `warn`, `error`).
- `app.rate_limit.messages_per_second`: The maximum number of messages per second the bot can send to Telegram.
- `app.rate_limit.burst`: The maximum burst of messages the bot can send to Telegram.
- `database.host`: The database host.
- `database.port`: The database port.
- `database.user`: The database user.
- `database.password`: The database password.
- `database.dbname`: The database name.
- `database.sslmode`: The SSL mode for the database connection (e.g., `disable`, `require`).
- `web.listen_addr`: Address for the web server (default: `:8089`).
- `web.dashboard_url`: Base URL for the dashboard (e.g., `http://localhost:8089`).
- `web.token_expiry_minutes`: Dashboard session token validity (default: 60 minutes).

## 🌐 Web Dashboard

The bot includes a web-based dashboard for managing your Real-Debrid downloads visually.

### Features

- **Real-time Status**: View account status, premium days remaining, and fidelity points.
- **Torrent Management**: View, filter, and search active torrents.
- **File Selection**: Manage file selection for multi-file torrents.
- **Batch Actions**: Delete multiple torrents at once or manage download links.
- **Dark Mode UI**: Modern, responsive interface with dark mode support.

### Accessing the Dashboard

1. Send the `/dashboard` command to the bot in Telegram.
2. The bot will generate a **temporary, one-time login link**.
3. Click the link to access the dashboard.
   - The link is valid for **60 minutes** by default (configurable).
   - Once authenticated, your session is secure and tied to your browser.

**Note**: The dashboard runs on port `8089` by default. Ensure this port is accessible or behind a reverse proxy.

## 🐳 Running with Docker Compose

You can easily run the bot using **Docker Compose**, which automatically handles deployment configuration and environment variables.

### Quick Start (Docker Compose)

1. **Create config.yaml from example**

   ```bash
   cp example-config.yaml config.yaml
   ```

   Edit your credentials inside [`config.yaml`](example-config.yaml).

2. **Start the bot**
   ```bash
   docker compose up -d
   ```

That’s it — the bot will load configuration from your mounted `config.yaml` automatically.
