package db

import (
	"time"

	"gorm.io/gorm"
)

// User represents a Telegram user
type User struct {
	ID            uint   `gorm:"primaryKey"`
	UserID        int64  `gorm:"uniqueIndex;not null"`
	Username      string `gorm:"index"`
	FirstName     string
	LastName      string
	IsSuperAdmin  bool      `gorm:"default:false"`
	FirstSeenAt   time.Time `gorm:"not null"`
	LastSeenAt    time.Time `gorm:"not null"`
	TotalCommands int       `gorm:"default:0"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`

	// Relationships
	ActivityLogs       []ActivityLog      `gorm:"foreignKey:UserID"`
	TorrentActivities  []TorrentActivity  `gorm:"foreignKey:UserID"`
	DownloadActivities []DownloadActivity `gorm:"foreignKey:UserID"`
	CommandLogs        []CommandLog       `gorm:"foreignKey:UserID"`
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
	ActivityTypeUnauthorized       ActivityType = "unauthorized"
	ActivityTypeError              ActivityType = "error"
)

// ActivityLog is the main activity logging table
type ActivityLog struct {
	ID              uint         `gorm:"primaryKey"`
	UserID          uint         `gorm:"index;not null"`
	ChatID          int64        `gorm:"index;not null"`
	Username        string       `gorm:"index"`
	ActivityType    ActivityType `gorm:"index;not null"`
	Command         string       `gorm:"index"`
	MessageThreadID int
	Success         bool      `gorm:"default:true"`
	ErrorMessage    string    `gorm:"type:text"`
	Metadata        string    `gorm:"type:jsonb"` // Store additional data as JSON
	CreatedAt       time.Time `gorm:"index"`

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

// TorrentActivity tracks torrent-specific activities
type TorrentActivity struct {
	ID            uint   `gorm:"primaryKey"`
	UserID        uint   `gorm:"index;not null"`
	ChatID        int64  `gorm:"index;not null"`
	TorrentID     string `gorm:"index;not null"`
	TorrentHash   string `gorm:"index"`
	TorrentName   string
	MagnetLink    string `gorm:"type:text"`
	Action        string `gorm:"index;not null"` // add, delete, info, select_files
	Status        string
	FileSize      int64
	Progress      float64
	Success       bool      `gorm:"default:true"`
	ErrorMessage  string    `gorm:"type:text"`
	Metadata      string    `gorm:"type:jsonb;default:'{}'"`
	CreatedAt     time.Time `gorm:"index"`
	SelectedFiles string    `gorm:"type:jsonb;not null;default:'[]'"` // Stores selected files as JSON array

	// Relationships
	User               User               `gorm:"foreignKey:UserID"`
	DownloadActivities []DownloadActivity `gorm:"foreignKey:TorrentActivityID"`
}

// DownloadActivity tracks download/unrestrict activities
type DownloadActivity struct {
	ID                uint   `gorm:"primaryKey"`
	UserID            uint   `gorm:"index;not null"`
	ChatID            int64  `gorm:"index;not null"`
	DownloadID        string `gorm:"index"`
	OriginalLink      string `gorm:"type:text"`
	FileName          string
	FileSize          int64
	Host              string    `gorm:"index"`
	Action            string    `gorm:"index;not null"` // unrestrict, list, delete
	Success           bool      `gorm:"default:true"`
	ErrorMessage      string    `gorm:"type:text"`
	Metadata          string    `gorm:"type:jsonb"`
	CreatedAt         time.Time `gorm:"index"`
	TorrentActivityID *uint     `gorm:"index"` // Links to originating torrent activity

	// Relationships
	User            User             `gorm:"foreignKey:UserID"`
	TorrentActivity *TorrentActivity `gorm:"foreignKey:TorrentActivityID"`
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
	User User `gorm:"foreignKey:UserID"`
}

// TableName overrides
func (User) TableName() string {
	return "users"
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
