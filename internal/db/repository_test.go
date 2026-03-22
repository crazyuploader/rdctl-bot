package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate all models
	err = db.AutoMigrate(
		&User{},
		&Chat{},
		&ActivityLog{},
		&TorrentActivity{},
		&DownloadActivity{},
		&CommandLog{},
		&Setting{},
		&KeptTorrent{},
		&KeptTorrentAction{},
		&SettingAudit{},
	)
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func TestKeptTorrentRepository(t *testing.T) {
	database := setupTestDB(t)
	userRepo := NewUserRepository(database)
	keptRepo := NewKeptTorrentRepository(database)
	ctx := context.Background()

	// Create a test user
	user, err := userRepo.GetOrCreateUser(ctx, 123, "testuser", "Test", "User", false)
	assert.NoError(t, err)
	assert.NotNil(t, user)

	t.Run("KeepTorrent successful", func(t *testing.T) {
		err := keptRepo.KeepTorrent(ctx, "torrent1", "file1.mp4", 123, 0)
		assert.NoError(t, err)

		isKept, err := keptRepo.IsKept(ctx, "torrent1")
		assert.NoError(t, err)
		assert.True(t, isKept)

		count, err := keptRepo.CountKeptByUser(ctx, 123)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("KeepTorrent limit enforcement", func(t *testing.T) {
		// Already has 1 (torrent1)
		err := keptRepo.KeepTorrent(ctx, "torrent2", "file2.mp4", 123, 2)
		assert.NoError(t, err)

		// Third one should fail if limit is 2
		err = keptRepo.KeepTorrent(ctx, "torrent3", "file3.mp4", 123, 2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "maximum kept torrent limit (2) reached")

		count, err := keptRepo.CountKeptByUser(ctx, 123)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("KeepTorrent user not found", func(t *testing.T) {
		err := keptRepo.KeepTorrent(ctx, "torrent4", "file4.mp4", 999, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "actor user 999 not found")
	})

	t.Run("UnkeepTorrent successful", func(t *testing.T) {
		err := keptRepo.UnkeepTorrent(ctx, "torrent1", 123)
		assert.NoError(t, err)

		isKept, err := keptRepo.IsKept(ctx, "torrent1")
		assert.NoError(t, err)
		assert.False(t, isKept)

		count, err := keptRepo.CountKeptByUser(ctx, 123)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count) // Only torrent2 remains
	})

	t.Run("GetKeptTorrentIDs", func(t *testing.T) {
		ids, err := keptRepo.GetKeptTorrentIDs(ctx)
		assert.NoError(t, err)
		assert.True(t, ids["torrent2"])
		assert.False(t, ids["torrent1"])
	})
}
