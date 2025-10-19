package db

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&User{}, &ActivityLog{}, &TorrentActivity{}, &DownloadActivity{}, &CommandLog{})
	require.NoError(t, err)

	return db
}

func TestNewUserRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestUserRepository_GetOrCreateUser_NewUser(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	chatID := int64(123456)
	username := "testuser"
	firstName := "Test"
	lastName := "User"
	isSuperAdmin := true
	isAllowed := true

	user, err := repo.GetOrCreateUser(chatID, username, firstName, lastName, isSuperAdmin, isAllowed)
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.Equal(t, chatID, user.ChatID)
	assert.Equal(t, username, user.Username)
	assert.Equal(t, firstName, user.FirstName)
	assert.Equal(t, lastName, user.LastName)
	assert.True(t, user.IsSuperAdmin)
	assert.True(t, user.IsAllowed)
	assert.NotZero(t, user.FirstSeenAt)
	assert.NotZero(t, user.LastSeenAt)
	assert.Equal(t, 0, user.TotalCommands)
}

func TestUserRepository_GetOrCreateUser_ExistingUser(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	chatID := int64(123456)

	// Create user first time
	user1, err := repo.GetOrCreateUser(chatID, "user1", "First1", "Last1", false, true)
	require.NoError(t, err)
	firstSeenAt := user1.FirstSeenAt
	time.Sleep(10 * time.Millisecond) // Small delay to ensure time difference

	// Get same user with updated info
	user2, err := repo.GetOrCreateUser(chatID, "user2_updated", "First2", "Last2", false, true)
	require.NoError(t, err)

	// ID should be the same
	assert.Equal(t, user1.ID, user2.ID)
	// Updated fields
	assert.Equal(t, "user2_updated", user2.Username)
	assert.Equal(t, "First2", user2.FirstName)
	assert.Equal(t, "Last2", user2.LastName)
	// FirstSeenAt should not change
	assert.Equal(t, firstSeenAt.Unix(), user2.FirstSeenAt.Unix())
	// LastSeenAt should be updated
	assert.True(t, user2.LastSeenAt.After(firstSeenAt) || user2.LastSeenAt.Equal(firstSeenAt))
}

func TestUserRepository_GetOrCreateUser_EmptyUsername(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	user, err := repo.GetOrCreateUser(123456, "", "First", "Last", false, true)
	require.NoError(t, err)
	assert.Equal(t, "", user.Username)
	assert.Equal(t, "First", user.FirstName)
}

func TestNewActivityRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewActivityRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestActivityRepository_LogActivity_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewActivityRepository(db)

	// Create a user first
	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	metadata := map[string]interface{}{
		"torrent_id": "test123",
		"count":      42,
	}

	err = repo.LogActivity(
		user.ID,
		123456,
		"testuser",
		ActivityTypeTorrentAdd,
		"/add",
		5,
		true,
		"",
		metadata,
	)
	require.NoError(t, err)

	// Verify activity was logged
	var activity ActivityLog
	err = db.First(&activity).Error
	require.NoError(t, err)
	assert.Equal(t, user.ID, activity.UserID)
	assert.Equal(t, ActivityTypeTorrentAdd, activity.ActivityType)
	assert.True(t, activity.Success)

	// Verify metadata
	var metaMap map[string]interface{}
	err = json.Unmarshal([]byte(activity.Metadata), &metaMap)
	require.NoError(t, err)
	assert.Equal(t, "test123", metaMap["torrent_id"])
}

func TestActivityRepository_LogActivity_WithError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewActivityRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	err = repo.LogActivity(
		user.ID,
		123456,
		"testuser",
		ActivityTypeError,
		"/fail",
		0,
		false,
		"Something went wrong",
		nil,
	)
	require.NoError(t, err)

	var activity ActivityLog
	err = db.First(&activity).Error
	require.NoError(t, err)
	assert.False(t, activity.Success)
	assert.Equal(t, "Something went wrong", activity.ErrorMessage)
}

func TestActivityRepository_LogActivity_InvalidMetadata(t *testing.T) {
	db := setupTestDB(t)
	repo := NewActivityRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	// Metadata with invalid JSON content (channels can't be marshaled)
	metadata := map[string]interface{}{
		"invalid": make(chan int),
	}

	// Should still succeed with empty JSON object
	err = repo.LogActivity(user.ID, 123456, "testuser", ActivityTypeTorrentAdd, "/add", 0, true, "", metadata)
	require.NoError(t, err)

	var activity ActivityLog
	err = db.First(&activity).Error
	require.NoError(t, err)
	assert.Equal(t, "{}", activity.Metadata)
}

func TestNewTorrentRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTorrentRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestTorrentRepository_LogTorrentActivity_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTorrentRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	metadata := map[string]interface{}{
		"seeders": 100,
	}

	err = repo.LogTorrentActivity(
		user.ID,
		123456,
		"torrent123",
		"hash123",
		"Test Torrent",
		"magnet:?xt=urn:btih:test",
		"add",
		"downloading",
		1024*1024*100, // 100 MB
		75.5,
		true,
		"",
		metadata,
	)
	require.NoError(t, err)

	var activity TorrentActivity
	err = db.First(&activity).Error
	require.NoError(t, err)
	assert.Equal(t, "torrent123", activity.TorrentID)
	assert.Equal(t, "hash123", activity.TorrentHash)
	assert.Equal(t, int64(1024*1024*100), activity.FileSize)
	assert.Equal(t, 75.5, activity.Progress)
	assert.True(t, activity.Success)
}

func TestTorrentRepository_GetTorrentActivities_NoFilter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTorrentRepository(db)

	userRepo := NewUserRepository(db)
	user1, _ := userRepo.GetOrCreateUser(111, "user1", "U", "1", false, true)
	user2, _ := userRepo.GetOrCreateUser(222, "user2", "U", "2", false, true)

	// Log multiple activities
	repo.LogTorrentActivity(user1.ID, 111, "t1", "", "", "", "add", "", 0, 0, true, "", nil)
	repo.LogTorrentActivity(user2.ID, 222, "t2", "", "", "", "add", "", 0, 0, true, "", nil)
	repo.LogTorrentActivity(user1.ID, 111, "t3", "", "", "", "info", "", 0, 0, true, "", nil)

	activities, err := repo.GetTorrentActivities(0, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(activities))
}

func TestTorrentRepository_GetTorrentActivities_WithUserFilter(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTorrentRepository(db)

	userRepo := NewUserRepository(db)
	user1, _ := userRepo.GetOrCreateUser(111, "user1", "U", "1", false, true)
	user2, _ := userRepo.GetOrCreateUser(222, "user2", "U", "2", false, true)

	// Log activities for different users
	repo.LogTorrentActivity(user1.ID, 111, "t1", "", "", "", "add", "", 0, 0, true, "", nil)
	repo.LogTorrentActivity(user2.ID, 222, "t2", "", "", "", "add", "", 0, 0, true, "", nil)
	repo.LogTorrentActivity(user1.ID, 111, "t3", "", "", "", "info", "", 0, 0, true, "", nil)

	activities, err := repo.GetTorrentActivities(user1.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, len(activities))
	for _, a := range activities {
		assert.Equal(t, user1.ID, a.UserID)
	}
}

func TestTorrentRepository_GetTorrentActivities_WithLimit(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTorrentRepository(db)

	userRepo := NewUserRepository(db)
	user, _ := userRepo.GetOrCreateUser(111, "user1", "U", "1", false, true)

	// Log multiple activities
	for i := 0; i < 10; i++ {
		repo.LogTorrentActivity(user.ID, 111, "t"+string(rune(i)), "", "", "", "add", "", 0, 0, true, "", nil)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	activities, err := repo.GetTorrentActivities(0, 5)
	require.NoError(t, err)
	assert.Equal(t, 5, len(activities))
}

func TestTorrentRepository_GetTorrentActivities_OrderedByCreatedAt(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTorrentRepository(db)

	userRepo := NewUserRepository(db)
	user, _ := userRepo.GetOrCreateUser(111, "user1", "U", "1", false, true)

	// Log activities with delays
	repo.LogTorrentActivity(user.ID, 111, "t1", "", "", "", "add", "", 0, 0, true, "", nil)
	time.Sleep(10 * time.Millisecond)
	repo.LogTorrentActivity(user.ID, 111, "t2", "", "", "", "add", "", 0, 0, true, "", nil)
	time.Sleep(10 * time.Millisecond)
	repo.LogTorrentActivity(user.ID, 111, "t3", "", "", "", "add", "", 0, 0, true, "", nil)

	activities, err := repo.GetTorrentActivities(0, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(activities))
	
	// Should be ordered DESC by created_at (newest first)
	assert.Equal(t, "t3", activities[0].TorrentID)
	assert.Equal(t, "t2", activities[1].TorrentID)
	assert.Equal(t, "t1", activities[2].TorrentID)
}

func TestNewDownloadRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDownloadRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestDownloadRepository_LogDownloadActivity_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDownloadRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	metadata := map[string]interface{}{
		"quality": "HD",
	}

	err = repo.LogDownloadActivity(
		user.ID,
		123456,
		"download123",
		"https://example.com/file.zip",
		"file.zip",
		"example.com",
		"unrestrict",
		1024*1024*50, // 50 MB
		true,
		"",
		metadata,
	)
	require.NoError(t, err)

	var activity DownloadActivity
	err = db.First(&activity).Error
	require.NoError(t, err)
	assert.Equal(t, "download123", activity.DownloadID)
	assert.Equal(t, "example.com", activity.Host)
	assert.Equal(t, int64(1024*1024*50), activity.FileSize)
	assert.True(t, activity.Success)
}

func TestNewCommandRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommandRepository(db)

	assert.NotNil(t, repo)
	assert.Equal(t, db, repo.db)
}

func TestCommandRepository_LogCommand_Success(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommandRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	err = repo.LogCommand(
		user.ID,
		123456,
		"testuser",
		"start",
		"/start",
		0,
		250,
		true,
		"",
		100,
	)
	require.NoError(t, err)

	// Verify command log
	var cmdLog CommandLog
	err = db.First(&cmdLog).Error
	require.NoError(t, err)
	assert.Equal(t, "start", cmdLog.Command)
	assert.Equal(t, int64(250), cmdLog.ExecutionTime)
	assert.Equal(t, 100, cmdLog.ResponseLength)
	assert.True(t, cmdLog.Success)

	// Verify user's total_commands was incremented
	var updatedUser User
	err = db.First(&updatedUser, user.ID).Error
	require.NoError(t, err)
	assert.Equal(t, 1, updatedUser.TotalCommands)
}

func TestCommandRepository_LogCommand_IncrementsTotalCommands(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommandRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	// Log multiple commands
	for i := 0; i < 5; i++ {
		err = repo.LogCommand(user.ID, 123456, "testuser", "test", "/test", 0, 100, true, "", 10)
		require.NoError(t, err)
	}

	// Verify total_commands
	var updatedUser User
	err = db.First(&updatedUser, user.ID).Error
	require.NoError(t, err)
	assert.Equal(t, 5, updatedUser.TotalCommands)
}

func TestCommandRepository_LogCommand_WithError(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommandRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	err = repo.LogCommand(user.ID, 123456, "testuser", "fail", "/fail", 0, 100, false, "Command failed", 0)
	require.NoError(t, err)

	var cmdLog CommandLog
	err = db.First(&cmdLog).Error
	require.NoError(t, err)
	assert.False(t, cmdLog.Success)
	assert.Equal(t, "Command failed", cmdLog.ErrorMessage)

	// total_commands should still be incremented even on failure
	var updatedUser User
	err = db.First(&updatedUser, user.ID).Error
	require.NoError(t, err)
	assert.Equal(t, 1, updatedUser.TotalCommands)
}

func TestCommandRepository_GetUserStats_Success(t *testing.T) {
	db := setupTestDB(t)
	commandRepo := NewCommandRepository(db)
	activityRepo := NewActivityRepository(db)
	torrentRepo := NewTorrentRepository(db)
	downloadRepo := NewDownloadRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	// Log some commands
	commandRepo.LogCommand(user.ID, 123456, "testuser", "start", "/start", 0, 100, true, "", 10)
	commandRepo.LogCommand(user.ID, 123456, "testuser", "help", "/help", 0, 100, true, "", 10)

	// Log activities
	activityRepo.LogActivity(user.ID, 123456, "testuser", ActivityTypeTorrentAdd, "/add", 0, true, "", nil)
	activityRepo.LogActivity(user.ID, 123456, "testuser", ActivityTypeTorrentList, "/list", 0, true, "", nil)

	// Log torrent activities
	torrentRepo.LogTorrentActivity(user.ID, 123456, "t1", "", "", "", "add", "", 0, 0, true, "", nil)
	torrentRepo.LogTorrentActivity(user.ID, 123456, "t2", "", "", "", "add", "", 0, 0, true, "", nil)

	// Log download activities
	downloadRepo.LogDownloadActivity(user.ID, 123456, "d1", "", "", "", "unrestrict", 0, true, "", nil)

	stats, err := commandRepo.GetUserStats(user.ID)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats["total_commands"])
	assert.Equal(t, int64(2), stats["total_activities"])
	assert.Equal(t, int64(2), stats["total_torrents"])
	assert.Equal(t, int64(1), stats["total_downloads"])
	assert.NotNil(t, stats["first_seen_at"])
	assert.NotNil(t, stats["last_seen_at"])
}

func TestCommandRepository_GetUserStats_UserNotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommandRepository(db)

	stats, err := repo.GetUserStats(999999)
	require.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "user not found")
}

func TestCommandRepository_GetUserStats_NoActivities(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommandRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	stats, err := repo.GetUserStats(user.ID)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats["total_commands"])
	assert.Equal(t, int64(0), stats["total_activities"])
	assert.Equal(t, int64(0), stats["total_torrents"])
	assert.Equal(t, int64(0), stats["total_downloads"])
}

func TestRepositories_ConcurrentAccess(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCommandRepository(db)

	userRepo := NewUserRepository(db)
	user, err := userRepo.GetOrCreateUser(123456, "testuser", "Test", "User", false, true)
	require.NoError(t, err)

	// Simulate concurrent command logging
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			err := repo.LogCommand(user.ID, 123456, "testuser", "test", "/test", 0, 100, true, "", 10)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify total_commands
	var updatedUser User
	err = db.First(&updatedUser, user.ID).Error
	require.NoError(t, err)
	assert.Equal(t, 10, updatedUser.TotalCommands)
}

func TestRepositories_EdgeCases(t *testing.T) {
	t.Run("LogActivity with empty metadata", func(t *testing.T) {
		db := setupTestDB(t)
		repo := NewActivityRepository(db)
		userRepo := NewUserRepository(db)
		user, _ := userRepo.GetOrCreateUser(123, "u", "F", "L", false, true)

		err := repo.LogActivity(user.ID, 123, "u", ActivityTypeTorrentAdd, "/add", 0, true, "", nil)
		require.NoError(t, err)

		var activity ActivityLog
		db.First(&activity)
		assert.Equal(t, "{}", activity.Metadata)
	})

	t.Run("LogTorrentActivity with very long metadata", func(t *testing.T) {
		db := setupTestDB(t)
		repo := NewTorrentRepository(db)
		userRepo := NewUserRepository(db)
		user, _ := userRepo.GetOrCreateUser(123, "u", "F", "L", false, true)

		largeMetadata := map[string]interface{}{
			"files": make([]string, 100),
		}

		err := repo.LogTorrentActivity(user.ID, 123, "t1", "", "", "", "add", "", 0, 0, true, "", largeMetadata)
		require.NoError(t, err)
	})

	t.Run("GetOrCreateUser with negative chat ID", func(t *testing.T) {
		db := setupTestDB(t)
		repo := NewUserRepository(db)

		user, err := repo.GetOrCreateUser(-123456, "user", "F", "L", false, true)
		require.NoError(t, err)
		assert.Equal(t, int64(-123456), user.ChatID)
	})

	t.Run("LogCommand with zero execution time", func(t *testing.T) {
		db := setupTestDB(t)
		repo := NewCommandRepository(db)
		userRepo := NewUserRepository(db)
		user, _ := userRepo.GetOrCreateUser(123, "u", "F", "L", false, true)

		err := repo.LogCommand(user.ID, 123, "u", "fast", "/fast", 0, 0, true, "", 0)
		require.NoError(t, err)

		var cmdLog CommandLog
		db.First(&cmdLog)
		assert.Equal(t, int64(0), cmdLog.ExecutionTime)
	})
}