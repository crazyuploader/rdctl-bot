package db

import "time"

// User is the public-facing user type returned by repositories.
// Field types are chosen to match the old GORM model API so that
// callers (handlers.go, autodelete.go, etc.) compile without changes.
type User struct {
	ID            int64
	UserID        int64
	Username      string
	FirstName     string
	LastName      string
	IsSuperAdmin  bool
	IsAllowed     bool
	FirstSeenAt   time.Time
	LastSeenAt    time.Time
	TotalCommands int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Chat is the public-facing chat type returned by repositories.
type Chat struct {
	ID        int64
	ChatID    int64
	Title     string
	Type      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Setting is the public-facing setting type returned by repositories.
type Setting struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

// SettingAudit is the public-facing audit record type returned by repositories.
type SettingAudit struct {
	ID        int64
	Key       string
	OldValue  string
	NewValue  string
	ChangedBy int64
	ChatID    *int64
	ChangedAt time.Time
}

// TorrentActivity is the public-facing torrent activity type.
type TorrentActivity struct {
	ID            int64
	RequestID     string
	UserID        int64
	ChatID        int64
	TorrentID     string
	TorrentHash   string
	TorrentName   string
	MagnetLink    string
	Action        string
	Status        string
	FileSize      int64
	Progress      float64
	Success       bool
	ErrorMessage  string
	Metadata      string
	CreatedAt     time.Time
	SelectedFiles string
}

// KeptTorrentUser holds the minimal user info embedded in a KeptTorrent record.
type KeptTorrentUser struct {
	ID        int64
	UserID    int64
	Username  string
	FirstName string
	LastName  string
}

// KeptTorrent is the public-facing kept-torrent type, matching the old GORM
// model which preloaded the User association.
type KeptTorrent struct {
	ID        int64
	TorrentID string
	Filename  string
	KeptByID  int64
	KeptAt    time.Time
	User      KeptTorrentUser
}

// derefStr returns the string value of a *string, or "" if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
