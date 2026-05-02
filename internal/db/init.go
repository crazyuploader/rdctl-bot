package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Init runs migrations and returns a connection pool.
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

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping: %w", err)
	}
	log.Println("Database connected and migrations completed successfully!")
	return pool, nil
}

// Close shuts down the connection pool.
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

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
// converted to a URL first.
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

	// golang-migrate DSN: pgx5://user:password@host:port/dbname?sslmode=...
	authority := host + ":" + port
	if user != "" || password != "" {
		if password != "" {
			authority = user + ":" + password + "@" + authority
		} else {
			authority = user + "@" + authority
		}
	}
	return fmt.Sprintf("pgx5://%s/%s?sslmode=%s", authority, dbname, sslmode)
}

// parseKVDSN parses a libpq-style "key=value" DSN into a map.
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
