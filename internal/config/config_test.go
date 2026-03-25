package config

import (
	"reflect"
	"strings"
	"testing"
)

// TestGetDSN verifies that GetDSN produces correctly formatted PostgreSQL connection strings.
func TestGetDSN(t *testing.T) {
	tests := []struct {
		name     string
		cfg      DatabaseConfig
		wantPart string
	}{
		{
			name: "all fields populated",
			cfg: DatabaseConfig{
				Host:     "db.example.com",
				Port:     5432,
				User:     "admin",
				Password: "secret",
				DBName:   "mydb",
				SSLMode:  "require",
			},
			wantPart: "host=db.example.com user=admin password=secret dbname=mydb port=5432 sslmode=require TimeZone=UTC",
		},
		{
			name: "default port",
			cfg: DatabaseConfig{
				Host:    "localhost",
				Port:    5432,
				User:    "postgres",
				DBName:  "testdb",
				SSLMode: "disable",
			},
			wantPart: "port=5432",
		},
		{
			name: "custom port",
			cfg: DatabaseConfig{
				Host:    "localhost",
				Port:    5433,
				User:    "postgres",
				DBName:  "testdb",
				SSLMode: "disable",
			},
			wantPart: "port=5433",
		},
		{
			name: "sslmode require",
			cfg: DatabaseConfig{
				Host:    "localhost",
				Port:    5432,
				User:    "postgres",
				DBName:  "mydb",
				SSLMode: "require",
			},
			wantPart: "sslmode=require",
		},
		{
			name: "timezone always UTC",
			cfg: DatabaseConfig{
				Host:    "localhost",
				Port:    5432,
				User:    "postgres",
				DBName:  "mydb",
				SSLMode: "disable",
			},
			wantPart: "TimeZone=UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.cfg.GetDSN()
			if !strings.Contains(dsn, tt.wantPart) {
				t.Errorf("GetDSN() = %q, want it to contain %q", dsn, tt.wantPart)
			}
		})
	}
}

// TestGetDSN_PostgreSQLOnly verifies that DatabaseConfig produces correct PostgreSQL DSN
// and does not contain SQLite-related fields (SQLitePath or IsSQLite).
func TestGetDSN_PostgreSQLOnly(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	dsn := cfg.GetDSN()

	// Must contain all PostgreSQL-specific DSN components
	required := []string{"host=", "user=", "password=", "dbname=", "port=", "sslmode=", "TimeZone=UTC"}
	for _, part := range required {
		if !strings.Contains(dsn, part) {
			t.Errorf("GetDSN() = %q, missing required component %q", dsn, part)
		}
	}

	// Use reflection to ensure DatabaseConfig does not have SQLite-related fields
	dbConfigType := reflect.TypeOf(DatabaseConfig{})
	_, hasSQLitePath := dbConfigType.FieldByName("SQLitePath")
	if hasSQLitePath {
		t.Error("DatabaseConfig should not contain SQLitePath field")
	}
	_, hasIsSQLite := dbConfigType.FieldByName("IsSQLite")
	if hasIsSQLite {
		t.Error("DatabaseConfig should not contain IsSQLite field")
	}
}

// buildMinimalConfig creates a Config with the minimum fields needed to pass Validate.
func buildMinimalConfig() *Config {
	return &Config{
		Telegram: TelegramConfig{
			BotToken:       "valid_token",
			AllowedChatIDs: []int64{100},
			SuperAdminIDs:  []int64{200},
		},
		RealDebrid: RealDebridConfig{
			APIToken: "valid_rd_token",
		},
		Database: DatabaseConfig{
			DBName: "mydb",
		},
		Web: WebConfig{
			APIKey: "myapikey",
		},
	}
}

// TestValidate_DatabaseDefaults checks that missing database fields get sensible defaults applied.
func TestValidate_DatabaseDefaults(t *testing.T) {
	c := buildMinimalConfig()
	// Leave Host, Port, User, SSLMode empty — Validate should fill them in.

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}

	if c.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, want %q", c.Database.Host, "localhost")
	}
	if c.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want %d", c.Database.Port, 5432)
	}
	if c.Database.User != "postgres" {
		t.Errorf("Database.User = %q, want %q", c.Database.User, "postgres")
	}
	if c.Database.SSLMode != "disable" {
		t.Errorf("Database.SSLMode = %q, want %q", c.Database.SSLMode, "disable")
	}
}

// TestValidate_RequiresDatabaseName checks that an empty DBName causes an error.
func TestValidate_RequiresDatabaseName(t *testing.T) {
	c := buildMinimalConfig()
	c.Database.DBName = ""

	err := c.Validate(false)
	if err == nil {
		t.Fatal("Validate() expected error for empty DBName, got nil")
	}
	if !strings.Contains(err.Error(), "database name is required") {
		t.Errorf("Validate() error = %q, want it to contain %q", err.Error(), "database name is required")
	}
}

// TestValidate_ExistingDatabaseHostPreserved verifies that a pre-configured host is not overwritten.
func TestValidate_ExistingDatabaseHostPreserved(t *testing.T) {
	c := buildMinimalConfig()
	c.Database.Host = "custom-db-host"

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.Database.Host != "custom-db-host" {
		t.Errorf("Database.Host = %q, want %q (should not be overwritten)", c.Database.Host, "custom-db-host")
	}
}

// TestValidate_ExistingPortPreserved verifies that a pre-configured port is not overwritten.
func TestValidate_ExistingPortPreserved(t *testing.T) {
	c := buildMinimalConfig()
	c.Database.Port = 5433

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.Database.Port != 5433 {
		t.Errorf("Database.Port = %d, want %d (should not be overwritten)", c.Database.Port, 5433)
	}
}

// TestValidate_IPTestURL_Valid verifies valid IP test URLs are accepted.
func TestValidate_IPTestURL_Valid(t *testing.T) {
	c := buildMinimalConfig()
	c.RealDebrid.IPTestURL = "https://api.ipify.org?format=json"

	if err := c.Validate(false); err != nil {
		t.Errorf("Validate() with valid IPTestURL returned error: %v", err)
	}
}

// TestValidate_IPVerifyURL_Valid verifies valid IP verify URLs are accepted.
func TestValidate_IPVerifyURL_Valid(t *testing.T) {
	c := buildMinimalConfig()
	c.RealDebrid.IPVerifyURL = "https://api.ipify.org?format=json"

	if err := c.Validate(false); err != nil {
		t.Errorf("Validate() with valid IPVerifyURL returned error: %v", err)
	}
}

// TestValidate_RateLimitDefaults checks that rate limit defaults are applied when zero.
func TestValidate_RateLimitDefaults(t *testing.T) {
	c := buildMinimalConfig()
	// Leave MessagesPerSecond and Burst at zero.

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.App.RateLimit.MessagesPerSecond != 25 {
		t.Errorf("RateLimit.MessagesPerSecond = %d, want 25", c.App.RateLimit.MessagesPerSecond)
	}
	if c.App.RateLimit.Burst != 5 {
		t.Errorf("RateLimit.Burst = %d, want 5", c.App.RateLimit.Burst)
	}
}

// TestValidate_WebOnlySkipsTelegramValidation verifies webOnly=true skips Telegram token checks.
func TestValidate_WebOnlySkipsTelegramValidation(t *testing.T) {
	c := buildMinimalConfig()
	// Clear Telegram fields — should not matter in webOnly mode.
	c.Telegram.BotToken = ""
	c.Telegram.AllowedChatIDs = nil
	c.Telegram.SuperAdminIDs = nil

	if err := c.Validate(true); err != nil {
		t.Errorf("Validate(webOnly=true) should not require Telegram fields, got error: %v", err)
	}
}

// TestValidate_RequiresRealDebridToken verifies that missing RD API token causes an error.
func TestValidate_RequiresRealDebridToken(t *testing.T) {
	c := buildMinimalConfig()
	c.RealDebrid.APIToken = ""

	err := c.Validate(false)
	if err == nil {
		t.Fatal("Validate() expected error for missing RD API token, got nil")
	}
	if !strings.Contains(err.Error(), "real-debrid API token is required") {
		t.Errorf("Validate() error = %q, want it to contain %q", err.Error(), "real-debrid API token is required")
	}
}

// TestValidate_RequiresWebAPIKey verifies that missing web APIKey causes an error.
func TestValidate_RequiresWebAPIKey(t *testing.T) {
	c := buildMinimalConfig()
	c.Web.APIKey = ""

	err := c.Validate(false)
	if err == nil {
		t.Fatal("Validate() expected error for missing web API key, got nil")
	}
	if !strings.Contains(err.Error(), "web api_key is required") {
		t.Errorf("Validate() error = %q, want it to contain %q", err.Error(), "web api_key is required")
	}
}

// TestValidate_RealDebridBaseURLDefault checks that a missing BaseURL gets a sensible default.
func TestValidate_RealDebridBaseURLDefault(t *testing.T) {
	c := buildMinimalConfig()
	c.RealDebrid.BaseURL = ""

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.RealDebrid.BaseURL != "https://api.real-debrid.com/rest/1.0" {
		t.Errorf("RealDebrid.BaseURL = %q, want default URL", c.RealDebrid.BaseURL)
	}
}

// TestValidate_RealDebridTimeoutDefault checks that a zero Timeout gets the default value of 30.
func TestValidate_RealDebridTimeoutDefault(t *testing.T) {
	c := buildMinimalConfig()
	c.RealDebrid.Timeout = 0

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.RealDebrid.Timeout != 30 {
		t.Errorf("RealDebrid.Timeout = %d, want 30", c.RealDebrid.Timeout)
	}
}

// TestValidate_WebListenAddrDefault checks that an empty ListenAddr gets the default value.
func TestValidate_WebListenAddrDefault(t *testing.T) {
	c := buildMinimalConfig()
	c.Web.ListenAddr = ""

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.Web.ListenAddr != ":8080" {
		t.Errorf("Web.ListenAddr = %q, want %q", c.Web.ListenAddr, ":8080")
	}
}

// TestValidate_MetricsRequiresCredentials verifies that enabling metrics requires both user and password.
func TestValidate_MetricsRequiresCredentials(t *testing.T) {
	c := buildMinimalConfig()
	c.Web.Metrics.Enabled = true
	c.Web.Metrics.User = ""
	c.Web.Metrics.Password = ""

	err := c.Validate(false)
	if err == nil {
		t.Fatal("Validate() expected error when metrics enabled with no credentials, got nil")
	}
	if !strings.Contains(err.Error(), "web metrics user and password are required") {
		t.Errorf("Validate() error = %q, want metrics credentials error", err.Error())
	}
}

// TestValidate_SSLModePreserved checks that a pre-set SSLMode is not overwritten.
func TestValidate_SSLModePreserved(t *testing.T) {
	c := buildMinimalConfig()
	c.Database.SSLMode = "require"

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.Database.SSLMode != "require" {
		t.Errorf("Database.SSLMode = %q, want %q (should not be overwritten)", c.Database.SSLMode, "require")
	}
}

// TestValidate_TokenExpiryDefault checks that a zero TokenExpiryMinutes gets the default.
func TestValidate_TokenExpiryDefault(t *testing.T) {
	c := buildMinimalConfig()
	c.Web.TokenExpiryMinutes = 0

	if err := c.Validate(false); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if c.Web.TokenExpiryMinutes != 60 {
		t.Errorf("Web.TokenExpiryMinutes = %d, want 60", c.Web.TokenExpiryMinutes)
	}
}