package db

import (
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────
// parseKVDSN
// ─────────────────────────────────────────────────────────────

func TestParseKVDSN_BasicPairs(t *testing.T) {
	result := parseKVDSN("host=localhost user=alice dbname=mydb")
	tests := []struct {
		key  string
		want string
	}{
		{"host", "localhost"},
		{"user", "alice"},
		{"dbname", "mydb"},
	}
	for _, tt := range tests {
		if got := result[tt.key]; got != tt.want {
			t.Errorf("parseKVDSN key %q: got %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestParseKVDSN_QuotedValues(t *testing.T) {
	// parseKVDSN uses strings.Fields so values cannot contain spaces.
	// Single-word values wrapped in single quotes should be stripped.
	result := parseKVDSN("password='mysecret' host=db")
	if got := result["password"]; got != "mysecret" {
		t.Errorf("parseKVDSN quoted value: got %q, want %q", got, "mysecret")
	}
	if got := result["host"]; got != "db" {
		t.Errorf("parseKVDSN host after quoted value: got %q, want %q", got, "db")
	}
}

func TestParseKVDSN_NoEqualsFieldIgnored(t *testing.T) {
	result := parseKVDSN("noequals host=myhost")
	if _, ok := result["noequals"]; ok {
		t.Error("parseKVDSN: field without '=' should be ignored")
	}
	if got := result["host"]; got != "myhost" {
		t.Errorf("parseKVDSN host: got %q, want %q", got, "myhost")
	}
}

func TestParseKVDSN_EmptyDSN(t *testing.T) {
	result := parseKVDSN("")
	if len(result) != 0 {
		t.Errorf("parseKVDSN empty DSN: expected empty map, got %v", result)
	}
}

func TestParseKVDSN_AllCommonFields(t *testing.T) {
	// Note: values with spaces are not supported by parseKVDSN (uses strings.Fields).
	// Single-word quoted values like 'p@ss' have their quotes stripped.
	dsn := "host=pg.example.com port=5433 user=admin password='p@ss' dbname=appdb sslmode=require"
	result := parseKVDSN(dsn)
	expected := map[string]string{
		"host":     "pg.example.com",
		"port":     "5433",
		"user":     "admin",
		"password": "p@ss",
		"dbname":   "appdb",
		"sslmode":  "require",
	}
	for k, want := range expected {
		if got := result[k]; got != want {
			t.Errorf("parseKVDSN[%q]: got %q, want %q", k, got, want)
		}
	}
}

func TestParseKVDSN_EmptyValue(t *testing.T) {
	// key= with empty value after the equals sign
	result := parseKVDSN("host= user=bob")
	if got := result["host"]; got != "" {
		t.Errorf("parseKVDSN empty value: got %q, want %q", got, "")
	}
	if got := result["user"]; got != "bob" {
		t.Errorf("parseKVDSN user: got %q, want %q", got, "bob")
	}
}

func TestParseKVDSN_SingleQuoteNotPairedIgnored(t *testing.T) {
	// Single quote only at start (not at end) — should NOT be stripped
	result := parseKVDSN("password='only_start")
	got := result["password"]
	// Value starts with ' but does not end with ', so quotes are not removed
	if got != "'only_start" {
		t.Errorf("parseKVDSN unpaired quote: got %q, want %q", got, "'only_start")
	}
}

// ─────────────────────────────────────────────────────────────
// toMigrateDSN — URL prefix forms
// ─────────────────────────────────────────────────────────────

func TestToMigrateDSN_PostgresqlPrefix(t *testing.T) {
	input := "postgresql://user:pass@host:5432/dbname"
	got := toMigrateDSN(input)
	want := "pgx5://user:pass@host:5432/dbname"
	if got != want {
		t.Errorf("toMigrateDSN postgresql://: got %q, want %q", got, want)
	}
}

func TestToMigrateDSN_PostgresPrefix(t *testing.T) {
	input := "postgres://user:pass@host:5432/dbname"
	got := toMigrateDSN(input)
	want := "pgx5://user:pass@host:5432/dbname"
	if got != want {
		t.Errorf("toMigrateDSN postgres://: got %q, want %q", got, want)
	}
}

func TestToMigrateDSN_PgxPrefix(t *testing.T) {
	input := "pgx://user:pass@host:5432/dbname"
	got := toMigrateDSN(input)
	want := "pgx5://user:pass@host:5432/dbname"
	if got != want {
		t.Errorf("toMigrateDSN pgx://: got %q, want %q", got, want)
	}
}

func TestToMigrateDSN_Pgx5PrefixPassthrough(t *testing.T) {
	input := "pgx5://user:pass@host:5432/dbname"
	got := toMigrateDSN(input)
	if got != input {
		t.Errorf("toMigrateDSN pgx5:// passthrough: got %q, want %q", got, input)
	}
}

func TestToMigrateDSN_AllURLPrefixesProducePgx5Scheme(t *testing.T) {
	prefixes := []string{"postgresql://", "postgres://", "pgx://", "pgx5://"}
	rest := "user:pass@db:5432/mydb?sslmode=require"
	for _, pfx := range prefixes {
		input := pfx + rest
		got := toMigrateDSN(input)
		if !strings.HasPrefix(got, "pgx5://") {
			t.Errorf("toMigrateDSN(%q): result %q does not start with pgx5://", input, got)
		}
		if !strings.HasSuffix(got, rest) {
			t.Errorf("toMigrateDSN(%q): result %q does not end with %q", input, got, rest)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// toMigrateDSN — key=value forms
// ─────────────────────────────────────────────────────────────

func TestToMigrateDSN_KVFull(t *testing.T) {
	dsn := "host=pg.example.com port=5433 user=admin password=secret dbname=appdb sslmode=require"
	got := toMigrateDSN(dsn)
	want := "pgx5://admin:secret@pg.example.com:5433/appdb?sslmode=require"
	if got != want {
		t.Errorf("toMigrateDSN KV full: got %q, want %q", got, want)
	}
}

func TestToMigrateDSN_KVNoPassword(t *testing.T) {
	dsn := "host=localhost port=5432 user=alice dbname=testdb sslmode=disable"
	got := toMigrateDSN(dsn)
	want := "pgx5://alice@localhost:5432/testdb?sslmode=disable"
	if got != want {
		t.Errorf("toMigrateDSN KV no password: got %q, want %q", got, want)
	}
}

func TestToMigrateDSN_KVNoUserNoPassword(t *testing.T) {
	dsn := "host=localhost port=5432 dbname=testdb sslmode=disable"
	got := toMigrateDSN(dsn)
	want := "pgx5://localhost:5432/testdb?sslmode=disable"
	if got != want {
		t.Errorf("toMigrateDSN KV no user no password: got %q, want %q", got, want)
	}
}

func TestToMigrateDSN_KVDefaultHost(t *testing.T) {
	dsn := "user=bob dbname=mydb sslmode=disable"
	got := toMigrateDSN(dsn)
	if !strings.Contains(got, "localhost") {
		t.Errorf("toMigrateDSN KV default host: expected 'localhost' in %q", got)
	}
}

func TestToMigrateDSN_KVDefaultPort(t *testing.T) {
	dsn := "host=pg user=bob dbname=mydb sslmode=disable"
	got := toMigrateDSN(dsn)
	if !strings.Contains(got, "5432") {
		t.Errorf("toMigrateDSN KV default port: expected '5432' in %q", got)
	}
}

func TestToMigrateDSN_KVDefaultSSLMode(t *testing.T) {
	dsn := "host=localhost user=bob dbname=mydb"
	got := toMigrateDSN(dsn)
	if !strings.HasSuffix(got, "?sslmode=disable") {
		t.Errorf("toMigrateDSN KV default sslmode: expected suffix ?sslmode=disable in %q", got)
	}
}

func TestToMigrateDSN_KVResultStartsWithPgx5(t *testing.T) {
	dsn := "host=localhost user=bob dbname=mydb"
	got := toMigrateDSN(dsn)
	if !strings.HasPrefix(got, "pgx5://") {
		t.Errorf("toMigrateDSN KV: expected pgx5:// prefix, got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────
// Close — nil safety
// ─────────────────────────────────────────────────────────────

func TestClose_NilPool(t *testing.T) {
	// Close(nil) must not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close(nil) panicked: %v", r)
		}
	}()
	Close(nil)
}
