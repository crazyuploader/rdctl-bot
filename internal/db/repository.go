package db

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository handles all database operations related to User entities.
// It provides methods for creating, retrieving, and updating user information.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository instance with the provided database connection.
// It initializes the repository with the given *gorm.DB instance for all subsequent operations.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetOrCreateUser retrieves an existing user by their Telegram user ID or creates a new user record if one doesn't exist.
// This function performs an upsert operation, updating existing user records with new information while preserving their ID.
// It returns the updated user record and any error encountered during the operation.
func (r *UserRepository) GetOrCreateUser(userID int64, username, firstName, lastName string, isSuperAdmin bool) (*User, error) {
	now := time.Now().UTC()
	user := User{
		UserID:       userID,
		Username:     username,
		FirstName:    firstName,
		LastName:     lastName,
		IsSuperAdmin: isSuperAdmin,
		FirstSeenAt:  now,
		LastSeenAt:   now,
	}

	// Use clause.OnConflict to perform an upsert based on user_id
	result := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"username":       username,
			"first_name":     firstName,
			"last_name":      lastName,
			"is_super_admin": isSuperAdmin,
			"last_seen_at":   now,
		}),
	}).Create(&user)

	if result.Error != nil {
		return nil, result.Error
	}

	// Retrieve the user to ensure all fields are current
	var updatedUser User
	if err := r.db.Where("user_id = ?", userID).First(&updatedUser).Error; err != nil {
		return nil, err
	}

	return &updatedUser, nil
}

// ActivityRepository handles all database operations related to activity logging.
// It provides methods for recording and retrieving user activities in the system.
type ActivityRepository struct {
	db *gorm.DB
}

// NewActivityRepository creates a new ActivityRepository instance with the provided database connection.
// It initializes the repository with the given *gorm.DB instance for all subsequent operations.
func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// LogActivity records a general activity performed by a user in the system.
// This function creates a new ActivityLog entry with the provided details.
// It converts the metadata map to JSON format before storing it in the database.
// The function returns any error encountered during the database operation.
func (r *ActivityRepository) LogActivity(userID uint, chatID int64, username string, activityType ActivityType, command string, messageThreadID int, success bool, errorMsg string, metadata map[string]interface{}) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	activity := ActivityLog{
		UserID:          userID,
		ChatID:          chatID,
		Username:        username,
		ActivityType:    activityType,
		Command:         command,
		MessageThreadID: messageThreadID,
		Success:         success,
		ErrorMessage:    errorMsg,
		Metadata:        string(metadataJSON),
		CreatedAt:       time.Now().UTC(),
	}

	return r.db.Create(&activity).Error
}

// TorrentRepository handles all database operations related to torrent activities.
// It provides methods for recording and retrieving torrent-specific operations.
type TorrentRepository struct {
	db *gorm.DB
}

// NewTorrentRepository creates a new TorrentRepository instance with the provided database connection.
// It initializes the repository with the given *gorm.DB instance for all subsequent operations.
func NewTorrentRepository(db *gorm.DB) *TorrentRepository {
	return &TorrentRepository{db: db}
}

// LogTorrentActivity records a torrent-specific activity performed by a user.
// This function creates a new TorrentActivity entry with the provided details.
// It ensures metadata is properly formatted as JSON and provides default values for optional fields.
// The function returns any error encountered during the database operation.
func (r *TorrentRepository) LogTorrentActivity(userID uint, chatID int64, torrentID, torrentHash, torrentName, magnetLink, action, status string, fileSize int64, progress float64, success bool, errorMsg string, metadata map[string]interface{}) error {
	// Ensure metadata is never nil
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	// Ensure selected_files has valid default JSON array
	selectedFiles := "[]"

	activity := TorrentActivity{
		UserID:        userID,
		ChatID:        chatID,
		TorrentID:     torrentID,
		TorrentHash:   torrentHash,
		TorrentName:   torrentName,
		MagnetLink:    magnetLink,
		Action:        action,
		Status:        status,
		FileSize:      fileSize,
		Progress:      progress,
		Success:       success,
		ErrorMessage:  errorMsg,
		Metadata:      string(metadataJSON),
		SelectedFiles: selectedFiles,
		CreatedAt:     time.Now().UTC(),
	}

	return r.db.Create(&activity).Error
}

// GetTorrentActivities retrieves torrent activities from the database with optional filtering.
// This function allows fetching torrent activities for a specific user or all users.
// It orders results by creation time in descending order and applies a limit if specified.
// The function returns the retrieved activities and any error encountered during the operation.
func (r *TorrentRepository) GetTorrentActivities(userID uint, limit int) ([]TorrentActivity, error) {
	var activities []TorrentActivity
	query := r.db.Order("created_at DESC")

	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&activities).Error
	return activities, err
}

// DownloadRepository handles all database operations related to download activities.
// It provides methods for recording and retrieving download/unrestrict operations.
type DownloadRepository struct {
	db *gorm.DB
}

// NewDownloadRepository creates a new DownloadRepository instance with the provided database connection.
// It initializes the repository with the given *gorm.DB instance for all subsequent operations.
func NewDownloadRepository(db *gorm.DB) *DownloadRepository {
	return &DownloadRepository{db: db}
}

// LogDownloadActivity records a download/unrestrict activity performed by a user.
// This function creates a new DownloadActivity entry with the provided details.
// It ensures metadata is properly formatted as JSON and handles optional torrent activity association.
// The function returns any error encountered during the database operation.
func (r *DownloadRepository) LogDownloadActivity(userID uint, chatID int64, downloadID, originalLink, fileName, host, action string, fileSize int64, success bool, errorMsg string, metadata map[string]interface{}, torrentActivityID *uint) error {
	// Ensure metadata is never nil
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	activity := DownloadActivity{
		UserID:            userID,
		ChatID:            chatID,
		DownloadID:        downloadID,
		OriginalLink:      originalLink,
		FileName:          fileName,
		FileSize:          fileSize,
		Host:              host,
		Action:            action,
		Success:           success,
		ErrorMessage:      errorMsg,
		Metadata:          string(metadataJSON),
		CreatedAt:         time.Now().UTC(),
		TorrentActivityID: torrentActivityID,
	}

	return r.db.Create(&activity).Error
}

// CommandRepository handles all database operations related to command logging.
// It provides methods for recording command executions and retrieving user statistics.
type CommandRepository struct {
	db *gorm.DB
}

// NewCommandRepository creates a new CommandRepository instance with the provided database connection.
// It initializes the repository with the given *gorm.DB instance for all subsequent operations.
func NewCommandRepository(db *gorm.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

// LogCommand logs a command execution and atomically increments the user's total_commands counter.
// This function creates a new CommandLog entry and updates the user's command count in a single transaction.
// It ensures data consistency by using a database transaction to perform both operations.
// The function returns any error encountered during the transaction.
func (r *CommandRepository) LogCommand(userID uint, chatID int64, username, command, fullCommand string, messageThreadID int, executionTime int64, success bool, errorMsg string, responseLength int) error {
	cmdLog := CommandLog{
		UserID:          userID,
		ChatID:          chatID,
		Username:        username,
		Command:         command,
		FullCommand:     fullCommand,
		MessageThreadID: messageThreadID,
		ExecutionTime:   executionTime,
		Success:         success,
		ErrorMessage:    errorMsg,
		ResponseLength:  responseLength,
		CreatedAt:       time.Now().UTC(),
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Create the command log within the transaction
		if err := tx.Create(&cmdLog).Error; err != nil {
			return err
		}

		// Atomically increment total_commands for the user
		if err := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("total_commands", gorm.Expr("total_commands + ?", 1)).Error; err != nil {
			return err
		}

		return nil
	})
}

// GetUserStats retrieves comprehensive statistics about a user's activities.
// This function gathers information about the user's command usage, torrent activities,
// and download operations from various tables in the database.
// It returns a map containing the collected statistics and any error encountered during the operation.
func (r *CommandRepository) GetUserStats(userID uint) (map[string]interface{}, error) {
	var user User
	err := r.db.First(&user, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	var totalActivities int64
	if res := r.db.Model(&ActivityLog{}).Where("user_id = ?", userID).Count(&totalActivities); res.Error != nil {
		return nil, res.Error
	}

	var totalTorrents int64
	if res := r.db.Model(&TorrentActivity{}).Where("user_id = ? AND action = ?", userID, "add").Count(&totalTorrents); res.Error != nil {
		return nil, res.Error
	}

	var totalDownloads int64
	if res := r.db.Model(&DownloadActivity{}).Where("user_id = ? AND action = ?", userID, "unrestrict").Count(&totalDownloads); res.Error != nil {
		return nil, res.Error
	}

	stats := map[string]interface{}{
		"total_commands":   user.TotalCommands,
		"total_activities": totalActivities,
		"total_torrents":   totalTorrents,
		"total_downloads":  totalDownloads,
		"first_seen_at":    user.FirstSeenAt,
		"last_seen_at":     user.LastSeenAt,
	}

	return stats, nil
}
