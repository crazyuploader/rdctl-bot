package bot

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserFromUpdate_Message(t *testing.T) {
	// Create a minimal bot instance for testing helper methods
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "testuser",
				FirstName: "Test",
				LastName:  "User",
			},
			Chat: models.Chat{
				ID: 123456,
			},
			MessageThreadID: 5,
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(123456), chatID)
	assert.Equal(t, 5, messageThreadID)
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "Test", firstName)
	assert.Equal(t, "User", lastName)
}

func TestGetUserFromUpdate_MessageNoThreadID(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "testuser",
				FirstName: "Test",
				LastName:  "User",
			},
			Chat: models.Chat{
				ID: 123456,
			},
			MessageThreadID: 0, // No thread ID
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(123456), chatID)
	assert.Equal(t, 0, messageThreadID)
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "Test", firstName)
	assert.Equal(t, "User", lastName)
}

func TestGetUserFromUpdate_MessageEmptyUsername(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "", // Empty username
				FirstName: "Test",
				LastName:  "User",
			},
			Chat: models.Chat{
				ID: 123456,
			},
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(123456), chatID)
	assert.Equal(t, 0, messageThreadID)
	assert.Equal(t, "Test", username, "Should use firstName when username is empty")
	assert.Equal(t, "Test", firstName)
	assert.Equal(t, "User", lastName)
}

func TestGetUserFromUpdate_MessageNoFrom(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: nil, // No from user
			Chat: models.Chat{
				ID: 123456,
			},
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(123456), chatID)
	assert.Equal(t, 0, messageThreadID)
	assert.Equal(t, "", username)
	assert.Equal(t, "", firstName)
	assert.Equal(t, "", lastName)
}

func TestGetUserFromUpdate_CallbackQuery(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			From: models.User{
				Username:  "callbackuser",
				FirstName: "Callback",
				LastName:  "User",
			},
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					Chat: models.Chat{
						ID: 789012,
					},
					MessageThreadID: 10,
				},
			},
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(789012), chatID)
	assert.Equal(t, 10, messageThreadID)
	assert.Equal(t, "callbackuser", username)
	assert.Equal(t, "Callback", firstName)
	assert.Equal(t, "User", lastName)
}

func TestGetUserFromUpdate_CallbackQueryEmptyUsername(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			From: models.User{
				Username:  "", // Empty username
				FirstName: "Callback",
				LastName:  "User",
			},
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					Chat: models.Chat{
						ID: 789012,
					},
				},
			},
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(789012), chatID)
	assert.Equal(t, 0, messageThreadID)
	assert.Equal(t, "Callback", username, "Should use firstName when username is empty")
	assert.Equal(t, "Callback", firstName)
	assert.Equal(t, "User", lastName)
}

func TestGetUserFromUpdate_CallbackQueryNoMessage(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			From: models.User{
				Username:  "user",
				FirstName: "First",
				LastName:  "Last",
			},
			Message: models.MaybeInaccessibleMessage{
				Message: nil, // No message
			},
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(0), chatID)
	assert.Equal(t, 0, messageThreadID)
	assert.Equal(t, "user", username)
	assert.Equal(t, "First", firstName)
	assert.Equal(t, "Last", lastName)
}

func TestGetUserFromUpdate_NoMessageOrCallback(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		// Neither Message nor CallbackQuery set
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(0), chatID)
	assert.Equal(t, 0, messageThreadID)
	assert.Equal(t, "", username)
	assert.Equal(t, "", firstName)
	assert.Equal(t, "", lastName)
}

func TestGetUserFromUpdate_SpecialCharacters(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "test_user-123",
				FirstName: "Test 😀",
				LastName:  "O'Brien",
			},
			Chat: models.Chat{
				ID: 123456,
			},
		},
	}

	chatID, messageThreadID, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, int64(123456), chatID)
	assert.Equal(t, "test_user-123", username)
	assert.Contains(t, firstName, "😀")
	assert.Contains(t, lastName, "'")
}

func TestGetUserFromUpdate_NegativeChatID(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "testuser",
				FirstName: "Test",
			},
			Chat: models.Chat{
				ID: -123456789, // Negative chat ID (groups)
			},
		},
	}

	chatID, _, _, _, _ := b.getUserFromUpdate(update)

	assert.Equal(t, int64(-123456789), chatID)
}

func TestGetUserFromUpdate_LargeChatID(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username: "testuser",
			},
			Chat: models.Chat{
				ID: 9223372036854775807, // Max int64
			},
		},
	}

	chatID, _, _, _, _ := b.getUserFromUpdate(update)

	assert.Equal(t, int64(9223372036854775807), chatID)
}

func TestGetUserFromUpdate_EmptyFirstAndLastName(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "",
				FirstName: "",
				LastName:  "",
			},
			Chat: models.Chat{
				ID: 123456,
			},
		},
	}

	_, _, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, "", username)
	assert.Equal(t, "", firstName)
	assert.Equal(t, "", lastName)
}

func TestGetUserFromUpdate_OnlyFirstName(t *testing.T) {
	b := &Bot{}

	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "",
				FirstName: "SingleName",
				LastName:  "",
			},
			Chat: models.Chat{
				ID: 123456,
			},
		},
	}

	_, _, username, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, "SingleName", username, "Should use firstName when username is empty")
	assert.Equal(t, "SingleName", firstName)
	assert.Equal(t, "", lastName)
}

func TestGetUserFromUpdate_MultipleThreadIDs(t *testing.T) {
	b := &Bot{}

	tests := []struct {
		name            string
		messageThreadID int
	}{
		{"thread ID 1", 1},
		{"thread ID 100", 100},
		{"thread ID 999999", 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := &models.Update{
				Message: &models.Message{
					From: &models.User{
						Username: "testuser",
					},
					Chat: models.Chat{
						ID: 123456,
					},
					MessageThreadID: tt.messageThreadID,
				},
			}

			_, messageThreadID, _, _, _ := b.getUserFromUpdate(update)
			assert.Equal(t, tt.messageThreadID, messageThreadID)
		})
	}
}

func TestGetUserFromUpdate_LongUsername(t *testing.T) {
	b := &Bot{}

	longUsername := "very_long_username_with_many_characters_123456789"
	
	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  longUsername,
				FirstName: "Test",
			},
			Chat: models.Chat{
				ID: 123456,
			},
		},
	}

	_, _, username, _, _ := b.getUserFromUpdate(update)

	assert.Equal(t, longUsername, username)
}

func TestGetUserFromUpdate_LongNames(t *testing.T) {
	b := &Bot{}

	longFirstName := "VeryLongFirstNameWithManyCharactersThatExceedsNormalLength"
	longLastName := "VeryLongLastNameWithManyCharactersThatExceedsNormalLength"
	
	update := &models.Update{
		Message: &models.Message{
			From: &models.User{
				Username:  "user",
				FirstName: longFirstName,
				LastName:  longLastName,
			},
			Chat: models.Chat{
				ID: 123456,
			},
		},
	}

	_, _, _, firstName, lastName := b.getUserFromUpdate(update)

	assert.Equal(t, longFirstName, firstName)
	assert.Equal(t, longLastName, lastName)
}

func TestDefaultHandler(t *testing.T) {
	// defaultHandler should do nothing and not panic
	require.NotPanics(t, func() {
		defaultHandler(nil, nil, nil)
	})
}

func TestBot_Structure(t *testing.T) {
	// Test that Bot struct has expected fields
	b := &Bot{}
	
	assert.NotNil(t, b, "Bot should be instantiable")
	// These fields should exist (nil initially but structurally present)
	_ = b.api
	_ = b.rdClient
	_ = b.middleware
	_ = b.config
	_ = b.db
	_ = b.userRepo
	_ = b.activityRepo
	_ = b.torrentRepo
	_ = b.downloadRepo
	_ = b.commandRepo
}