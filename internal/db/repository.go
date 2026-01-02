package db

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository handles user operations
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a UserRepository using the provided gorm.DB.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetOrCreateUser gets or creates a user based on their Telegram user ID
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

// ActivityRepository handles activity logging
type ActivityRepository struct {
	db *gorm.DB
}

// NewActivityRepository returns a new ActivityRepository using the provided GORM DB handle.
func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// LogActivity logs a general activity
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

// TorrentRepository handles torrent activity logging
type TorrentRepository struct {
	db *gorm.DB
}

// NewTorrentRepository creates and returns a TorrentRepository backed by the provided gorm.DB.
func NewTorrentRepository(db *gorm.DB) *TorrentRepository {
	return &TorrentRepository{db: db}
}

// LogTorrentActivity logs torrent-specific activity
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

// GetTorrentActivities retrieves torrent activities with filters
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

// DownloadRepository handles download activity logging
type DownloadRepository struct {
	db *gorm.DB
}

// NewDownloadRepository returns a DownloadRepository configured with the provided GORM database handle.
//
// NewDownloadRepository creates a DownloadRepository that uses the provided *gorm.DB for persistence.
func NewDownloadRepository(db *gorm.DB) *DownloadRepository {
	return &DownloadRepository{db: db}
}

// LogDownloadActivity logs download/unrestrict activity with optional torrent association
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

// CommandRepository handles command logging
type CommandRepository struct {
	db *gorm.DB
}

// NewCommandRepository creates a new CommandRepository backed by the provided GORM DB handle.
func NewCommandRepository(db *gorm.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

// LogCommand logs command execution and atomically increments the user's total_commands counter.
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

// GetUserStats retrieves user statistics
func (r *CommandRepository) GetUserStats(userID uint) (map[string]interface{}, error) {
	var user User
	err := r.db.First(&user, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("user not found") // Or a more specific error type
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
