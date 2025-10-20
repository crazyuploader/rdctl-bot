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

// Bot represents the Telegram bot
type Bot struct {
	api          *bot.Bot
	rdClient     *realdebrid.Client
	middleware   *Middleware
	config       *config.Config
	db           *gorm.DB
	userRepo     *db.UserRepository
	activityRepo *db.ActivityRepository
	torrentRepo  *db.TorrentRepository
	downloadRepo *db.DownloadRepository
	commandRepo  *db.CommandRepository
}

// NewBot creates and returns a fully configured Bot.
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

	// Initialize database
	database, err := db.Init(cfg.Database.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

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

// Start begins processing updates
func (b *Bot) Start(ctx context.Context) error {
	b.registerHandlers()
	log.Println("Bot started. Waiting for messages...")
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

	// Message handlers for links
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "magnet:?", bot.MatchTypeContains, b.handleMagnetLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "http://", bot.MatchTypePrefix, b.handleHosterLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "https://", bot.MatchTypePrefix, b.handleHosterLink)
}

// Stop gracefully stops the bot
func (b *Bot) Stop() {
	log.Println("Bot stopping...")
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
	log.Println("Bot stopped")
}

// defaultHandler ignores unhandled updates
func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Silently ignore
}

// getUserFromUpdate extracts user information from an update
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

// withAuth is a middleware to check authorization and execute the handler
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

// sendUnauthorizedMessage sends an unauthorized message
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

// maskUsername masks the username for privacy
func (b *Bot) maskUsername(username string) string {
	if len(username) <= 5 {
		return "*****"
	}
	return "*****" + username[5:]
}

// performIPTests performs IP address checks using an optional proxy and test endpoints.
// When ipVerifyURL is provided, it verifies the primary IP (from ipTestURL or default) matches the verification endpoint and returns an error if the primary IP cannot be obtained, the verification request or response parsing fails, or the IPs do not match; it returns nil on success.
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
