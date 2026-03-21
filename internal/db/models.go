package db

import (
	"time"

	"gorm.io/gorm"
)

// Chat represents a Telegram chat (private, group, supergroup, channel)
type Chat struct {
	ID        uint      `gorm:"primaryKey"`
	ChatID    int64     `gorm:"uniqueIndex;not null"`
	Title     string
	Type      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User represents a Telegram user
type User struct {
	ID            uint   `gorm:"primaryKey"`
	UserID        int64  `gorm:"uniqueIndex;not null"`
	Username      string `gorm:"index"`
	FirstName     string
	LastName      string
	IsSuperAdmin  bool      `gorm:"default:false"`
	IsAllowed     bool      `gorm:"default:false"` // Not used anymore, kept for migration compatibility
	FirstSeenAt   time.Time `gorm:"not null"`
	LastSeenAt    time.Time `gorm:"not null"`
	TotalCommands int       `gorm:"default:0"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
}

// ActivityType represents the type of activity
type ActivityType string

const (
	ActivityTypeTorrentAdd         ActivityType = "torrent_add"
	ActivityTypeTorrentDelete      ActivityType = "torrent_delete"
	ActivityTypeTorrentInfo        ActivityType = "torrent_info"
	ActivityTypeTorrentList        ActivityType = "torrent_list"
	ActivityTypeDownloadUnrestrict ActivityType = "download_unrestrict"
	ActivityTypeDownloadList       ActivityType = "download_list"
	ActivityTypeDownloadDelete     ActivityType = "download_delete"
	ActivityTypeCommandStart       ActivityType = "command_start"
	ActivityTypeCommandHelp        ActivityType = "command_help"
	ActivityTypeCommandStatus      ActivityType = "command_status"
	ActivityTypeMagnetLink         ActivityType = "magnet_link"
	ActivityTypeHosterLink         ActivityType = "hoster_link"
	ActivityTypeCommandDashboard   ActivityType = "command_dashboard"
	ActivityTypeTorrentKeep        ActivityType = "torrent_keep"
	ActivityTypeTorrentUnkeep      ActivityType = "torrent_unkeep"
	ActivityTypeUnauthorized       ActivityType = "unauthorized"
	ActivityTypeError              ActivityType = "error"
)

// ActivityLog is the main activity logging table
type ActivityLog struct {
	ID              uint         `gorm:"primaryKey"`
	RequestID       string       `gorm:"index"` // Correlation ID for request tracing
	UserID          uint         `gorm:"index:idx_user_created;not null"`
	ChatID          int64        `gorm:"index:idx_chat_created;not null"`
	Username        string       `gorm:"index"`
	ActivityType    ActivityType `gorm:"index;not null"`
	Command         string       `gorm:"index"`
	MessageThreadID int
	Success         bool      `gorm:"default:true"`
	ErrorMessage    string    `gorm:"type:text"`
	Metadata        string    `gorm:"type:json"` // Store additional data as JSON
	CreatedAt       time.Time `gorm:"index:idx_user_created;index:idx_chat_created;not null"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Chat Chat `gorm:"foreignKey:ChatID;references:ChatID;constraint:OnDelete:CASCADE"`
}

// TorrentActivity tracks torrent-specific activities
type TorrentActivity struct {
	ID            uint   `gorm:"primaryKey"`
	RequestID     string `gorm:"index"` // Correlation ID for request tracing
	UserID        uint   `gorm:"index:idx_torrent_user_action;not null"`
	ChatID        int64  `gorm:"index;not null"`
	TorrentID     string `gorm:"index;not null"`
	TorrentHash   string `gorm:"index"`
	TorrentName   string
	MagnetLink    string `gorm:"type:text"`
	Action        string `gorm:"index:idx_torrent_user_action;not null"` // add, delete, info, select_files
	Status        string
	FileSize      int64
	Progress      float64
	Success       bool      `gorm:"default:true"`
	ErrorMessage  string    `gorm:"type:text"`
	Metadata      string    `gorm:"type:json;default:'{}'"`
	CreatedAt     time.Time `gorm:"index;not null"`
	SelectedFiles string    `gorm:"type:json;not null;default:'[]'"` // Stores selected files as JSON array

	User               User               `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Chat               Chat               `gorm:"foreignKey:ChatID;references:ChatID;constraint:OnDelete:CASCADE"`
	DownloadActivities []DownloadActivity `gorm:"foreignKey:TorrentActivityID"`
}

// DownloadActivity tracks download/unrestrict activities
type DownloadActivity struct {
	ID                uint   `gorm:"primaryKey"`
	RequestID         string `gorm:"index"` // Correlation ID for request tracing
	UserID            uint   `gorm:"index:idx_download_user_action;not null"`
	ChatID            int64  `gorm:"index;not null"`
	DownloadID        string `gorm:"index"`
	OriginalLink      string `gorm:"type:text"`
	FileName          string
	FileSize          int64
	Host              string    `gorm:"index"`
	Action            string    `gorm:"index:idx_download_user_action;not null"` // unrestrict, list, delete
	Success           bool      `gorm:"default:true"`
	ErrorMessage      string    `gorm:"type:text"`
	Metadata          string    `gorm:"type:json"`
	CreatedAt         time.Time `gorm:"index;not null"`
	TorrentActivityID *uint     `gorm:"index"` // Links to originating torrent activity

	User            User             `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Chat            Chat             `gorm:"foreignKey:ChatID;references:ChatID;constraint:OnDelete:CASCADE"`
	TorrentActivity *TorrentActivity `gorm:"foreignKey:TorrentActivityID;constraint:OnDelete:SET NULL"`
}

// CommandLog tracks all command executions
type CommandLog struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"index;not null"`
	ChatID          int64  `gorm:"index;not null"`
	Username        string `gorm:"index"`
	Command         string `gorm:"index;not null"`
	FullCommand     string `gorm:"type:text"`
	MessageThreadID int
	ExecutionTime   int64  // milliseconds
	Success         bool   `gorm:"default:true"`
	ErrorMessage    string `gorm:"type:text"`
	ResponseLength  int
	CreatedAt       time.Time `gorm:"index"`

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Chat Chat `gorm:"foreignKey:ChatID;references:ChatID;constraint:OnDelete:CASCADE"`
}

// TableName overrides
func (User) TableName() string {
	return "users"
}

func (Chat) TableName() string {
	return "chats"
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}

func (TorrentActivity) TableName() string {
	return "torrent_activities"
}

func (DownloadActivity) TableName() string {
	return "download_activities"
}

func (CommandLog) TableName() string {
	return "command_logs"
}

// Setting stores key-value configuration that can be changed at runtime via Telegram
type Setting struct {
	Key       string    `gorm:"primaryKey"`
	Value     string    `gorm:"not null"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (Setting) TableName() string {
	return "settings"
}

// KeptTorrent represents a torrent that a user has marked to be kept (excluded from auto-delete)
type KeptTorrent struct {
	ID        uint   `gorm:"primaryKey"`
	TorrentID string `gorm:"uniqueIndex;not null"` // Real-Debrid torrent ID
	Filename  string
	KeptByID  int64     `gorm:"index;not null"` // Telegram user ID who kept it
	KeptAt    time.Time `gorm:"not null"`
	
	User User `gorm:"foreignKey:KeptByID;references:UserID;constraint:OnDelete:CASCADE"`
}

func (KeptTorrent) TableName() string {
	return "kept_torrents"
}

// KeptTorrentAction tracks keep/unkeep actions on torrents
type KeptTorrentAction struct {
	ID        uint      `gorm:"primaryKey"`
	TorrentID string    `gorm:"index;not null"`
	Action    string    `gorm:"index;not null"` // "keep" or "unkeep"
	UserID    uint      `gorm:"index;not null"`
	Username  string    `gorm:"index"`
	CreatedAt time.Time `gorm:"index;not null"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

func (KeptTorrentAction) TableName() string {
	return "kept_torrent_actions"
}

// SettingAudit tracks changes to settings
type SettingAudit struct {
	ID        uint   `gorm:"primaryKey"`
	Key       string `gorm:"index;not null"`
	OldValue  string
	NewValue  string
	ChangedBy int64     `gorm:"index;not null"` // User who made change
	ChatID    int64     `gorm:"index"`          // Chat where change was made
	ChangedAt time.Time `gorm:"index;not null"`

	User User `gorm:"foreignKey:ChangedBy;references:UserID;constraint:OnDelete:CASCADE"`
	Chat Chat `gorm:"foreignKey:ChatID;references:ChatID;constraint:OnDelete:SET NULL"`
}

func (SettingAudit) TableName() string {
	return "setting_audits"
}
