package db

import (
	"context"
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
func (r *UserRepository) GetOrCreateUser(ctx context.Context, userID int64, username, firstName, lastName string, isSuperAdmin bool) (*User, error) {
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
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
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
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&updatedUser).Error; err != nil {
		return nil, err
	}

	return &updatedUser, nil
}

// ChatRepository handles chat operations
type ChatRepository struct {
	db *gorm.DB
}

// NewChatRepository creates a ChatRepository using the provided gorm.DB.
func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

// GetOrCreateChat gets or creates a chat based on its Telegram chat ID
func (r *ChatRepository) GetOrCreateChat(ctx context.Context, chatID int64, title, chatType string) (*Chat, error) {
	now := time.Now().UTC()
	chat := Chat{
		ChatID:    chatID,
		Title:     title,
		Type:      chatType,
		CreatedAt: now,
		UpdatedAt: now,
	}

	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"title":      title,
			"type":       chatType,
			"updated_at": now,
		}),
	}).Create(&chat)

	if result.Error != nil {
		return nil, result.Error
	}

	var updatedChat Chat
	if err := r.db.WithContext(ctx).Where("chat_id = ?", chatID).First(&updatedChat).Error; err != nil {
		return nil, err
	}

	return &updatedChat, nil
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
func (r *ActivityRepository) LogActivity(ctx context.Context, requestID string, userID uint, chatID int64, username string, activityType ActivityType, command string, messageThreadID int, success bool, errorMsg string, metadata map[string]interface{}) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	activity := ActivityLog{
		RequestID:       requestID,
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

	return r.db.WithContext(ctx).Create(&activity).Error
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
func (r *TorrentRepository) LogTorrentActivity(ctx context.Context, requestID string, userID uint, chatID int64, torrentID, torrentHash, torrentName, magnetLink, action, status string, fileSize int64, progress float64, success bool, errorMsg string, metadata map[string]interface{}) error {
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
		RequestID:     requestID,
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

	return r.db.WithContext(ctx).Create(&activity).Error
}

// GetTorrentActivities retrieves torrent activities with filters
func (r *TorrentRepository) GetTorrentActivities(ctx context.Context, userID uint, limit int) ([]TorrentActivity, error) {
	var activities []TorrentActivity
	query := r.db.WithContext(ctx).Order("created_at DESC")

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

// NewDownloadRepository creates a DownloadRepository backed by the provided gorm.DB.
func NewDownloadRepository(db *gorm.DB) *DownloadRepository {
	return &DownloadRepository{db: db}
}

// LogDownloadActivity logs download/unrestrict activity with optional torrent association
func (r *DownloadRepository) LogDownloadActivity(ctx context.Context, requestID string, userID uint, chatID int64, downloadID, originalLink, fileName, host, action string, fileSize int64, success bool, errorMsg string, metadata map[string]interface{}, torrentActivityID *uint) error {
	// Ensure metadata is never nil
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	activity := DownloadActivity{
		RequestID:         requestID,
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

	return r.db.WithContext(ctx).Create(&activity).Error
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
func (r *CommandRepository) LogCommand(ctx context.Context, userID uint, chatID int64, username, command, fullCommand string, messageThreadID int, executionTime int64, success bool, errorMsg string, responseLength int) error {
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

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
func (r *CommandRepository) GetUserStats(ctx context.Context, userID uint) (map[string]interface{}, error) {
	var user User
	err := r.db.WithContext(ctx).First(&user, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	var totalActivities int64
	if res := r.db.WithContext(ctx).Model(&ActivityLog{}).Where("user_id = ?", userID).Count(&totalActivities); res.Error != nil {
		return nil, res.Error
	}

	var totalTorrents int64
	if res := r.db.WithContext(ctx).Model(&TorrentActivity{}).Where("user_id = ? AND action = ?", userID, "add").Count(&totalTorrents); res.Error != nil {
		return nil, res.Error
	}

	var totalDownloads int64
	if res := r.db.WithContext(ctx).Model(&DownloadActivity{}).Where("user_id = ? AND action = ?", userID, "unrestrict").Count(&totalDownloads); res.Error != nil {
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

// SettingRepository handles runtime configuration settings
type SettingRepository struct {
	db *gorm.DB
}

// NewSettingRepository creates a new SettingRepository backed by the provided GORM DB handle.
func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

// GetSetting retrieves a setting value by key. Returns empty string if not found.
func (r *SettingRepository) GetSetting(ctx context.Context, key string) (string, error) {
	var setting Setting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// SetSetting creates or updates a setting value by key.
func (r *SettingRepository) SetSetting(ctx context.Context, key, value string) error {
	now := time.Now().UTC()
	setting := Setting{
		Key:       key,
		Value:     value,
		UpdatedAt: now,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"value":      value,
			"updated_at": now,
		}),
	}).Create(&setting).Error
}

// SetSettingWithAudit creates or updates a setting and logs the change
func (r *SettingRepository) SetSettingWithAudit(ctx context.Context, key, value string, changedBy int64, chatID int64) error {
	oldValue, _ := r.GetSetting(ctx, key)

	now := time.Now().UTC()
	setting := Setting{
		Key:       key,
		Value:     value,
		UpdatedAt: now,
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"value":      value,
				"updated_at": now,
			}),
		}).Create(&setting).Error; err != nil {
			return err
		}

		audit := SettingAudit{
			Key:       key,
			OldValue:  oldValue,
			NewValue:  value,
			ChangedBy: changedBy,
			ChatID:    chatID,
			ChangedAt: now,
		}
		return tx.Create(&audit).Error
	})
}

// GetSettingHistory retrieves the audit history for a setting key
func (r *SettingRepository) GetSettingHistory(ctx context.Context, key string, limit int) ([]SettingAudit, error) {
	var audits []SettingAudit
	query := r.db.WithContext(ctx).Where("key = ?", key).Order("changed_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&audits).Error
	return audits, err
}

// KeptTorrentRepository handles kept torrent operations
type KeptTorrentRepository struct {
	db *gorm.DB
}

// NewKeptTorrentRepository creates a new KeptTorrentRepository backed by the provided gorm.DB.
func NewKeptTorrentRepository(db *gorm.DB) *KeptTorrentRepository {
	return &KeptTorrentRepository{db: db}
}

// KeepTorrent marks a torrent as kept (excluded from auto-delete)
func (r *KeptTorrentRepository) KeepTorrent(ctx context.Context, torrentID, filename string, keptByID int64) error {
	now := time.Now().UTC()
	keptTorrent := KeptTorrent{
		TorrentID: torrentID,
		Filename:  filename,
		KeptByID:  keptByID,
		KeptAt:    now,
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "torrent_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"filename":   filename,
				"kept_by_id": keptByID,
				"kept_at":    now,
			}),
		}).Create(&keptTorrent).Error; err != nil {
			return err
		}

		var user User
		username := ""
		if err := tx.Where("user_id = ?", keptByID).First(&user).Error; err == nil {
			username = user.Username
		}

		action := KeptTorrentAction{
			TorrentID: torrentID,
			Action:    "keep",
			UserID:    user.ID,
			Username:  username,
			CreatedAt: now,
		}
		return tx.Create(&action).Error
	})
}

// UnkeepTorrent removes the keep mark from a torrent
func (r *KeptTorrentRepository) UnkeepTorrent(ctx context.Context, torrentID string, unkeptByID int64) error {
	now := time.Now().UTC()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("torrent_id = ?", torrentID).Delete(&KeptTorrent{}).Error; err != nil {
			return err
		}

		var user User
		username := ""
		if err := tx.Where("user_id = ?", unkeptByID).First(&user).Error; err == nil {
			username = user.Username
		}

		action := KeptTorrentAction{
			TorrentID: torrentID,
			Action:    "unkeep",
			UserID:    user.ID,
			Username:  username,
			CreatedAt: now,
		}
		return tx.Create(&action).Error
	})
}

// IsKept checks if a torrent is marked as kept
func (r *KeptTorrentRepository) IsKept(ctx context.Context, torrentID string) (bool, error) {
	var keptTorrent KeptTorrent
	err := r.db.WithContext(ctx).Where("torrent_id = ?", torrentID).First(&keptTorrent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetKeptTorrentIDs returns a map of all kept torrent IDs for quick lookup
func (r *KeptTorrentRepository) GetKeptTorrentIDs(ctx context.Context) (map[string]bool, error) {
	var keptTorrents []KeptTorrent
	err := r.db.WithContext(ctx).Find(&keptTorrents).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, kt := range keptTorrents {
		result[kt.TorrentID] = true
	}
	return result, nil
}

// ListKeptTorrents returns all kept torrents with user details
func (r *KeptTorrentRepository) ListKeptTorrents(ctx context.Context) ([]KeptTorrent, error) {
	var results []KeptTorrent
	err := r.db.WithContext(ctx).
		Preload("User").
		Order("kept_at DESC").
		Find(&results).Error
	return results, err
}

// CountKeptByUser returns the number of torrents kept by a specific user
func (r *KeptTorrentRepository) CountKeptByUser(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&KeptTorrent{}).Where("kept_by_id = ?", userID).Count(&count).Error
	return count, err
}
