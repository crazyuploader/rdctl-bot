package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init initializes the global database handle using the provided DSN, configures the connection pool, and runs automatic migrations.
// Init initializes the package-level GORM database connection using the provided PostgreSQL DSN.
// It configures a custom GORM logger, forces UTC for timestamps, applies connection pool settings
// (max idle connections 10, max open connections 100, connection max lifetime 1 hour), runs automatic
// schema migrations for the application's models, assigns the configured *gorm.DB to the package-level
// DB variable, and returns it.
// The dsn parameter is the PostgreSQL connection string.
// The function returns the configured *gorm.DB on success, or an error if connecting, obtaining the
// underlying sql.DB, or running migrations fails.
func Init(dsn string) (*gorm.DB, error) {
	// Configure custom logger
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Run migrations with proper ordering
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	DB = db
	log.Println("Database connected and migrations completed successfully!")
	return db, nil
}

// runMigrations performs automatic schema migrations for the application's models.
// It migrates the User, ActivityLog, TorrentActivity, DownloadActivity and CommandLog
// It returns any error encountered while migrating the User, ActivityLog, TorrentActivity, DownloadActivity, and CommandLog models.
// CRITICAL: Tables must be migrated in dependency order (parent tables before children)
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

// Close closes the underlying database connection held in the package-level DB.
// If DB is nil, Close does nothing and returns nil. Any error encountered while
// obtaining the underlying *sql.DB or while closing it is returned.
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
