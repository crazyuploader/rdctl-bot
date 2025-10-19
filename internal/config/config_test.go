package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseConfig_GetDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   DatabaseConfig
		expected string
	}{
		{
			name: "standard configuration",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
				DBName:   "testdb",
				SSLMode:  "disable",
			},
			expected: "host=localhost user=testuser password=testpass dbname=testdb port=5432 sslmode=disable TimeZone=UTC",
		},
		{
			name: "configuration with special characters in password",
			config: DatabaseConfig{
				Host:     "db.example.com",
				Port:     5433,
				User:     "admin",
				Password: "p@ssw0rd!",
				DBName:   "myapp",
				SSLMode:  "require",
			},
			expected: "host=db.example.com user=admin password=p@ssw0rd! dbname=myapp port=5433 sslmode=require TimeZone=UTC",
		},
		{
			name: "empty values",
			config: DatabaseConfig{
				Host:     "",
				Port:     0,
				User:     "",
				Password: "",
				DBName:   "",
				SSLMode:  "",
			},
			expected: "host= user= password= dbname= port=0 sslmode= TimeZone=UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetDSN()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid configuration",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123, 456},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "valid_api_token",
					BaseURL:  "https://api.real-debrid.com/rest/1.0",
					Timeout:  30,
				},
				App: AppConfig{
					LogLevel: "info",
					RateLimit: RateLimitConfig{
						MessagesPerSecond: 25,
						Burst:             5,
					},
				},
				Database: DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					User:     "postgres",
					Password: "password",
					DBName:   "rdctl",
					SSLMode:  "disable",
				},
			},
			wantError: false,
		},
		{
			name: "missing bot token",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "valid_api_token",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "telegram bot token is required",
		},
		{
			name: "placeholder bot token",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "YOUR_TELEGRAM_BOT_TOKEN",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "valid_api_token",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "telegram bot token is required",
		},
		{
			name: "missing API token",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "real-debrid API token is required",
		},
		{
			name: "placeholder API token",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "YOUR_REAL_DEBRID_API_TOKEN",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "real-debrid API token is required",
		},
		{
			name: "no allowed chat IDs",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "valid_api_token",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "at least one allowed chat ID is required",
		},
		{
			name: "no super admin IDs",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "valid_api_token",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "at least one super admin ID is required",
		},
		{
			name: "missing database name",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "valid_api_token",
				},
				Database: DatabaseConfig{
					DBName: "",
				},
			},
			wantError: true,
			errorMsg:  "database name is required",
		},
		{
			name: "invalid proxy URL",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "valid_api_token",
					Proxy:    "://invalid-url",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "invalid real-debrid proxy URL",
		},
		{
			name: "invalid IP test URL",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken:  "valid_api_token",
					IpTestURL: "://invalid-url",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "invalid real-debrid IP test URL",
		},
		{
			name: "invalid IP verify URL",
			config: Config{
				Telegram: TelegramConfig{
					BotToken:       "valid_token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken:    "valid_api_token",
					IpVerifyURL: "://invalid-url",
				},
				Database: DatabaseConfig{
					DBName: "rdctl",
				},
			},
			wantError: true,
			errorMsg:  "invalid real-debrid IP verify URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_DefaultValues(t *testing.T) {
	// Test that validation sets default values correctly
	config := Config{
		Telegram: TelegramConfig{
			BotToken:       "valid_token",
			AllowedChatIDs: []int64{123},
			SuperAdminIDs:  []int64{123},
		},
		RealDebrid: RealDebridConfig{
			APIToken: "valid_api_token",
			// BaseURL, Timeout not set
		},
		App: AppConfig{
			// RateLimit not set
		},
		Database: DatabaseConfig{
			DBName: "rdctl",
			// Host, Port, User, SSLMode not set
		},
	}

	err := config.Validate()
	require.NoError(t, err)

	// Check default values
	assert.Equal(t, "https://api.real-debrid.com/rest/1.0", config.RealDebrid.BaseURL)
	assert.Equal(t, 30, config.RealDebrid.Timeout)
	assert.Equal(t, 25, config.App.RateLimit.MessagesPerSecond)
	assert.Equal(t, 5, config.App.RateLimit.Burst)
	assert.Equal(t, "localhost", config.Database.Host)
	assert.Equal(t, 5432, config.Database.Port)
	assert.Equal(t, "postgres", config.Database.User)
	assert.Equal(t, "disable", config.Database.SSLMode)
}

func TestConfig_IsAllowedChat(t *testing.T) {
	config := &Config{
		Telegram: TelegramConfig{
			AllowedChatIDs: []int64{123, 456, 789},
		},
	}

	tests := []struct {
		name     string
		chatID   int64
		expected bool
	}{
		{"allowed chat ID - first", 123, true},
		{"allowed chat ID - middle", 456, true},
		{"allowed chat ID - last", 789, true},
		{"not allowed chat ID", 999, false},
		{"negative chat ID not allowed", -123, false},
		{"zero chat ID not allowed", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsAllowedChat(tt.chatID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_IsSuperAdmin(t *testing.T) {
	config := &Config{
		Telegram: TelegramConfig{
			SuperAdminIDs: []int64{111, 222},
		},
	}

	tests := []struct {
		name     string
		chatID   int64
		expected bool
	}{
		{"super admin - first", 111, true},
		{"super admin - second", 222, true},
		{"not super admin", 333, false},
		{"negative chat ID not admin", -111, false},
		{"zero chat ID not admin", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsSuperAdmin(tt.chatID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	// Test with non-existent file
	_, err := Load("/nonexistent/path/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoad_WithValidYAMLFile(t *testing.T) {
	// Create a temporary config file
	content := `
telegram:
  bot_token: "test_token_123"
  allowed_chat_ids:
    - 12345
    - 67890
  super_admin_ids:
    - 12345

realdebrid:
  api_token: "test_api_token_456"
  base_url: "https://api.real-debrid.com/rest/1.0"
  timeout: 30

app:
  log_level: "info"
  rate_limit:
    messages_per_second: 20
    burst: 3

database:
  host: "testdb.example.com"
  port: 5432
  user: "testuser"
  password: "testpass"
  dbname: "testdb"
  sslmode: "require"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// Load the config
	cfg, err := Load(tmpFile.Name())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify loaded values
	assert.Equal(t, "test_token_123", cfg.Telegram.BotToken)
	assert.Equal(t, []int64{12345, 67890}, cfg.Telegram.AllowedChatIDs)
	assert.Equal(t, []int64{12345}, cfg.Telegram.SuperAdminIDs)
	assert.Equal(t, "test_api_token_456", cfg.RealDebrid.APIToken)
	assert.Equal(t, "testdb.example.com", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "testuser", cfg.Database.User)
	assert.Equal(t, "testpass", cfg.Database.Password)
	assert.Equal(t, "testdb", cfg.Database.DBName)
	assert.Equal(t, "require", cfg.Database.SSLMode)
}

func TestLoad_WithInvalidYAMLFile(t *testing.T) {
	// Create a temporary config file with invalid YAML
	content := `
telegram:
  bot_token: "test_token"
  allowed_chat_ids: [invalid yaml structure
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// Try to load the invalid config
	_, err = Load(tmpFile.Name())
	require.Error(t, err)
}

func TestLoad_WithValidationErrors(t *testing.T) {
	// Create a config file that parses but fails validation
	content := `
telegram:
  bot_token: ""
  allowed_chat_ids: []
  super_admin_ids: []

realdebrid:
  api_token: ""

database:
  dbname: ""
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// Try to load the config
	_, err = Load(tmpFile.Name())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestConfig_EdgeCases(t *testing.T) {
	t.Run("empty config validation", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Validate()
		require.Error(t, err)
	})

	t.Run("single allowed chat ID", func(t *testing.T) {
		cfg := &Config{
			Telegram: TelegramConfig{
				BotToken:       "token",
				AllowedChatIDs: []int64{123},
				SuperAdminIDs:  []int64{123},
			},
			RealDebrid: RealDebridConfig{
				APIToken: "api_token",
			},
			Database: DatabaseConfig{
				DBName: "db",
			},
		}
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("large chat IDs", func(t *testing.T) {
		cfg := &Config{
			Telegram: TelegramConfig{
				AllowedChatIDs: []int64{9223372036854775807}, // max int64
				SuperAdminIDs:  []int64{9223372036854775806},
			},
		}
		assert.True(t, cfg.IsAllowedChat(9223372036854775807))
		assert.True(t, cfg.IsSuperAdmin(9223372036854775806))
	})

	t.Run("valid proxy URL formats", func(t *testing.T) {
		validProxies := []string{
			"http://proxy.example.com:8080",
			"https://proxy.example.com:8443",
			"socks5://proxy.example.com:1080",
			"http://user:pass@proxy.example.com:8080",
		}

		for _, proxy := range validProxies {
			cfg := &Config{
				Telegram: TelegramConfig{
					BotToken:       "token",
					AllowedChatIDs: []int64{123},
					SuperAdminIDs:  []int64{123},
				},
				RealDebrid: RealDebridConfig{
					APIToken: "api_token",
					Proxy:    proxy,
				},
				Database: DatabaseConfig{
					DBName: "db",
				},
			}
			err := cfg.Validate()
			assert.NoError(t, err, "proxy %s should be valid", proxy)
		}
	})
}