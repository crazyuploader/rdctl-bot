package bot

import (
	"context"
	"testing"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockRDClient is a mock of the Real-Debrid client
type MockRDClient struct {
	mock.Mock
}

func (m *MockRDClient) GetTorrents(limit, offset int) ([]realdebrid.Torrent, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]realdebrid.Torrent), args.Error(1)
}

func (m *MockRDClient) DeleteTorrent(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

// Implement remaining interface methods with no-ops or panics if called
func (m *MockRDClient) GetTorrentsWithCount(limit, offset int) (*realdebrid.TorrentsResult, error) {
	return nil, nil
}
func (m *MockRDClient) GetTorrentInfo(torrentID string) (*realdebrid.Torrent, error) { return nil, nil }
func (m *MockRDClient) AddMagnet(magnetURL string) (*realdebrid.AddMagnetResponse, error) {
	return nil, nil
}
func (m *MockRDClient) SelectFiles(torrentID string, fileIDs []int) error { return nil }
func (m *MockRDClient) SelectAllFiles(torrentID string) error             { return nil }
func (m *MockRDClient) CheckInstantAvailability(hashes []string) (realdebrid.InstantAvailability, error) {
	return nil, nil
}
func (m *MockRDClient) GetUser() (*realdebrid.User, error) { return nil, nil }
func (m *MockRDClient) GetDownloads(limit, offset int) ([]realdebrid.Download, error) {
	return nil, nil
}
func (m *MockRDClient) GetDownloadsWithCount(limit, offset int) (*realdebrid.DownloadsResult, error) {
	return nil, nil
}
func (m *MockRDClient) UnrestrictLink(link string) (*realdebrid.UnrestrictedLink, error) {
	return nil, nil
}
func (m *MockRDClient) DeleteDownload(downloadID string) error { return nil }
func (m *MockRDClient) GetSupportedRegex() ([]string, error)   { return nil, nil }

func setupTestBot(t *testing.T) (*Bot, *MockRDClient, *gorm.DB) {
	database, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := database.AutoMigrate(&db.User{}, &db.Setting{}, &db.KeptTorrent{}, &db.KeptTorrentAction{}, &db.TorrentActivity{}, &db.Chat{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	cfg := &config.Config{
		App: config.AppConfig{
			AutoDeleteDays: 30,
		},
	}

	mockRD := new(MockRDClient)

	b := &Bot{
		config:       cfg,
		db:           database,
		rdClient:     mockRD,
		userRepo:     db.NewUserRepository(database),
		settingRepo:  db.NewSettingRepository(database),
		keptRepo:     db.NewKeptTorrentRepository(database),
		torrentRepo:  db.NewTorrentRepository(database),
		systemUserID: 1,
	}

	return b, mockRD, database
}

func TestRunAutoDeleteCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("Deletes old torrents and skips kept ones", func(t *testing.T) {
		b, mockRD, database := setupTestBot(t)

		// Create a system user
		database.Create(&db.User{UserID: 0, ID: 1, Username: "system", FirstSeenAt: time.Now(), LastSeenAt: time.Now()})
		// Create a normal user
		database.Create(&db.User{UserID: 123, ID: 2, Username: "user123", FirstSeenAt: time.Now(), LastSeenAt: time.Now()})

		now := time.Now().UTC()
		oldDate := now.AddDate(0, 0, -40) // 40 days ago
		newDate := now.AddDate(0, 0, -10) // 10 days ago

		torrents := []realdebrid.Torrent{
			{ID: "old1", Filename: "old_file.mp4", Added: oldDate, Bytes: 100},
			{ID: "new1", Filename: "new_file.mp4", Added: newDate, Bytes: 200},
			{ID: "kept1", Filename: "kept_file.mp4", Added: oldDate, Bytes: 300},
		}

		// Mark one as kept
		err := b.keptRepo.KeepTorrent(ctx, "kept1", "kept_file.mp4", 123, 0)
		assert.NoError(t, err)

		mockRD.On("GetTorrents", 100, 0).Return(torrents, nil)
		mockRD.On("DeleteTorrent", "old1").Return(nil)

		b.runAutoDeleteCheck(ctx)

		mockRD.AssertExpectations(t)
		mockRD.AssertNotCalled(t, "DeleteTorrent", "new1")
		mockRD.AssertNotCalled(t, "DeleteTorrent", "kept1")
	})
}
