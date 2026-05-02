package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// It returns an error if migrations fail, if the DSN cannot be parsed, if the pool cannot be created, or if the initial ping fails (the pool is closed on ping failure).
func Init(dsn string) (*pgxpool.Pool, error) {
	if err := runMigrations(dsn); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping: %w", err)
	}
	log.Println("Database connected and migrations completed successfully!")
	return pool, nil
}

// Close closes the given connection pool. Calling Close with a nil pool is a no-op.
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// runMigrations applies the embedded SQL migrations to the database identified by dsn.
// It returns an error if creating the migration source or migrate instance fails, or if applying migrations fails (a "no change" result is treated as success).
func runMigrations(dsn string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	migrateDSN := toMigrateDSN(dsn)
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateDSN)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}
	return nil
}

// toMigrateDSN converts a key=value DSN (or URL DSN) to the "pgx5://" scheme
// required by the golang-migrate pgx/v5 driver.
//
// If the DSN already starts with a URL scheme, it is converted to pgx5://.
// If the DSN is in key=value form (e.g. "host=... user=... ...") it is
// toMigrateDSN converts a PostgreSQL DSN (URL or libpq key=value form) into a
// pgx5:// URL suitable for use with the golang-migrate pgx/v5 driver.
//
// If the input already uses a URL scheme (postgresql://, postgres://, pgx://,
// or pgx5://) the scheme is replaced with pgx5:// and the remainder is returned.
// For key=value form, the string is parsed with parseKVDSN and the resulting
// components are mapped into a URL: host defaults to "localhost", port to
// "5432", and sslmode to "disable" when absent. The returned URL includes user
// and password in the authority when provided and has the form
// pgx5://[user[:password]@]host:port/dbname?sslmode=value.
func toMigrateDSN(dsn string) string {
	// Already a URL — just swap the scheme.
	for _, prefix := range []string{"postgresql://", "postgres://", "pgx://", "pgx5://"} {
		if strings.HasPrefix(dsn, prefix) {
			rest := dsn[len(prefix):]
			return "pgx5://" + rest
		}
	}

	// key=value form — parse it into URL components.
	params := parseKVDSN(dsn)
	host := params["host"]
	if host == "" {
		host = "localhost"
	}
	port := params["port"]
	if port == "" {
		port = "5432"
	}
	user := params["user"]
	password := params["password"]
	dbname := params["dbname"]
	sslmode := params["sslmode"]
	if sslmode == "" {
		sslmode = "disable"
	}

	var userInfo *url.Userinfo
	if user != "" {
		if password != "" {
			userInfo = url.UserPassword(user, password)
		} else {
			userInfo = url.User(user)
		}
	}

	u := url.URL{
		Scheme:   "pgx5",
		User:     userInfo,
		Host:     host + ":" + port,
		Path:     "/" + dbname,
		RawQuery: url.Values{"sslmode": {sslmode}}.Encode(),
	}
	return u.String()
}

// parseKVDSN parses a libpq-style DSN string of space-separated `key=value` fields into a map.
// It splits on whitespace, ignores fields that do not contain an `=`, and removes surrounding single quotes from values.
func parseKVDSN(dsn string) map[string]string {
	result := make(map[string]string)
	fields := strings.Fields(dsn)
	for _, field := range fields {
		idx := strings.IndexByte(field, '=')
		if idx < 0 {
			continue
		}
		key := field[:idx]
		val := field[idx+1:]
		// Remove surrounding single quotes if present
		if len(val) >= 2 && val[0] == '\'' && val[len(val)-1] == '\'' {
			val = val[1 : len(val)-1]
		}
		result[key] = val
	}
	return result
}
