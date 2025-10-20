package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/go-telegram/bot/models"
	"golang.org/x/time/rate"
)

// Middleware handles authorization and rate limiting for bot operations.
// It provides methods to check user authorization and manage message rate limiting.
type Middleware struct {
	config  *config.Config // Application configuration
	limiter *rate.Limiter  // Rate limiter for message sending
}

// NewMiddleware creates a new Middleware instance configured from cfg.
// It initializes an internal rate limiter using cfg.App.RateLimit.MessagesPerSecond and cfg.App.RateLimit.Burst.
func NewMiddleware(cfg *config.Config) *Middleware {
	r := rate.Limit(cfg.App.RateLimit.MessagesPerSecond)
	b := cfg.App.RateLimit.Burst

	return &Middleware{
		config:  cfg,
		limiter: rate.NewLimiter(r, b),
	}
}

// CheckAuthorization verifies if the user is allowed to use the bot.
// It checks if the user is a superadmin or if the chat is in the allowed list.
// Returns:
//   - isAllowed: true if the user is authorized to use the bot
//   - isSuperAdmin: true if the user is a superadmin
func (m *Middleware) CheckAuthorization(chatID, userID int64) (bool, bool) {
	// Check if user is superadmin - they can use bot anywhere
	isSuperAdmin := m.config.IsSuperAdmin(userID)

	// Check if the chat itself is allowed
	isChatAllowed := m.config.IsAllowedChat(chatID)

	// User is allowed if either:
	// 1. They are a superadmin (can use anywhere), OR
	// 2. The chat is in the allowed list
	isAllowed := isSuperAdmin || isChatAllowed

	return isAllowed, isSuperAdmin
}

// WaitForRateLimit waits if rate limit is exceeded.
// It ensures that the bot doesn't send messages too quickly, respecting the configured rate limits.
// Returns an error if the context is canceled or the rate limit cannot be waited on.
func (m *Middleware) WaitForRateLimit() error {
	ctx := context.Background()
	if err := m.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}
	return nil
}

// LogCommand logs command usage to the console.
// It extracts user and chat information from the update and logs the command being executed.
// The log message includes the timestamp, user information, chat information, and the command being executed.
func (m *Middleware) LogCommand(update *models.Update, command string) {
	user := "unknown"
	userID := int64(0)
	chatID := int64(0)
	messageThreadID := int(0)

	if update.Message != nil {
		if update.Message.From != nil {
			user = update.Message.From.Username
			if user == "" {
				user = update.Message.From.FirstName
			}
			userID = update.Message.From.ID
		}
		chatID = update.Message.Chat.ID
		messageThreadID = update.Message.MessageThreadID
	} else if update.CallbackQuery != nil {
		user = update.CallbackQuery.From.Username
		if user == "" {
			user = update.CallbackQuery.From.FirstName
		}
		userID = update.CallbackQuery.From.ID
		if update.CallbackQuery.Message.Message != nil {
			chatID = update.CallbackQuery.Message.Message.Chat.ID
			messageThreadID = update.CallbackQuery.Message.Message.MessageThreadID
		}
	}

	logMessage := fmt.Sprintf("[%s] User: [username=%s, user_id=%d] - Chat: [chat_id=%d",
		time.Now().Format("2006-01-02 15:04:05"),
		user,
		userID,
		chatID,
	)

	if messageThreadID != 0 {
		logMessage += fmt.Sprintf(", topicID=%d]", messageThreadID)
	} else {
		logMessage += "]"
	}

	logMessage += fmt.Sprintf(" - Command: %s", command)

	log.Println(logMessage)
}

// LogUnauthorized logs unauthorized access attempts to the console.
// It records when a user tries to use the bot without proper authorization.
// The log message includes the timestamp, username, user ID, and chat ID of the unauthorized access attempt.
func (m *Middleware) LogUnauthorized(username string, chatID, userID int64) {
	log.Printf("[%s] UNAUTHORIZED - User: %s (UserID: %d, ChatID: %d)",
		time.Now().Format("2006-01-02 15:04:05"),
		username,
		userID,
		chatID,
	)
}
