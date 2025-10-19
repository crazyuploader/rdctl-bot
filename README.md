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
