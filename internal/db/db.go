package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
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

	// Use PostgreSQL
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   newLogger,
		NowFunc:                                  func() time.Time { return time.Now().UTC() },
		DisableForeignKeyConstraintWhenMigrating: true,
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

	// Step 1: Migrate Chat and User tables first (no dependencies)
	log.Println("Migrating chats table...")
	if err := db.AutoMigrate(&Chat{}); err != nil {
		return fmt.Errorf("failed to migrate chats table: %w", err)
	}

	log.Println("Migrating users table...")
	if err := db.AutoMigrate(&User{}); err != nil {
		return fmt.Errorf("failed to migrate users table: %w", err)
	}

	// Seed legacy chat IDs to satisfy foreign key constraints before migrating dependent tables
	if err := seedLegacyChats(db); err != nil {
		return fmt.Errorf("failed to seed legacy chats: %w", err)
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

	// Step 4: Migrate Settings table (no dependencies)
	log.Println("Migrating settings table...")
	if err := db.AutoMigrate(&Setting{}); err != nil {
		return fmt.Errorf("failed to migrate settings table: %w", err)
	}

	// Step 5: Migrate KeptTorrent table (no dependencies)
	log.Println("Migrating kept_torrents table...")
	if !db.Migrator().HasTable(&KeptTorrent{}) {
		if err := db.AutoMigrate(&KeptTorrent{}); err != nil {
			return fmt.Errorf("failed to migrate kept_torrents table: %w", err)
		}
	} else {
		log.Println("kept_torrents table already exists, skipping...")
	}

	// Step 6: Migrate Action/Audit tables
	log.Println("Migrating kept_torrent_actions table...")
	if !db.Migrator().HasTable(&KeptTorrentAction{}) {
		if err := db.AutoMigrate(&KeptTorrentAction{}); err != nil {
			return fmt.Errorf("failed to migrate kept_torrent_actions table: %w", err)
		}
	} else {
		log.Println("kept_torrent_actions table already exists, skipping...")
	}

	log.Println("Migrating setting_audits table...")
	if !db.Migrator().HasTable(&SettingAudit{}) {
		if err := db.AutoMigrate(&SettingAudit{}); err != nil {
			return fmt.Errorf("failed to migrate setting_audits table: %w", err)
		}
	} else {
		log.Println("setting_audits table already exists, skipping...")
	}

	log.Println("All migrations completed successfully!")

	// Create foreign key constraints manually (GORM AutoMigrate has bugs with FK direction)
	if err := createForeignKeys(db); err != nil {
		return fmt.Errorf("failed to create foreign key constraints: %w", err)
	}

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

// createForeignKeys manually creates foreign key constraints that GORM AutoMigrate gets wrong.
func createForeignKeys(db *gorm.DB) error {
	log.Println("Creating foreign key constraints...")

	foreignKeys := []struct {
		table      string
		column     string
		references string
		onDelete   string
	}{
		{"activity_logs", "user_id", "users(id)", "CASCADE"},
		{"activity_logs", "chat_id", "chats(chat_id)", "CASCADE"},
		{"torrent_activities", "user_id", "users(id)", "CASCADE"},
		{"torrent_activities", "chat_id", "chats(chat_id)", "CASCADE"},
		{"command_logs", "user_id", "users(id)", "CASCADE"},
		{"command_logs", "chat_id", "chats(chat_id)", "CASCADE"},
		{"download_activities", "user_id", "users(id)", "CASCADE"},
		{"download_activities", "chat_id", "chats(chat_id)", "CASCADE"},
		{"download_activities", "torrent_activity_id", "torrent_activities(id)", "SET NULL"},
		{"kept_torrents", "kept_by_id", "users(id)", "CASCADE"},
		{"kept_torrent_actions", "user_id", "users(id)", "CASCADE"},
		{"setting_audits", "changed_by", "users(user_id)", "CASCADE"},
		{"setting_audits", "chat_id", "chats(chat_id)", "SET NULL"},
	}

	for _, fk := range foreignKeys {
		constraintName := fmt.Sprintf("fk_%s_%s", fk.table, fk.column)

		// Check if constraint already exists
		var count int64
		db.Raw(`
			SELECT COUNT(*) FROM information_schema.table_constraints
			WHERE constraint_name = ? AND table_schema = 'public'
		`, constraintName).Scan(&count)

		if count > 0 {
			log.Printf("Constraint %s already exists, skipping...", constraintName)
			continue
		}

		// Create the foreign key
		sql := fmt.Sprintf(
			`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s ON DELETE %s`,
			fk.table, constraintName, fk.column, fk.references, fk.onDelete,
		)

		if err := db.Exec(sql).Error; err != nil {
			log.Printf("Warning: failed to create constraint %s: %v", constraintName, err)
			continue
		}
		log.Printf("Created constraint: %s", constraintName)
	}

	log.Println("Foreign key constraints created successfully!")
	return nil
}

// seedLegacyChats extracts all distinct chat_ids from existing logs and creates dummy Chat entries
// so that when GORM creates the foreign key constraints, it does not fail on orphaned data.
func seedLegacyChats(db *gorm.DB) error {
	log.Println("Seeding legacy chat IDs to satisfy foreign key constraints...")

	tables := []string{"activity_logs", "command_logs", "torrent_activities", "download_activities", "setting_audits"}
	uniqueChatIDs := map[int64]bool{0: true}

	for _, table := range tables {
		if db.Migrator().HasTable(table) {
			// Backfill legacy NULL chat_ids to the synthetic system chat (0)
			if err := db.Table(table).Where("chat_id IS NULL").Update("chat_id", 0).Error; err != nil {
				return fmt.Errorf("failed to backfill NULL chat IDs in %s: %w", table, err)
			}

			var ids []int64
			if err := db.Table(table).Where("chat_id != 0").Distinct("chat_id").Pluck("chat_id", &ids).Error; err != nil {
				return fmt.Errorf("failed to fetch distinct chat IDs from %s: %w", table, err)
			}
			for _, id := range ids {
				uniqueChatIDs[id] = true
			}
		}
	}

	now := time.Now().UTC()
	for chatID := range uniqueChatIDs {
		chat := Chat{
			ChatID:    chatID,
			Title:     fmt.Sprintf("Legacy Chat %d", chatID),
			Type:      "unknown",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.Where("chat_id = ?", chatID).FirstOrCreate(&chat).Error; err != nil {
			return fmt.Errorf("failed to seed legacy chat %d: %w", chatID, err)
		}
	}

	return nil
}
