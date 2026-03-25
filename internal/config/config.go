package config

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	RealDebrid RealDebridConfig `mapstructure:"realdebrid"`
	App        AppConfig        `mapstructure:"app"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Web        WebConfig        `mapstructure:"web"`
}

// WebConfig holds all web server configuration
type WebConfig struct {
	ListenAddr         string        `mapstructure:"listen_addr"`
	APIKey             string        `mapstructure:"api_key"`
	DashboardURL       string        `mapstructure:"dashboard_url"`
	TokenExpiryMinutes int           `mapstructure:"token_expiry_minutes"`
	Limiter            LimiterConfig `mapstructure:"limiter"`
	Metrics            MetricsConfig `mapstructure:"metrics"`
}

// LimiterConfig holds web server rate limiting settings
type LimiterConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	Max               int  `mapstructure:"max"`
	ExpirationSeconds int  `mapstructure:"expiration_seconds"`
	// Security settings
	BanDurationSeconds int `mapstructure:"ban_duration_seconds"`
	AuthFailLimit      int `mapstructure:"auth_fail_limit"`
	AuthFailWindow     int `mapstructure:"auth_fail_window"`
}

// MetricsConfig holds prometheus metrics settings
type MetricsConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

// TelegramConfig holds Telegram bot settings
type TelegramConfig struct {
	BotToken        string             `mapstructure:"bot_token"`
	AllowedChatIDs  []int64            `mapstructure:"allowed_chat_ids"`
	SuperAdminIDs   []int64            `mapstructure:"super_admin_ids"`
	AllowedTopicIDs map[string][]int64 `mapstructure:"allowed_topic_ids"` // map[chatID][]topicID - if set, bot only responds in these topics for that chat
}

// RealDebridConfig holds Real-Debrid API settings
type RealDebridConfig struct {
	APIToken    string `mapstructure:"api_token"`
	BaseURL     string `mapstructure:"base_url"`
	Timeout     int    `mapstructure:"timeout"`
	Proxy       string `mapstructure:"proxy"`
	IPTestURL   string `mapstructure:"ip_test_url"`
	IPVerifyURL string `mapstructure:"ip_verify_url"`
}

// AppConfig holds application settings
type AppConfig struct {
	LogLevel          string                  `mapstructure:"log_level"`
	RateLimit         RateLimitConfig         `mapstructure:"rate_limit"`
	MaxKeptTorrents   int                     `mapstructure:"max_kept_torrents"` // Max kept torrents per non-admin user (0 = unlimited, admins always unlimited)
	AutoDeleteDays    int                     `mapstructure:"auto_delete_days"`  // Default auto-delete days fallback when not set in DB
	AutoDeleteWarning AutoDeleteWarningConfig `mapstructure:"auto_delete_warning"`
}

// AutoDeleteWarningConfig holds settings for auto-delete warning notifications
type AutoDeleteWarningConfig struct {
	ChatID      int64 `mapstructure:"chat_id"`      // Chat ID to send warnings to (0 = disabled)
	TopicID     int   `mapstructure:"topic_id"`     // Topic/thread ID (0 = main chat)
	HoursBefore int   `mapstructure:"hours_before"` // Hours before deletion to send warning (default: 6)
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	MessagesPerSecond int `mapstructure:"messages_per_second"`
	Burst             int `mapstructure:"burst"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	// PostgreSQL configuration
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

var cfg *Config

// GetDSN returns the PostgreSQL connection DSN
func (d *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=UTC",
		d.Host, d.User, d.Password, d.DBName, d.Port, d.SSLMode)
}

// Load reads configuration into a Config from the specified file or from standard locations,
// supports overriding via environment variables prefixed with TGRD (dots replaced by underscores),
// unmarshals the resulting configuration, and validates it before returning it or an error.
// If cfgFile is non-empty it is used as the config file; otherwise a YAML file named "config"
// Load loads application configuration from the given file or from standard locations,
// applying environment variable overrides, unmarshals the result into a Config, validates it,
// and stores the loaded configuration in the package-level cfg variable.
//
// If cfgFile is non-empty it is used as the explicit config file. Otherwise the loader
// searches for a file named "config.yaml" in the current directory, $HOME/.telegram-rd-bot,
// and /etc/telegram-rd-bot. Environment variables prefixed with "TGRD" (dot replaced by underscore)
// override config values.
//
// On success the configured *Config is returned. An error is returned if the config file
// cannot be read, cannot be unmarshaled, or fails validation.
func Load(cfgFile string) (*Config, error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.telegram-rd-bot")
		viper.AddConfigPath("/etc/telegram-rd-bot")
	}

	// Environment variable support
	viper.SetEnvPrefix("TGRD")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read configuration
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if cfg.App.MaxKeptTorrents < 0 {
		return nil, errors.New("invalid configuration: App.MaxKeptTorrents must be >= 0")
	}
	if cfg.App.AutoDeleteDays < 0 {
		return nil, errors.New("invalid configuration: App.AutoDeleteDays must be >= 0")
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate(webOnly bool) error {
	// Telegram validation
	if !webOnly {
		if c.Telegram.BotToken == "" || c.Telegram.BotToken == "YOUR_TELEGRAM_BOT_TOKEN" {
			return fmt.Errorf("telegram bot token is required")
		}

		if len(c.Telegram.AllowedChatIDs) == 0 {
			return fmt.Errorf("at least one allowed chat ID is required")
		}

		if len(c.Telegram.SuperAdminIDs) == 0 {
			return fmt.Errorf("at least one super admin ID is required")
		}
	}

	if c.RealDebrid.APIToken == "" || c.RealDebrid.APIToken == "YOUR_REAL_DEBRID_API_TOKEN" {
		return fmt.Errorf("real-debrid API token is required")
	}

	// RealDebrid validation
	if c.RealDebrid.BaseURL == "" {
		c.RealDebrid.BaseURL = "https://api.real-debrid.com/rest/1.0"
	}

	if c.RealDebrid.Timeout == 0 {
		c.RealDebrid.Timeout = 30
	}

	if c.RealDebrid.Proxy != "" {
		if _, err := url.Parse(c.RealDebrid.Proxy); err != nil {
			return fmt.Errorf("invalid real-debrid proxy URL: %w", err)
		}
	}

	if c.RealDebrid.IPTestURL != "" {
		if _, err := url.Parse(c.RealDebrid.IPTestURL); err != nil {
			return fmt.Errorf("invalid real-debrid IP test URL: %w", err)
		}
	}

	if c.RealDebrid.IPVerifyURL != "" {
		if _, err := url.Parse(c.RealDebrid.IPVerifyURL); err != nil {
			return fmt.Errorf("invalid real-debrid IP verify URL: %w", err)
		}
	}

	// App validation
	if c.App.RateLimit.MessagesPerSecond == 0 {
		c.App.RateLimit.MessagesPerSecond = 25
	}

	if c.App.RateLimit.Burst == 0 {
		c.App.RateLimit.Burst = 5
	}

	// Auto-delete warning defaults
	if c.App.AutoDeleteWarning.HoursBefore == 0 {
		c.App.AutoDeleteWarning.HoursBefore = 6
	}

	// Database validation - PostgreSQL settings
	if c.Database.Host == "" {
		c.Database.Host = "localhost"
	}

	if c.Database.Port == 0 {
		c.Database.Port = 5432
	}

	if c.Database.User == "" {
		c.Database.User = "postgres"
	}

	if c.Database.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	if c.Database.SSLMode == "" {
		c.Database.SSLMode = "disable"
	}

	if c.Web.ListenAddr == "" {
		c.Web.ListenAddr = ":8080"
	}
	if c.Web.APIKey == "" {
		return fmt.Errorf("web api_key is required for dashboard access")
	}
	if c.Web.DashboardURL == "" {
		c.Web.DashboardURL = "http://localhost" + c.Web.ListenAddr
	}
	if c.Web.TokenExpiryMinutes == 0 {
		c.Web.TokenExpiryMinutes = 60 // Default 1 hour
	}

	// Limiter defaults
	if c.Web.Limiter.Max == 0 {
		c.Web.Limiter.Max = 3 // Strict default: 3 RPS
	}
	if c.Web.Limiter.ExpirationSeconds == 0 {
		c.Web.Limiter.ExpirationSeconds = 1
	}
	// Security defaults
	if c.Web.Limiter.BanDurationSeconds == 0 {
		c.Web.Limiter.BanDurationSeconds = 3600 // 1 hour
	}
	if c.Web.Limiter.AuthFailLimit == 0 {
		c.Web.Limiter.AuthFailLimit = 10 // 10 failures
	}
	if c.Web.Limiter.AuthFailWindow == 0 {
		c.Web.Limiter.AuthFailWindow = 60 // per 60 seconds
	}

	// Metrics defaults
	if c.Web.Metrics.Enabled {
		if c.Web.Metrics.User == "" || c.Web.Metrics.Password == "" {
			return fmt.Errorf("web metrics user and password are required when enabled")
		}
	}

	return nil
}

// Get returns the loaded configuration
func Get() *Config {
	return cfg
}

// IsAllowedChat checks if a chat ID is allowed
func (c *Config) IsAllowedChat(chatID int64) bool {
	for _, id := range c.Telegram.AllowedChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

// IsSuperAdmin checks if a user ID belongs to a super admin
func (c *Config) IsSuperAdmin(userID int64) bool {
	return slices.Contains(c.Telegram.SuperAdminIDs, userID)
}

// IsAllowedTopic checks if the given topic is allowed for the given chat.
// If AllowedTopicIDs is not configured (nil), it returns true (all topics allowed).
// If the chat is not in the map, it returns true (topic restriction not configured for this chat).
// If the chat is configured with an empty list, it returns true (all topics allowed for this chat).
// If the chat is configured with specific IDs, it returns true only if the topicID is in the list.
// If the chat has specific topics configured and the message is in the main chat (topicID=0), it returns false.
func (c *Config) IsAllowedTopic(chatID int64, topicID int) bool {
	if c.Telegram.AllowedTopicIDs == nil {
		return true
	}
	allowedTopics, ok := c.Telegram.AllowedTopicIDs[fmt.Sprintf("%d", chatID)]
	if !ok {
		return true // Chat not in map - topic restriction not configured, allow all
	}
	if len(allowedTopics) == 0 {
		return true // Empty list means all topics allowed for this chat
	}
	// If specific topics are configured and message is in main chat (not a thread), block it
	if topicID == 0 {
		return false
	}
	return slices.Contains(allowedTopics, int64(topicID))
}
