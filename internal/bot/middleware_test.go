package bot

import (
	"testing"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMiddleware(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	assert.NotNil(t, mw)
	assert.NotNil(t, mw.limiter)
	assert.Equal(t, cfg, mw.config)
}

func TestMiddleware_CheckAuthorization(t *testing.T) {
	tests := []struct {
		name              string
		chatID            int64
		allowedChatIDs    []int64
		superAdminIDs     []int64
		expectedAllowed   bool
		expectedSuperAdmin bool
	}{
		{
			name:              "allowed chat user",
			chatID:            123,
			allowedChatIDs:    []int64{123, 456},
			superAdminIDs:     []int64{789},
			expectedAllowed:   true,
			expectedSuperAdmin: false,
		},
		{
			name:              "super admin user",
			chatID:            789,
			allowedChatIDs:    []int64{123},
			superAdminIDs:     []int64{789},
			expectedAllowed:   true,
			expectedSuperAdmin: true,
		},
		{
			name:              "unauthorized user",
			chatID:            999,
			allowedChatIDs:    []int64{123},
			superAdminIDs:     []int64{789},
			expectedAllowed:   false,
			expectedSuperAdmin: false,
		},
		{
			name:              "super admin also in allowed list",
			chatID:            100,
			allowedChatIDs:    []int64{100, 200},
			superAdminIDs:     []int64{100},
			expectedAllowed:   true,
			expectedSuperAdmin: true,
		},
		{
			name:              "negative chat ID not allowed",
			chatID:            -123,
			allowedChatIDs:    []int64{123},
			superAdminIDs:     []int64{789},
			expectedAllowed:   false,
			expectedSuperAdmin: false,
		},
		{
			name:              "zero chat ID not allowed",
			chatID:            0,
			allowedChatIDs:    []int64{123},
			superAdminIDs:     []int64{789},
			expectedAllowed:   false,
			expectedSuperAdmin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Telegram: config.TelegramConfig{
					AllowedChatIDs: tt.allowedChatIDs,
					SuperAdminIDs:  tt.superAdminIDs,
				},
				App: config.AppConfig{
					RateLimit: config.RateLimitConfig{
						MessagesPerSecond: 10,
						Burst:             5,
					},
				},
			}

			mw := NewMiddleware(cfg)
			isAllowed, isSuperAdmin := mw.CheckAuthorization(tt.chatID)

			assert.Equal(t, tt.expectedAllowed, isAllowed, "isAllowed mismatch")
			assert.Equal(t, tt.expectedSuperAdmin, isSuperAdmin, "isSuperAdmin mismatch")
		})
	}
}

func TestMiddleware_WaitForRateLimit_NoError(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 100, // High limit to avoid waiting
				Burst:             10,
			},
		},
	}

	mw := NewMiddleware(cfg)
	
	// Should not error with high limit
	err := mw.WaitForRateLimit()
	assert.NoError(t, err)
}

func TestMiddleware_WaitForRateLimit_RateLimiting(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 2, // Low limit
				Burst:             1,
			},
		},
	}

	mw := NewMiddleware(cfg)
	
	// First call should succeed immediately
	start := time.Now()
	err := mw.WaitForRateLimit()
	assert.NoError(t, err)
	firstDuration := time.Since(start)

	// Second call should also succeed (uses burst)
	err = mw.WaitForRateLimit()
	assert.NoError(t, err)

	// Third call should wait
	start = time.Now()
	err = mw.WaitForRateLimit()
	assert.NoError(t, err)
	thirdDuration := time.Since(start)

	// Third call should take longer due to rate limiting
	assert.True(t, thirdDuration > firstDuration*10, "Expected rate limiting delay")
}

func TestMiddleware_LogCommand_WithMessage(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID:        12345,
				Username:  "testuser",
				FirstName: "Test",
				LastName:  "User",
			},
			Chat: models.Chat{
				ID: 67890,
			},
			MessageThreadID: 5,
		},
	}

	// Should not panic
	mw.LogCommand(update, "test_command")
}

func TestMiddleware_LogCommand_WithMessageNoFrom(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	update := &models.Update{
		Message: &models.Message{
			From: nil,
			Chat: models.Chat{
				ID: 67890,
			},
		},
	}

	// Should not panic
	mw.LogCommand(update, "test_command")
}

func TestMiddleware_LogCommand_WithCallbackQuery(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			From: models.User{
				ID:        12345,
				Username:  "testuser",
				FirstName: "Test",
				LastName:  "User",
			},
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					Chat: models.Chat{
						ID: 67890,
					},
					MessageThreadID: 10,
				},
			},
		},
	}

	// Should not panic
	mw.LogCommand(update, "callback_command")
}

func TestMiddleware_LogCommand_EmptyUsername(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID:        12345,
				Username:  "", // Empty username
				FirstName: "Test",
				LastName:  "User",
			},
			Chat: models.Chat{
				ID: 67890,
			},
		},
	}

	// Should not panic, should use FirstName
	mw.LogCommand(update, "test_command")
}

func TestMiddleware_LogCommand_WithThreadID(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				ID:        12345,
				Username:  "testuser",
				FirstName: "Test",
			},
			Chat: models.Chat{
				ID: 67890,
			},
			MessageThreadID: 42,
		},
	}

	// Should not panic and should handle thread ID
	mw.LogCommand(update, "test_command")
}

func TestMiddleware_LogCommand_NoMessageOrCallback(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	update := &models.Update{
		// No Message or CallbackQuery
	}

	// Should not panic
	mw.LogCommand(update, "test_command")
}

func TestMiddleware_LogUnauthorized(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 10,
				Burst:             5,
			},
		},
	}

	mw := NewMiddleware(cfg)

	// Should not panic
	mw.LogUnauthorized("testuser", 12345)
	mw.LogUnauthorized("", 0)
	mw.LogUnauthorized("user", -123)
}

func TestMiddleware_RateLimitSettings(t *testing.T) {
	tests := []struct {
		name              string
		messagesPerSecond int
		burst             int
	}{
		{"standard rate limit", 25, 5},
		{"low rate limit", 1, 1},
		{"high rate limit", 100, 20},
		{"burst only", 10, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				App: config.AppConfig{
					RateLimit: config.RateLimitConfig{
						MessagesPerSecond: tt.messagesPerSecond,
						Burst:             tt.burst,
					},
				},
			}

			mw := NewMiddleware(cfg)
			assert.NotNil(t, mw)
			assert.NotNil(t, mw.limiter)
		})
	}
}

func TestMiddleware_ConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			AllowedChatIDs: []int64{123, 456, 789},
			SuperAdminIDs:  []int64{123},
		},
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 100, // High to avoid blocking in test
				Burst:             50,
			},
		},
	}

	mw := NewMiddleware(cfg)

	// Test concurrent CheckAuthorization calls
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(chatID int64) {
			isAllowed, isSuperAdmin := mw.CheckAuthorization(chatID)
			assert.NotNil(t, isAllowed)
			assert.NotNil(t, isSuperAdmin)
			done <- true
		}(int64(i % 3))
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestMiddleware_MultipleRateLimitCalls(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			RateLimit: config.RateLimitConfig{
				MessagesPerSecond: 50,
				Burst:             10,
			},
		},
	}

	mw := NewMiddleware(cfg)

	// Make multiple rapid calls
	for i := 0; i < 5; i++ {
		err := mw.WaitForRateLimit()
		assert.NoError(t, err)
	}
}

func TestMiddleware_EdgeCases(t *testing.T) {
	t.Run("nil config fields", func(t *testing.T) {
		cfg := &config.Config{
			Telegram: config.TelegramConfig{
				AllowedChatIDs: nil,
				SuperAdminIDs:  nil,
			},
			App: config.AppConfig{
				RateLimit: config.RateLimitConfig{
					MessagesPerSecond: 10,
					Burst:             5,
				},
			},
		}

		mw := NewMiddleware(cfg)
		isAllowed, isSuperAdmin := mw.CheckAuthorization(123)
		assert.False(t, isAllowed)
		assert.False(t, isSuperAdmin)
	})

	t.Run("empty arrays", func(t *testing.T) {
		cfg := &config.Config{
			Telegram: config.TelegramConfig{
				AllowedChatIDs: []int64{},
				SuperAdminIDs:  []int64{},
			},
			App: config.AppConfig{
				RateLimit: config.RateLimitConfig{
					MessagesPerSecond: 10,
					Burst:             5,
				},
			},
		}

		mw := NewMiddleware(cfg)
		isAllowed, isSuperAdmin := mw.CheckAuthorization(123)
		assert.False(t, isAllowed)
		assert.False(t, isSuperAdmin)
	})

	t.Run("large chat ID", func(t *testing.T) {
		cfg := &config.Config{
			Telegram: config.TelegramConfig{
				AllowedChatIDs: []int64{9223372036854775807},
				SuperAdminIDs:  []int64{},
			},
			App: config.AppConfig{
				RateLimit: config.RateLimitConfig{
					MessagesPerSecond: 10,
					Burst:             5,
				},
			},
		}

		mw := NewMiddleware(cfg)
		isAllowed, isSuperAdmin := mw.CheckAuthorization(9223372036854775807)
		assert.True(t, isAllowed)
		assert.False(t, isSuperAdmin)
	})
}