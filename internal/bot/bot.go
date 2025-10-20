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
	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"
)

// Bot represents the main Telegram bot controller that handles all interactions with the Telegram API
// and coordinates operations with Real-Debrid and the database.
type Bot struct {
	api          *bot.Bot               // Telegram bot API client
	rdClient     *realdebrid.Client     // Real-Debrid client for torrent operations
	middleware   *Middleware            // Middleware for authorization and rate limiting
	config       *config.Config         // Application configuration
	db           *gorm.DB               // Database connection
	userRepo     *db.UserRepository     // User repository for database operations
	activityRepo *db.ActivityRepository // Activity repository for logging user actions
	torrentRepo  *db.TorrentRepository  // Torrent repository for torrent-related operations
	downloadRepo *db.DownloadRepository // Download repository for download-related operations
	commandRepo  *db.CommandRepository  // Command repository for command logging
}

// NewBot creates and returns a fully configured Bot instance.
// It initializes all components including the Telegram bot, Real-Debrid client,
// middleware, and database repositories.
func NewBot(cfg *config.Config, proxyURL, ipTestURL, ipVerifyURL string) (*Bot, error) {
	// Perform IP tests to verify network configuration
	if err := performIPTests(proxyURL, ipTestURL, ipVerifyURL); err != nil {
		return nil, fmt.Errorf("IP test failed: %w", err)
	}

	// Create bot options with default handler and debug mode if configured
	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}
	if cfg.App.LogLevel == "debug" {
		opts = append(opts, bot.WithDebug())
	}

	// Create Telegram bot with the provided token and options
	api, err := bot.New(cfg.Telegram.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	// Create Real-Debrid client with configuration from the config
	rdClient := realdebrid.NewClient(
		cfg.RealDebrid.BaseURL,
		cfg.RealDebrid.APIToken,
		proxyURL,
		time.Duration(cfg.RealDebrid.Timeout)*time.Second,
	)

	// Create middleware for authorization and rate limiting
	middleware := NewMiddleware(cfg)

	// Get bot information from Telegram API
	me, err := api.GetMe(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}

	log.Printf("Authorized on account @%s", me.Username)

	// Initialize database connection
	database, err := db.Init(cfg.Database.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create and return a fully configured Bot instance
	return &Bot{
		api:          api,
		rdClient:     rdClient,
		middleware:   middleware,
		config:       cfg,
		db:           database,
		userRepo:     db.NewUserRepository(database),
		activityRepo: db.NewActivityRepository(database),
		torrentRepo:  db.NewTorrentRepository(database),
		downloadRepo: db.NewDownloadRepository(database),
		commandRepo:  db.NewCommandRepository(database),
	}, nil
}

// Start begins processing updates from the Telegram API.
// It registers all command handlers and starts the bot's update loop.
func (b *Bot) Start(ctx context.Context) error {
	b.registerHandlers()
	log.Println("Bot started. Waiting for messages...")
	b.api.Start(ctx)
	return nil
}

// registerHandlers sets up all command and callback handlers for the bot.
// It registers handlers for various commands and message types that the bot will respond to.
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

	// Message handlers for links
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "magnet:?", bot.MatchTypeContains, b.handleMagnetLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "http://", bot.MatchTypePrefix, b.handleHosterLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "https://", bot.MatchTypePrefix, b.handleHosterLink)
}

// Stop gracefully stops the bot and closes the database connection.
// It performs cleanup operations to ensure resources are properly released.
func (b *Bot) Stop() {
	log.Println("Bot stopping...")
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
	log.Println("Bot stopped")
}

// defaultHandler ignores unhandled updates.
// This is the default handler that will be called for any updates that don't match registered handlers.
func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Silently ignore unhandled updates
}

// getUserFromUpdate extracts user information from an update object.
// It retrieves the user's ID, username, first name, last name, and chat ID from either a message or callback query.
func (b *Bot) getUserFromUpdate(update *models.Update) (chatID int64, messageThreadID int, username, firstName, lastName string, userID int64) {
	if update.Message != nil {
		chatID = update.Message.Chat.ID
		if update.Message.MessageThreadID != 0 {
			messageThreadID = update.Message.MessageThreadID
		}
		if update.Message.From != nil {
			username = update.Message.From.Username
			firstName = update.Message.From.FirstName
			lastName = update.Message.From.LastName
			userID = update.Message.From.ID
		}
	} else if update.CallbackQuery != nil {
		if update.CallbackQuery.Message.Message != nil {
			chatID = update.CallbackQuery.Message.Message.Chat.ID
			if update.CallbackQuery.Message.Message.MessageThreadID != 0 {
				messageThreadID = update.CallbackQuery.Message.Message.MessageThreadID
			}
		}
		username = update.CallbackQuery.From.Username
		firstName = update.CallbackQuery.From.FirstName
		lastName = update.CallbackQuery.From.LastName
		userID = update.CallbackQuery.From.ID
	}

	if username == "" {
		username = firstName
	}
	return
}

// withAuth is a middleware function that checks user authorization and executes the handler.
// It verifies if the user is allowed to use the bot, creates or updates the user record,
// and executes the provided handler function with the user information.
func (b *Bot) withAuth(ctx context.Context, update *models.Update, handler func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User)) {
	chatID, messageThreadID, username, firstName, lastName, userID := b.getUserFromUpdate(update)

	isAllowed, isSuperAdmin := b.middleware.CheckAuthorization(chatID, userID)

	user, err := b.userRepo.GetOrCreateUser(userID, username, firstName, lastName, isSuperAdmin)
	if err != nil {
		log.Printf("Error getting/creating user: %v", err)
		if chatID != 0 {
			if err2 := b.middleware.WaitForRateLimit(); err2 != nil {
				log.Printf("Rate limit error: %v", err2)
			}
			_, _ = b.api.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:          chatID,
				Text:            "[ERROR] An internal error occurred. Please try again later.",
				MessageThreadID: messageThreadID,
			})
		}
		return
	}

	if !isAllowed {
		b.middleware.LogUnauthorized(username, chatID, userID)
		b.sendUnauthorizedMessage(ctx, chatID, messageThreadID, userID)
		if user != nil {
			b.activityRepo.LogActivity(user.ID, chatID, username, db.ActivityTypeUnauthorized, "", messageThreadID, false, "Unauthorized access attempt", nil)
		}
		return
	}

	handler(ctx, chatID, messageThreadID, isSuperAdmin, user)
}

// sendUnauthorizedMessage sends an unauthorized access message to the user.
// It informs the user that they are not authorized to use the bot and provides their user ID and chat ID.
func (b *Bot) sendUnauthorizedMessage(ctx context.Context, chatID int64, messageThreadID int, userID int64) {
	text := fmt.Sprintf(
		"[UNAUTHORIZED]\n\n"+
			"You are not authorized to use this bot.\n\n"+
			"Your User ID is: <code>%d</code>\n"+
			"Chat ID: <code>%d</code>\n\n"+
			"Please contact the administrator to add your User ID to the super admin list or add this chat to the allowed chats list.",
		userID,
		chatID,
	)

	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
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

// maskUsername masks the username for privacy by replacing the first 5 characters with asterisks.
// This helps protect user privacy by not exposing their full username in logs and messages.
func (b *Bot) maskUsername(username string) string {
	if len(username) <= 5 {
		return "*****"
	}
	return "*****" + username[5:]
}

// performIPTests performs IP address checks using an optional proxy and test endpoints.
// It verifies that the bot can reach the specified IP test URL and, if provided,
// that the verification URL returns the same IP address. This helps ensure proper
// network configuration and proxy settings.
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
