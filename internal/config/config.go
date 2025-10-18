package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	RealDebrid RealDebridConfig `mapstructure:"realdebrid"`
	App        AppConfig        `mapstructure:"app"`
}

// TelegramConfig holds Telegram bot settings
type TelegramConfig struct {
	BotToken       string  `mapstructure:"bot_token"`
	AllowedChatIDs []int64 `mapstructure:"allowed_chat_ids"`
	SuperAdminIDs  []int64 `mapstructure:"super_admin_ids"`
}

// RealDebridConfig holds Real-Debrid API settings
type RealDebridConfig struct {
	APIToken    string `mapstructure:"api_token"`
	BaseURL     string `mapstructure:"base_url"`
	Timeout     int    `mapstructure:"timeout"`
	Proxy       string `mapstructure:"proxy"`
	IpTestURL   string `mapstructure:"ip_test_url"`
	IpVerifyURL string `mapstructure:"ip_verify_url"`
}

// AppConfig holds application settings
type AppConfig struct {
	LogLevel  string          `mapstructure:"log_level"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	MessagesPerSecond int `mapstructure:"messages_per_second"`
	Burst             int `mapstructure:"burst"`
}

var cfg *Config

// Load reads configuration from file and environment variables
func Load(cfgFile string) (*Config, error) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.telegram-rd-bot")
		viper.AddConfigPath("/etc/telegram-rd-bot/")
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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Telegram.BotToken == "" || c.Telegram.BotToken == "YOUR_TELEGRAM_BOT_TOKEN" {
		return fmt.Errorf("telegram bot token is required")
	}

	if c.RealDebrid.APIToken == "" || c.RealDebrid.APIToken == "YOUR_REAL_DEBRID_API_TOKEN" {
		return fmt.Errorf("real-debrid API token is required")
	}

	if len(c.Telegram.AllowedChatIDs) == 0 {
		return fmt.Errorf("at least one allowed chat ID is required")
	}

	if len(c.Telegram.SuperAdminIDs) == 0 {
		return fmt.Errorf("at least one super admin ID is required")
	}

	if c.RealDebrid.BaseURL == "" {
		c.RealDebrid.BaseURL = "https://api.real-debrid.com/rest/1.0"
	}

	if c.RealDebrid.Timeout <= 0 {
		c.RealDebrid.Timeout = 30
	}

	if c.RealDebrid.Proxy != "" {
		_, err := url.Parse(c.RealDebrid.Proxy)
		if err != nil {
			return fmt.Errorf("invalid real-debrid proxy URL: %w", err)
		}
	}

	if c.RealDebrid.IpTestURL != "" {
		_, err := url.Parse(c.RealDebrid.IpTestURL)
		if err != nil {
			return fmt.Errorf("invalid real-debrid IP test URL: %w", err)
		}
	}

	if c.RealDebrid.IpVerifyURL != "" {
		_, err := url.Parse(c.RealDebrid.IpVerifyURL)
		if err != nil {
			return fmt.Errorf("invalid real-debrid IP verify URL: %w", err)
		}
	}

	if c.App.RateLimit.MessagesPerSecond <= 0 {
		c.App.RateLimit.MessagesPerSecond = 25
	}

	if c.App.RateLimit.Burst <= 0 {
		c.App.RateLimit.Burst = 5
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

// IsSuperAdmin checks if a chat ID belongs to a super admin
func (c *Config) IsSuperAdmin(chatID int64) bool {
	for _, id := range c.Telegram.SuperAdminIDs {
		if id == chatID {
			return true
		}
	}
	return false
}
