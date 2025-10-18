package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot represents the Telegram bot
type Bot struct {
	api        *tgbotapi.BotAPI
	rdClient   *realdebrid.Client
	middleware *Middleware
	config     *config.Config
}

// NewBot creates a new bot instance
func NewBot(cfg *config.Config) (*Bot, error) {
	// Create Telegram bot API
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	// Set debug mode based on log level
	api.Debug = cfg.App.LogLevel == "debug"

	// Create Real-Debrid client
	rdClient := realdebrid.NewClient(
		cfg.RealDebrid.BaseURL,
		cfg.RealDebrid.APIToken,
		time.Duration(cfg.RealDebrid.Timeout)*time.Second,
	)

	// Create middleware
	middleware := NewMiddleware(cfg)

	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:        api,
		rdClient:   rdClient,
		middleware: middleware,
		config:     cfg,
	}, nil
}

// Start begins processing updates
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	log.Println("Bot started. Waiting for messages...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down bot...")
			b.api.StopReceivingUpdates()
			return nil

		case update := <-updates:
			go b.handleUpdate(update)
		}
	}
}

// handleUpdate processes a single update
func (b *Bot) handleUpdate(update tgbotapi.Update) {
	// Handle callback queries
	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		_, isSuperAdmin := b.middleware.CheckAuthorization(chatID)
		b.handleCallbackQuery(update, isSuperAdmin)
		return
	}

	// Handle messages
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// Check authorization
	isAllowed, isSuperAdmin := b.middleware.CheckAuthorization(chatID)
	if !isAllowed {
		b.middleware.LogCommand(update, "UNAUTHORIZED")
		if err := b.middleware.SendUnauthorizedMessage(b.api, chatID); err != nil {
			log.Printf("Error sending unauthorized message: %v", err)
		}
		return
	}

	// Handle commands
	if update.Message.IsCommand() {
		b.handleCommand(update, isSuperAdmin)
		return
	}

	// Handle non-command messages (e.g., magnet links)
	b.handleMessage(update)
}

// Stop gracefully stops the bot
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}
