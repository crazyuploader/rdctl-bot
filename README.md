# Telegram Real-Debrid Bot

A powerful Telegram bot for managing Real-Debrid torrents and hoster links on the go, written in Go.

## Configuration

The bot uses `config.yaml`. See `example-config.yaml` for a template.

**Configuration Options:**

- `telegram.bot_token`: Your Telegram bot token.
- `telegram.allowed_chat_ids`: List of allowed chat IDs.
- `telegram.super_admin_ids`: List of super admin chat IDs.
- `realdebrid.api_token`: Your Real-Debrid API token.
- `realdebrid.base_url`: API base URL (default: `https://api.real-debrid.com/rest/1.0`).
- `realdebrid.timeout`: Request timeout in seconds (default: `30`).
- `realdebrid.proxy`: (Optional) HTTP/SOCKS5 proxy URL.
- `realdebrid.ip_test_url`: (Optional) URL for IP testing (e.g. via proxy).
- `realdebrid.ip_verify_url`: (Optional) Second URL to verify IP consistency.
- `app.log_level`: Logging level (`debug`, `info`, `warn`, `error`).
- `app.rate_limit.messages_per_second`: Max messages/sec to Telegram.
- `app.rate_limit.burst`: Max message burst to Telegram.
- `database.host`, `port`, `user`, `password`, `dbname`, `sslmode`: Database connection details.
- `web.listen_addr`: Web server address (default: `:8089`).
- `web.dashboard_url`: Base URL for dashboard links.
- `web.token_expiry_minutes`: Session validity (default: 60 min).
- `web.limiter.enabled`: Enable rate limiting (default: `true`).
- `web.limiter.max`: Max requests per window (default: `3`).
- `web.limiter.expiration_seconds`: Rate limit window (default: `1`).
- `web.limiter.ban_duration_seconds`: Ban duration for auth failures (default: `3600`).
- `web.limiter.auth_fail_limit`: Max failed login attempts (default: `10`).
- `web.limiter.auth_fail_window`: Time window for tracking failures (default: `60`).

## 🌐 Web Dashboard

A visual interface for managing downloads. Access it by sending `/dashboard` to the bot.

**Setup**:

- Runs on port `8089`.
- **Reverse Proxy**: Configure your proxy (Nginx/Caddy) to pass standard headers (`X-Forwarded-For`, `X-Forwarded-Proto`).
- Set `web.dashboard_url` in config to your public domain.

## 🐳 Quick Start (Docker Compose)

1. `cp example-config.yaml config.yaml` and edit credentials.
2. Run `docker compose up -d`.

## License

[MIT](LICENSE)
