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

// Middleware handles authorization and rate limiting
type Middleware struct {
	config  *config.Config
	limiter *rate.Limiter
}

// NewMiddleware creates a Middleware configured from cfg.
// It initializes an internal rate limiter using cfg.App.RateLimit.MessagesPerSecond and cfg.App.RateLimit.Burst.
func NewMiddleware(cfg *config.Config) *Middleware {
	r := rate.Limit(cfg.App.RateLimit.MessagesPerSecond)
	b := cfg.App.RateLimit.Burst

	return &Middleware{
		config:  cfg,
		limiter: rate.NewLimiter(r, b),
	}
}

// CheckAuthorization verifies if the user is allowed to use the bot
func (m *Middleware) CheckAuthorization(chatID, userID int64) (bool, bool) {
	isSuperAdmin := m.config.IsSuperAdmin(userID)
	isAllowed := m.config.IsAllowedChat(chatID) || isSuperAdmin

	return isAllowed, isSuperAdmin
}

// WaitForRateLimit waits if rate limit is exceeded
func (m *Middleware) WaitForRateLimit() error {
	ctx := context.Background()
	if err := m.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit error: %w", err)
	}
	return nil
}

// LogCommand logs command usage
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

	logMessage := fmt.Sprintf("[%s] User: [username=%s, id=%d] - Chat: [id=%d",
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

// LogUnauthorized logs unauthorized access attempts
func (m *Middleware) LogUnauthorized(username string, chatID int64) {
	log.Printf("[%s] UNAUTHORIZED - User: %s (ID: %d)",
		time.Now().Format("2006-01-02 15:04:05"),
		username,
		chatID,
	)
}
