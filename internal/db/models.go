package db

import (
	"time"

	"gorm.io/gorm"
)

// User represents a Telegram user in the system.
// It stores user information and activity tracking data.
type User struct {
	ID            uint           `gorm:"primaryKey"`           // Unique identifier for the user in the database
	UserID        int64          `gorm:"uniqueIndex;not null"` // Telegram user ID (unique identifier from Telegram)
	Username      string         `gorm:"index"`                // Telegram username (may be empty)
	FirstName     string         // User's first name
	LastName      string         // User's last name
	IsSuperAdmin  bool           `gorm:"default:false"` // Indicates if the user has super admin privileges
	IsAllowed     bool           `gorm:"default:false"` // Legacy field - not used anymore, kept for migration compatibility
	FirstSeenAt   time.Time      `gorm:"not null"`      // Timestamp of when the user was first seen by the bot
	LastSeenAt    time.Time      `gorm:"not null"`      // Timestamp of when the user was last active
	TotalCommands int            `gorm:"default:0"`     // Count of commands executed by the user
	CreatedAt     time.Time      // When the user record was created
	UpdatedAt     time.Time      // When the user record was last updated
	DeletedAt     gorm.DeletedAt `gorm:"index"` // Soft delete timestamp
}

// ActivityType represents the type of activity performed by a user.
// It's used to categorize different actions in the activity log.
type ActivityType string

const (
	ActivityTypeTorrentAdd         ActivityType = "torrent_add"         // User added a torrent
	ActivityTypeTorrentDelete      ActivityType = "torrent_delete"      // User deleted a torrent
	ActivityTypeTorrentInfo        ActivityType = "torrent_info"        // User requested torrent information
	ActivityTypeTorrentList        ActivityType = "torrent_list"        // User listed torrents
	ActivityTypeDownloadUnrestrict ActivityType = "download_unrestrict" // User unrestricted a download
	ActivityTypeDownloadList       ActivityType = "download_list"       // User listed downloads
	ActivityTypeDownloadDelete     ActivityType = "download_delete"     // User deleted a download
	ActivityTypeCommandStart       ActivityType = "command_start"       // User started the bot
	ActivityTypeCommandHelp        ActivityType = "command_help"        // User requested help
	ActivityTypeCommandStatus      ActivityType = "command_status"      // User checked status
	ActivityTypeMagnetLink         ActivityType = "magnet_link"         // User used a magnet link
	ActivityTypeHosterLink         ActivityType = "hoster_link"         // User used a hoster link
	ActivityTypeUnauthorized       ActivityType = "unauthorized"        // Unauthorized access attempt
	ActivityTypeError              ActivityType = "error"               // An error occurred
)

// ActivityLog tracks general user activities in the system.
// It records what actions users perform and their outcomes.
type ActivityLog struct {
	ID              uint         `gorm:"primaryKey"`     // Unique identifier for the activity log entry
	UserID          uint         `gorm:"index;not null"` // ID of the user who performed the activity
	ChatID          int64        `gorm:"index;not null"` // Telegram chat ID where the activity occurred
	Username        string       `gorm:"index"`          // Username of the user (may be empty)
	ActivityType    ActivityType `gorm:"index;not null"` // Type of activity performed
	Command         string       `gorm:"index"`          // Command that was executed (if applicable)
	MessageThreadID int          // ID of the message thread (if in a thread)
	Success         bool         `gorm:"default:true"` // Whether the activity was successful
	ErrorMessage    string       `gorm:"type:text"`    // Error message if the activity failed
	Metadata        string       `gorm:"type:jsonb"`   // Additional data stored as JSON
	CreatedAt       time.Time    `gorm:"index"`        // When the activity was recorded

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"` // Reference to the User who performed the activity
}

// TorrentActivity tracks torrent-specific activities performed by users.
// It records detailed information about torrent operations.
type TorrentActivity struct {
	ID            uint      `gorm:"primaryKey"`     // Unique identifier for the torrent activity
	UserID        uint      `gorm:"index;not null"` // ID of the user who performed the activity
	ChatID        int64     `gorm:"index;not null"` // Telegram chat ID where the activity occurred
	TorrentID     string    `gorm:"index;not null"` // Unique identifier for the torrent
	TorrentHash   string    `gorm:"index"`          // Hash of the torrent
	TorrentName   string    // Name of the torrent
	MagnetLink    string    `gorm:"type:text"`      // Magnet link for the torrent
	Action        string    `gorm:"index;not null"` // Action performed (add, delete, info, select_files)
	Status        string    // Current status of the torrent operation
	FileSize      int64     // Size of the torrent in bytes
	Progress      float64   // Progress of the torrent operation (0.0 to 1.0)
	Success       bool      `gorm:"default:true"`                     // Whether the operation was successful
	ErrorMessage  string    `gorm:"type:text"`                        // Error message if the operation failed
	Metadata      string    `gorm:"type:jsonb;default:'{}'"`          // Additional data stored as JSON
	CreatedAt     time.Time `gorm:"index"`                            // When the activity was recorded
	SelectedFiles string    `gorm:"type:jsonb;not null;default:'[]'"` // Selected files stored as JSON array

	// Relationships
	User               User               `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"` // Reference to the User
	DownloadActivities []DownloadActivity `gorm:"foreignKey:TorrentActivityID"`                  // Related download activities
}

// DownloadActivity tracks download/unrestrict activities.
// It records detailed information about file downloads and unrestrict operations.
type DownloadActivity struct {
	ID                uint      `gorm:"primaryKey"`     // Unique identifier for the download activity
	UserID            uint      `gorm:"index;not null"` // ID of the user who performed the activity
	ChatID            int64     `gorm:"index;not null"` // Telegram chat ID where the activity occurred
	DownloadID        string    `gorm:"index"`          // Unique identifier for the download
	OriginalLink      string    `gorm:"type:text"`      // Original link that was downloaded or unrestricted
	FileName          string    // Name of the downloaded file
	FileSize          int64     // Size of the file in bytes
	Host              string    `gorm:"index"`          // Host service where the file is stored
	Action            string    `gorm:"index;not null"` // Action performed (unrestrict, list, delete)
	Success           bool      `gorm:"default:true"`   // Whether the operation was successful
	ErrorMessage      string    `gorm:"type:text"`      // Error message if the operation failed
	Metadata          string    `gorm:"type:jsonb"`     // Additional data stored as JSON
	CreatedAt         time.Time `gorm:"index"`          // When the activity was recorded
	TorrentActivityID *uint     `gorm:"index"`          // Optional reference to a related TorrentActivity

	// Relationships
	User            User             `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`             // Reference to the User
	TorrentActivity *TorrentActivity `gorm:"foreignKey:TorrentActivityID;constraint:OnDelete:SET NULL"` // Optional reference to TorrentActivity
}

// CommandLog tracks all command executions in the system.
// It records detailed information about commands users run.
type CommandLog struct {
	ID              uint      `gorm:"primaryKey"`      // Unique identifier for the command log entry
	UserID          uint      `gorm:"index;not null"`  // ID of the user who executed the command
	ChatID          int64     `gorm:"index;not null"`  // Telegram chat ID where the command was executed
	Username        string    `gorm:"index"`           // Username of the user (may be empty)
	Command         string    `gorm:"index; not null"` // The command that was executed
	FullCommand     string    `gorm:"type:text"`       // The full command with arguments
	MessageThreadID int       // ID of the message thread (if in a thread)
	ExecutionTime   int64     // Execution time in milliseconds
	Success         bool      `gorm:"default:true"` // Whether the command was successful
	ErrorMessage    string    `gorm:"type:text"`    // Error message if the command failed
	ResponseLength  int       // Length of the response in bytes
	CreatedAt       time.Time `gorm:"index"` // When the command was executed

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"` // Reference to the User
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
