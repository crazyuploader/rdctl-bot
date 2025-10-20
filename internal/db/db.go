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

// DB is the package-level database handle that will be initialized by the Init function.
// It provides access to the database connection throughout the application.
var DB *gorm.DB

// Init initializes the database connection and performs schema migrations.
// This function establishes a connection to a PostgreSQL database using the provided DSN,
// configures the connection pool, sets up logging, and runs database migrations in the correct order.
// It returns the configured *gorm.DB instance and any error encountered during the process.
func Init(dsn string) (*gorm.DB, error) {
	// Configure custom logger for GORM
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond, // Log queries slower than this threshold
			LogLevel:                  logger.Info,            // Log level (Info, Warn, Error, Silent)
			IgnoreRecordNotFoundError: true,                   // Ignore "record not found" errors
			Colorful:                  true,                   // Enable colored output
		},
	)

	// Open a database connection with the provided DSN
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC() // Force UTC for all timestamps
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB to configure connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)           // Maximum number of connections in the idle connection pool
	sqlDB.SetMaxOpenConns(100)          // Maximum number of open connections to the database
	sqlDB.SetConnMaxLifetime(time.Hour) // Maximum amount of time a connection may be reused

	// Run migrations with proper ordering
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Assign the configured database handle to the package-level variable
	DB = db
	log.Println("Database connected and migrations completed successfully!")
	return db, nil
}

// runMigrations performs automatic schema migrations for all application models.
// This function migrates database tables in the correct order based on their dependencies.
// It ensures that parent tables are migrated before child tables that reference them.
// The function returns any error encountered during the migration process.
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

// Close closes the underlying database connection.
// This function safely closes the database connection held in the package-level DB variable.
// If DB is nil, Close does nothing and returns nil. Any error encountered while obtaining
// the underlying *sql.DB or while closing it is returned.
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
