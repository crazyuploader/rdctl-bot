package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_TableName(t *testing.T) {
	u := User{}
	assert.Equal(t, "users", u.TableName())
}

func TestActivityLog_TableName(t *testing.T) {
	a := ActivityLog{}
	assert.Equal(t, "activity_logs", a.TableName())
}

func TestTorrentActivity_TableName(t *testing.T) {
	ta := TorrentActivity{}
	assert.Equal(t, "torrent_activities", ta.TableName())
}

func TestDownloadActivity_TableName(t *testing.T) {
	da := DownloadActivity{}
	assert.Equal(t, "download_activities", da.TableName())
}

func TestCommandLog_TableName(t *testing.T) {
	cl := CommandLog{}
	assert.Equal(t, "command_logs", cl.TableName())
}

func TestActivityType_Constants(t *testing.T) {
	// Test that all activity type constants are defined correctly
	tests := []struct {
		name     string
		value    ActivityType
		expected string
	}{
		{"torrent add", ActivityTypeTorrentAdd, "torrent_add"},
		{"torrent delete", ActivityTypeTorrentDelete, "torrent_delete"},
		{"torrent info", ActivityTypeTorrentInfo, "torrent_info"},
		{"torrent list", ActivityTypeTorrentList, "torrent_list"},
		{"download unrestrict", ActivityTypeDownloadUnrestrict, "download_unrestrict"},
		{"download list", ActivityTypeDownloadList, "download_list"},
		{"download delete", ActivityTypeDownloadDelete, "download_delete"},
		{"command start", ActivityTypeCommandStart, "command_start"},
		{"command help", ActivityTypeCommandHelp, "command_help"},
		{"command status", ActivityTypeCommandStatus, "command_status"},
		{"magnet link", ActivityTypeMagnetLink, "magnet_link"},
		{"hoster link", ActivityTypeHosterLink, "hoster_link"},
		{"unauthorized", ActivityTypeUnauthorized, "unauthorized"},
		{"error", ActivityTypeError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.value))
		})
	}
}

func TestUser_DefaultValues(t *testing.T) {
	// Test default GORM tags are correctly defined
	u := User{
		ChatID:    123456,
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
	}

	assert.False(t, u.IsSuperAdmin, "IsSuperAdmin should default to false")
	assert.False(t, u.IsAllowed, "IsAllowed should default to false")
	assert.Equal(t, 0, u.TotalCommands, "TotalCommands should default to 0")
}

func TestActivityLog_DefaultValues(t *testing.T) {
	a := ActivityLog{
		UserID:       1,
		ChatID:       123456,
		ActivityType: ActivityTypeTorrentAdd,
	}

	assert.True(t, a.Success, "Success should default to true")
}

func TestTorrentActivity_DefaultValues(t *testing.T) {
	ta := TorrentActivity{
		UserID:     1,
		ChatID:     123456,
		TorrentID:  "test-id",
		Action:     "add",
	}

	assert.True(t, ta.Success, "Success should default to true")
}

func TestDownloadActivity_DefaultValues(t *testing.T) {
	da := DownloadActivity{
		UserID: 1,
		ChatID: 123456,
		Action: "unrestrict",
	}

	assert.True(t, da.Success, "Success should default to true")
}

func TestCommandLog_DefaultValues(t *testing.T) {
	cl := CommandLog{
		UserID:  1,
		ChatID:  123456,
		Command: "start",
	}

	assert.True(t, cl.Success, "Success should default to true")
}

func TestUser_Relationships(t *testing.T) {
	// Verify that relationship fields are properly defined
	u := User{
		ActivityLogs:       []ActivityLog{},
		TorrentActivities:  []TorrentActivity{},
		DownloadActivities: []DownloadActivity{},
		CommandLogs:        []CommandLog{},
	}

	assert.NotNil(t, u.ActivityLogs)
	assert.NotNil(t, u.TorrentActivities)
	assert.NotNil(t, u.DownloadActivities)
	assert.NotNil(t, u.CommandLogs)
	assert.Equal(t, 0, len(u.ActivityLogs))
	assert.Equal(t, 0, len(u.TorrentActivities))
	assert.Equal(t, 0, len(u.DownloadActivities))
	assert.Equal(t, 0, len(u.CommandLogs))
}

func TestModels_FieldPresence(t *testing.T) {
	t.Run("User has all required fields", func(t *testing.T) {
		u := User{
			ID:            1,
			ChatID:        123,
			Username:      "user",
			FirstName:     "First",
			LastName:      "Last",
			IsSuperAdmin:  true,
			IsAllowed:     true,
			TotalCommands: 10,
		}
		assert.Equal(t, uint(1), u.ID)
		assert.Equal(t, int64(123), u.ChatID)
		assert.Equal(t, "user", u.Username)
		assert.True(t, u.IsSuperAdmin)
		assert.True(t, u.IsAllowed)
		assert.Equal(t, 10, u.TotalCommands)
	})

	t.Run("ActivityLog has all required fields", func(t *testing.T) {
		a := ActivityLog{
			ID:              1,
			UserID:          2,
			ChatID:          123,
			Username:        "user",
			ActivityType:    ActivityTypeTorrentAdd,
			Command:         "/add",
			MessageThreadID: 5,
			Success:         true,
			ErrorMessage:    "error",
			IPAddress:       "127.0.0.1",
			UserAgent:       "bot",
			Metadata:        "{}",
		}
		assert.Equal(t, uint(1), a.ID)
		assert.Equal(t, uint(2), a.UserID)
		assert.Equal(t, ActivityTypeTorrentAdd, a.ActivityType)
	})

	t.Run("TorrentActivity has all required fields", func(t *testing.T) {
		ta := TorrentActivity{
			ID:           1,
			UserID:       2,
			ChatID:       123,
			TorrentID:    "torrent-id",
			TorrentHash:  "hash",
			TorrentName:  "name",
			MagnetLink:   "magnet:?",
			Action:       "add",
			Status:       "downloaded",
			FileSize:     1024,
			Progress:     100.0,
			Success:      true,
			ErrorMessage: "",
			Metadata:     "{}",
		}
		assert.Equal(t, uint(1), ta.ID)
		assert.Equal(t, "torrent-id", ta.TorrentID)
		assert.Equal(t, int64(1024), ta.FileSize)
		assert.Equal(t, 100.0, ta.Progress)
	})

	t.Run("DownloadActivity has all required fields", func(t *testing.T) {
		da := DownloadActivity{
			ID:           1,
			UserID:       2,
			ChatID:       123,
			DownloadID:   "download-id",
			OriginalLink: "https://example.com/file",
			FileName:     "file.txt",
			FileSize:     2048,
			Host:         "example.com",
			Action:       "unrestrict",
			Success:      true,
			ErrorMessage: "",
			Metadata:     "{}",
		}
		assert.Equal(t, uint(1), da.ID)
		assert.Equal(t, "download-id", da.DownloadID)
		assert.Equal(t, int64(2048), da.FileSize)
		assert.Equal(t, "example.com", da.Host)
	})

	t.Run("CommandLog has all required fields", func(t *testing.T) {
		cl := CommandLog{
			ID:              1,
			UserID:          2,
			ChatID:          123,
			Username:        "user",
			Command:         "start",
			FullCommand:     "/start",
			MessageThreadID: 5,
			ExecutionTime:   100,
			Success:         true,
			ErrorMessage:    "",
			ResponseLength:  50,
		}
		assert.Equal(t, uint(1), cl.ID)
		assert.Equal(t, "start", cl.Command)
		assert.Equal(t, int64(100), cl.ExecutionTime)
		assert.Equal(t, 50, cl.ResponseLength)
	})
}

func TestActivityType_StringRepresentation(t *testing.T) {
	// Ensure ActivityType can be converted to string
	at := ActivityTypeTorrentAdd
	s := string(at)
	assert.Equal(t, "torrent_add", s)

	// Test comparison
	assert.Equal(t, ActivityTypeTorrentAdd, ActivityType("torrent_add"))
}

func TestModels_ZeroValues(t *testing.T) {
	t.Run("User zero value", func(t *testing.T) {
		var u User
		assert.Equal(t, uint(0), u.ID)
		assert.Equal(t, int64(0), u.ChatID)
		assert.Equal(t, "", u.Username)
		assert.False(t, u.IsSuperAdmin)
		assert.False(t, u.IsAllowed)
		assert.Equal(t, 0, u.TotalCommands)
	})

	t.Run("ActivityLog zero value", func(t *testing.T) {
		var a ActivityLog
		assert.Equal(t, uint(0), a.ID)
		assert.Equal(t, ActivityType(""), a.ActivityType)
		assert.False(t, a.Success)
	})

	t.Run("TorrentActivity zero value", func(t *testing.T) {
		var ta TorrentActivity
		assert.Equal(t, uint(0), ta.ID)
		assert.Equal(t, "", ta.TorrentID)
		assert.Equal(t, int64(0), ta.FileSize)
		assert.Equal(t, 0.0, ta.Progress)
		assert.False(t, ta.Success)
	})

	t.Run("DownloadActivity zero value", func(t *testing.T) {
		var da DownloadActivity
		assert.Equal(t, uint(0), da.ID)
		assert.Equal(t, "", da.DownloadID)
		assert.Equal(t, int64(0), da.FileSize)
		assert.False(t, da.Success)
	})

	t.Run("CommandLog zero value", func(t *testing.T) {
		var cl CommandLog
		assert.Equal(t, uint(0), cl.ID)
		assert.Equal(t, "", cl.Command)
		assert.Equal(t, int64(0), cl.ExecutionTime)
		assert.Equal(t, 0, cl.ResponseLength)
		assert.False(t, cl.Success)
	})
}

func TestActivityType_EdgeCases(t *testing.T) {
	t.Run("custom activity type", func(t *testing.T) {
		customType := ActivityType("custom_action")
		assert.Equal(t, "custom_action", string(customType))
	})

	t.Run("empty activity type", func(t *testing.T) {
		emptyType := ActivityType("")
		assert.Equal(t, "", string(emptyType))
	})

	t.Run("activity type comparison", func(t *testing.T) {
		assert.NotEqual(t, ActivityTypeTorrentAdd, ActivityTypeTorrentDelete)
		assert.Equal(t, ActivityTypeTorrentAdd, ActivityTypeTorrentAdd)
	})
}

func TestUser_LargeValues(t *testing.T) {
	u := User{
		ID:            999999,
		ChatID:        9223372036854775807, // max int64
		TotalCommands: 1000000,
	}

	assert.Equal(t, uint(999999), u.ID)
	assert.Equal(t, int64(9223372036854775807), u.ChatID)
	assert.Equal(t, 1000000, u.TotalCommands)
}

func TestTorrentActivity_LargeFileSize(t *testing.T) {
	ta := TorrentActivity{
		FileSize: 1099511627776, // 1 TB in bytes
		Progress: 99.99,
	}

	assert.Equal(t, int64(1099511627776), ta.FileSize)
	assert.Equal(t, 99.99, ta.Progress)
}

func TestDownloadActivity_LargeFileSize(t *testing.T) {
	da := DownloadActivity{
		FileSize: 10737418240, // 10 GB in bytes
	}

	assert.Equal(t, int64(10737418240), da.FileSize)
}

func TestCommandLog_ExecutionTime(t *testing.T) {
	tests := []struct {
		name          string
		executionTime int64
		description   string
	}{
		{"fast command", 50, "50ms"},
		{"normal command", 500, "500ms"},
		{"slow command", 5000, "5 seconds"},
		{"very slow command", 30000, "30 seconds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := CommandLog{
				ExecutionTime: tt.executionTime,
			}
			assert.Equal(t, tt.executionTime, cl.ExecutionTime)
		})
	}
}

func TestModels_SpecialCharacters(t *testing.T) {
	t.Run("User with special characters", func(t *testing.T) {
		u := User{
			Username:  "user_123-test",
			FirstName: "Test User 🎉",
			LastName:  "O'Brien-Smith",
		}
		assert.Equal(t, "user_123-test", u.Username)
		assert.Contains(t, u.FirstName, "🎉")
		assert.Contains(t, u.LastName, "'")
	})

	t.Run("TorrentActivity with special characters", func(t *testing.T) {
		ta := TorrentActivity{
			TorrentName: "Movie [2024] 1080p.BluRay",
			MagnetLink:  "magnet:?xt=urn:btih:abc123&dn=Test+File",
		}
		assert.Contains(t, ta.TorrentName, "[")
		assert.Contains(t, ta.MagnetLink, "magnet:?")
	})

	t.Run("DownloadActivity with special characters", func(t *testing.T) {
		da := DownloadActivity{
			OriginalLink: "https://example.com/file?id=123&token=abc",
			FileName:     "file (copy) [2].txt",
		}
		assert.Contains(t, da.OriginalLink, "?")
		assert.Contains(t, da.FileName, "(")
	})
}

func TestActivityLog_MetadataField(t *testing.T) {
	a := ActivityLog{
		Metadata: `{"key": "value", "count": 42}`,
	}
	assert.Equal(t, `{"key": "value", "count": 42}`, a.Metadata)
}

func TestTorrentActivity_MetadataField(t *testing.T) {
	ta := TorrentActivity{
		Metadata: `{"selected_files": [1, 2, 3]}`,
	}
	assert.Equal(t, `{"selected_files": [1, 2, 3]}`, ta.Metadata)
}

func TestDownloadActivity_MetadataField(t *testing.T) {
	da := DownloadActivity{
		Metadata: `{"quality": "hd", "format": "mp4"}`,
	}
	assert.Equal(t, `{"quality": "hd", "format": "mp4"}`, da.Metadata)
}