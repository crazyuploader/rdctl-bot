package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// Bot represents the Telegram bot
type Bot struct {
	api        *bot.Bot
	rdClient   *realdebrid.Client
	middleware *Middleware
	config     *config.Config
}

// NewBot creates a new bot instance
func NewBot(cfg *config.Config, proxyURL, ipTestURL, ipVerifyURL string) (*Bot, error) {
	// Perform IP tests first
	if err := performIPTests(proxyURL, ipTestURL, ipVerifyURL); err != nil {
		return nil, fmt.Errorf("IP test failed: %w", err)
	}

	// Create bot options
	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}

	if cfg.App.LogLevel == "debug" {
		opts = append(opts, bot.WithDebug())
	}

	// Create Telegram bot
	api, err := bot.New(cfg.Telegram.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	// Create Real-Debrid client
	rdClient := realdebrid.NewClient(
		cfg.RealDebrid.BaseURL,
		cfg.RealDebrid.APIToken,
		proxyURL,
		time.Duration(cfg.RealDebrid.Timeout)*time.Second,
	)

	// Create middleware
	middleware := NewMiddleware(cfg)

	me, err := api.GetMe(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}

	log.Printf("Authorized on account @%s", me.Username)

	return &Bot{
		api:        api,
		rdClient:   rdClient,
		middleware: middleware,
		config:     cfg,
	}, nil
}

// Start begins processing updates
func (b *Bot) Start(ctx context.Context) error {
	// Register handlers
	b.registerHandlers()

	log.Println("Bot started. Waiting for messages...")

	// Start receiving updates
	b.api.Start(ctx)

	return nil
}

// registerHandlers sets up all command and callback handlers
func (b *Bot) registerHandlers() {
	// Command handlers
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStartCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, b.handleHelpCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/list", bot.MatchTypeExact, b.handleListCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/add", bot.MatchTypePrefix, b.handleAddCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/info", bot.MatchTypePrefix, b.handleInfoCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/delete", bot.MatchTypePrefix, b.handleDeleteCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/del", bot.MatchTypePrefix, b.handleDeleteCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/unrestrict", bot.MatchTypePrefix, b.handleUnrestrictCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/downloads", bot.MatchTypeExact, b.handleDownloadsCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/removelink", bot.MatchTypePrefix, b.handleRemoveLinkCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/status", bot.MatchTypeExact, b.handleStatusCommand)

	// Callback query handlers
	b.api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "refresh_", bot.MatchTypePrefix, b.handleRefreshCallback)
	b.api.RegisterHandler(bot.HandlerTypeCallbackQueryData, "delete_", bot.MatchTypePrefix, b.handleDeleteCallback)

	// Message handlers for links (not commands)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "magnet:?", bot.MatchTypeContains, b.handleMagnetLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "http://", bot.MatchTypePrefix, b.handleHosterLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "https://", bot.MatchTypePrefix, b.handleHosterLink)
}

// Stop gracefully stops the bot
func (b *Bot) Stop() {
	log.Println("Bot stopped")
}

// defaultHandler handles messages that don't match any registered handler
func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Silently ignore unhandled updates
}

// Helper to check authorization and execute handler
func (b *Bot) withAuth(ctx context.Context, update *models.Update, handler func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool)) {
	var chatID int64
	var messageThreadID int
	var username string

	if update.Message != nil {
		chatID = update.Message.Chat.ID
		if update.Message.MessageThreadID != 0 {
			messageThreadID = update.Message.MessageThreadID
		}
		username = update.Message.From.Username
		if username == "" {
			username = update.Message.From.FirstName
		}
	} else if update.CallbackQuery != nil {
		chatID = update.CallbackQuery.Message.Message.Chat.ID
		if update.CallbackQuery.Message.Message.MessageThreadID != 0 {
			messageThreadID = update.CallbackQuery.Message.Message.MessageThreadID
		}
		username = update.CallbackQuery.From.Username
		if username == "" {
			username = update.CallbackQuery.From.FirstName
		}
	}

	isAllowed, isSuperAdmin := b.middleware.CheckAuthorization(chatID)
	if !isAllowed {
		b.middleware.LogUnauthorized(username, chatID)
		b.sendUnauthorizedMessage(ctx, chatID, messageThreadID)
		return
	}

	handler(ctx, chatID, messageThreadID, isSuperAdmin)
}

// sendUnauthorizedMessage sends an unauthorized message
func (b *Bot) sendUnauthorizedMessage(ctx context.Context, chatID int64, messageThreadID int) {
	text := fmt.Sprintf(
		"⛔ Unauthorized\n\n"+
			"You are not authorized to use this bot.\n"+
			"Your Chat ID: %d\n\n"+
			"Please contact the administrator to get access.",
		chatID,
	)

	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}

	if messageThreadID != 0 {
		params.MessageThreadID = messageThreadID
	}

	if err := b.middleware.WaitForRateLimit(); err != nil {
		log.Printf("Rate limit error: %v", err)
	}

	if _, err := b.api.SendMessage(ctx, params); err != nil {
		log.Printf("Error sending unauthorized message: %v", err)
	}
}

// performIPTests performs IP verification tests
func performIPTests(proxyURL, ipTestURL, ipVerifyURL string) error {
	var ipTestClient *http.Client
	var primaryIP string

	currentIpTestURL := "https://api.ipify.org?format=json"
	if ipTestURL != "" {
		currentIpTestURL = ipTestURL
	}

	if proxyURL != "" {
		log.Println("Proxy configured. Performing IP test...")
		parsedProxyURL, err := url.Parse(proxyURL)
		if err != nil {
			log.Printf("Warning: Invalid proxy URL for IP test: %v. Skipping IP test.", err)
			ipTestClient = &http.Client{Timeout: 10 * time.Second}
		} else {
			ipTestClient = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(parsedProxyURL),
				},
				Timeout: 10 * time.Second,
			}
		}
	} else {
		log.Println("No proxy configured. Performing direct IP test...")
		ipTestClient = &http.Client{Timeout: 10 * time.Second}
	}

	// Perform primary IP test
	resp, err := ipTestClient.Get(currentIpTestURL)
	if err != nil {
		log.Printf("Warning: Failed to perform primary IP test: %v", err)
	} else {
		defer resp.Body.Close()
		var ipResponse struct {
			IP string `json:"ip"`
		}
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &ipResponse); err != nil {
			log.Printf("Warning: Failed to parse primary IP test response: %v", err)
		} else {
			primaryIP = ipResponse.IP
			log.Printf("Primary IP detected: %s", primaryIP)
		}
	}

	// Perform IP verification test if verify URL is provided
	if ipVerifyURL != "" {
		if primaryIP == "" {
			return fmt.Errorf("cannot perform IP verification without a primary IP")
		}
		log.Println("Performing IP verification test...")
		verifyResp, verifyErr := ipTestClient.Get(ipVerifyURL)
		if verifyErr != nil {
			return fmt.Errorf("failed to perform IP verification test: %w", verifyErr)
		}
		defer verifyResp.Body.Close()
		var verifyIpResponse struct {
			IP string `json:"ip"`
		}
		verifyBody, _ := io.ReadAll(verifyResp.Body)
		if err := json.Unmarshal(verifyBody, &verifyIpResponse); err != nil {
			return fmt.Errorf("failed to parse IP verification test response: %w", err)
		}
		log.Printf("Verification IP detected: %s", verifyIpResponse.IP)
		if primaryIP != verifyIpResponse.IP {
			return fmt.Errorf("primary IP (%s) does not match verification IP (%s)", primaryIP, verifyIpResponse.IP)
		}
		log.Println("Primary and verification IPs match. Continuing...")
	}

	return nil
}
