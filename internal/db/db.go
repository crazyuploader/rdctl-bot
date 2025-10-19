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
// It opens a PostgreSQL connection with a custom GORM logger and UTC timestamps, applies connection pool settings (max idle 10, max open 100, conn max lifetime 1h), runs AutoMigrate for the application's models, assigns the resulting *gorm.DB to the package-level DB, and returns it. An error is returned if connecting, obtaining the underlying sql.DB, or running migrations fails.
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

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	DB = db
	log.Println("Database connected and migrations completed successfully!")
	return db, nil
}

// runMigrations performs automatic schema migrations for the application's models.
// It migrates the User, ActivityLog, TorrentActivity, DownloadActivity and CommandLog
// models and returns any error encountered during migration.
func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&ActivityLog{},
		&TorrentActivity{},
		&DownloadActivity{},
		&CommandLog{},
	)
}

// Close closes the underlying database connection initialized by Init.
// If the package DB is nil, Close does nothing and returns nil.
// Any error encountered while retrieving the underlying sql.DB or while closing it is returned.
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
