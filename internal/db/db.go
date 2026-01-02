package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/crazyuploader/rdctl-bot/internal/config"
)

// Init initializes the database connection using either PostgreSQL or SQLite based on config.
// It configures logging, connection pool settings, runs migrations, and returns the *gorm.DB.
func Init(dsn string) (*gorm.DB, error) {
	cfg := config.Get()
	isDebug := cfg != nil && cfg.App.LogLevel == "debug"

	// Configure custom logger
	logLevel := logger.Warn
	paramQueries := true
	if isDebug {
		logLevel = logger.Info
		paramQueries = false
	}

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
			ParameterizedQueries:      paramQueries,
		},
	)

	var db *gorm.DB
	var err error

	// Check if we should use SQLite or PostgreSQL
	if cfg != nil && cfg.Database.IsSQLite() {
		// Use SQLite
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: newLogger,
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SQLite database: %w", err)
		}

		// SQLite specific connection settings
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get SQLite database instance: %w", err)
		}

		// Verify connection is working
		if err := sqlDB.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
		}

		// SQLite connection pool settings (simpler than PostgreSQL)
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		// Use PostgreSQL
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: newLogger,
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL database: %w", err)
		}

		// Get underlying SQL DB to configure connection pool
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get PostgreSQL database instance: %w", err)
		}

		// Verify connection is working
		if err := sqlDB.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
		}

		// Set connection pool settings
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	// Run migrations with proper ordering
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database connected and migrations completed successfully!")
	return db, nil
}

// runMigrations performs automatic schema migrations in dependency order (parent tables first).
func runMigrations(db *gorm.DB) error {
	log.Println("Starting database migrations...")

	// Step 1: Migrate User table first (no dependencies)
	log.Println("Migrating users table...")
	if err := db.AutoMigrate(&User{}); err != nil {
		return fmt.Errorf("failed to migrate users table: %w", err)
	}

	// Step 2: Migrate tables that depend on User
	log.Println("Migrating activity_logs table...")
	if err := db.AutoMigrate(&ActivityLog{}); err != nil {
		return fmt.Errorf("failed to migrate activity_logs table: %w", err)
	}

	log.Println("Migrating torrent_activities table...")
	if err := db.AutoMigrate(&TorrentActivity{}); err != nil {
		return fmt.Errorf("failed to migrate torrent_activities table: %w", err)
	}

	log.Println("Migrating command_logs table...")
	if err := db.AutoMigrate(&CommandLog{}); err != nil {
		return fmt.Errorf("failed to migrate command_logs table: %w", err)
	}

	// Step 3: Migrate DownloadActivity last (depends on both User and TorrentActivity)
	log.Println("Migrating download_activities table...")
	if err := db.AutoMigrate(&DownloadActivity{}); err != nil {
		return fmt.Errorf("failed to migrate download_activities table: %w", err)
	}

	log.Println("All migrations completed successfully!")
	return nil
}

// Close closes the underlying database connection. Returns nil if db is nil.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
