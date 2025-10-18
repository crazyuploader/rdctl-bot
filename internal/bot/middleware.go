package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/time/rate"
)

// Middleware handles authorization and rate limiting
type Middleware struct {
	config  *config.Config
	limiter *rate.Limiter
}

// NewMiddleware creates a new middleware instance
func NewMiddleware(cfg *config.Config) *Middleware {
	// Create rate limiter based on config
	r := rate.Limit(cfg.App.RateLimit.MessagesPerSecond)
	b := cfg.App.RateLimit.Burst

	return &Middleware{
		config:  cfg,
		limiter: rate.NewLimiter(r, b),
	}
}

// CheckAuthorization verifies if the user is allowed to use the bot
func (m *Middleware) CheckAuthorization(chatID int64) (bool, bool) {
	isSuperAdmin := m.config.IsSuperAdmin(chatID)
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
func (m *Middleware) LogCommand(update tgbotapi.Update, command string) {
	user := "unknown"
	chatID := int64(0)

	if update.Message != nil {
		if update.Message.From != nil {
			user = update.Message.From.UserName
			if user == "" {
				user = update.Message.From.FirstName
			}
		}
		chatID = update.Message.Chat.ID
	} else if update.CallbackQuery != nil {
		if update.CallbackQuery.From != nil {
			user = update.CallbackQuery.From.UserName
			if user == "" {
				user = update.CallbackQuery.From.FirstName
			}
		}
		chatID = update.CallbackQuery.Message.Chat.ID
	}

	log.Printf("[%s] User: %s (ID: %d) - Command: %s",
		time.Now().Format("2006-01-02 15:04:05"),
		user,
		chatID,
		command,
	)
}

// SendUnauthorizedMessage sends unauthorized message to user
func (m *Middleware) SendUnauthorizedMessage(bot *tgbotapi.BotAPI, chatID int64) error {
	msg := tgbotapi.NewMessage(chatID,
		"⛔ Unauthorized\n\n"+
			"You are not authorized to use this bot.\n"+
			fmt.Sprintf("Your Chat ID: %d\n\n", chatID)+
			"Please contact the administrator to get access.",
	)

	if err := m.WaitForRateLimit(); err != nil {
		return err
	}

	_, err := bot.Send(msg)
	return err
}
