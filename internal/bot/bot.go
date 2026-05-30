package bot

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/crazyuploader/rdctl-bot/internal/web"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RealDebridClient defines the required interface for Real-Debrid operations.
// This allows for mocking in unit tests.
type RealDebridClient interface {
	GetTorrents(limit, offset int) ([]realdebrid.Torrent, error)
	GetTorrentsWithCount(limit, offset int) (*realdebrid.TorrentsResult, error)
	GetActiveCount() (*realdebrid.ActiveCount, error)
	GetTorrentInfo(torrentID string) (*realdebrid.Torrent, error)
	AddMagnet(magnetURL string) (*realdebrid.AddMagnetResponse, error)
	SelectFiles(torrentID string, fileIDs []int) error
	SelectAllFiles(torrentID string) error
	DeleteTorrent(torrentID string) error
	CheckInstantAvailability(hashes []string) (realdebrid.InstantAvailability, error)
	GetUser() (*realdebrid.User, error)
	GetDownloads(limit, offset int) ([]realdebrid.Download, error)
	GetDownloadsWithCount(limit, offset int) (*realdebrid.DownloadsResult, error)
	UnrestrictLink(link string) (*realdebrid.UnrestrictedLink, error)
	DeleteDownload(downloadID string) error
	GetSupportedRegex() ([]string, error)
}

// Bot represents the Telegram bot
type Bot struct {
	api            *bot.Bot
	rdClient       RealDebridClient
	middleware     *Middleware
	supportedRegex []*regexp.Regexp
	config         *config.Config
	db             *pgxpool.Pool
	userRepo       *db.UserRepository
	activityRepo   *db.ActivityRepository
	torrentRepo    *db.TorrentRepository
	downloadRepo   *db.DownloadRepository
	commandRepo    *db.CommandRepository
	settingRepo    *db.SettingRepository
	keptRepo       *db.KeptTorrentRepository
	chatRepo       *db.ChatRepository
	tokenStore     *web.TokenStore
	wg             sync.WaitGroup
	cancel         context.CancelFunc
	systemUserID   int64
}

// IPTestConfig holds configuration for proxy IP testing
type IPTestConfig struct {
	ProxyURL      string
	TestURL       string // URL to fetch IP from (default: https://api.ipify.org?format=json)
	StremThruURL  string // If set, verifies primary IP via StremThru /v0/health/__debug__
	StremThruAuth string // Optional "username:password" for StremThru Basic auth (sent as Proxy-Authorization header)
}

// NewBot creates and returns a fully configured Bot.
func NewBot(cfg *config.Config, database *pgxpool.Pool, ipTest IPTestConfig) (*Bot, error) {
	// Perform IP tests first
	if err := performIPTests(ipTest); err != nil {
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
		ipTest.ProxyURL,
		time.Duration(cfg.RealDebrid.Timeout)*time.Second,
	)

	// Create middleware
	middleware := NewMiddleware(cfg)

	me, err := api.GetMe(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}

	log.Printf("Authorized on account @%s", me.Username)

	// Fetch supported regexes
	regexList, err := rdClient.GetSupportedRegex()
	var supportedRegex []*regexp.Regexp
	if err != nil {
		log.Printf("Warning: Failed to fetch supported regexes: %v. All links will be allowed (fallback).", err)
	} else {
		for _, r := range regexList {
			// Real-Debrid regexes are returned as /pattern/ strings (PCRE style)
			// Go uses RE2 and doesn't use delimiters.
			// We need to strip the leading/trailing slashes and handle escapes.
			if len(r) > 2 && r[0] == '/' && r[len(r)-1] == '/' {
				r = r[1 : len(r)-1]
			}
			// Unescape \/ to / because / is not a special character in Go regex
			// format in the string is literal BACKSLASH + SLASH -> \/
			// We want just SLASH -> /
			// Note: The string r already has JSON string escapes removed, so it contains literal backslashes
			r = strings.ReplaceAll(r, `\/`, `/`)

			compiled, err := regexp.Compile(r)
			if err != nil {
				log.Printf("Warning: Failed to compile regex '%s': %v", r, err)
				continue
			}
			supportedRegex = append(supportedRegex, compiled)
		}
		log.Printf("Loaded %d supported host regexes", len(supportedRegex))
	}

	b := &Bot{
		api:            api,
		rdClient:       rdClient,
		middleware:     middleware,
		supportedRegex: supportedRegex,
		config:         cfg,
		db:             database,
		userRepo:       db.NewUserRepository(database),
		activityRepo:   db.NewActivityRepository(database),
		torrentRepo:    db.NewTorrentRepository(database),
		downloadRepo:   db.NewDownloadRepository(database),
		commandRepo:    db.NewCommandRepository(database),
		settingRepo:    db.NewSettingRepository(database),
		keptRepo:       db.NewKeptTorrentRepository(database),
		chatRepo:       db.NewChatRepository(database),
	}

	// Create or retrieve system user for automated operations
	systemUser, err := b.userRepo.GetOrCreateUser(context.Background(), 0, "system", "System", "Bot", "", false, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create system user: %w", err)
	}
	b.systemUserID = systemUser.ID

	return b, nil
}

// Start begins processing updates
func (b *Bot) Start(ctx context.Context) error {
	b.registerHandlers()

	// Create a cancellable context for the bot's lifecycle
	botCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	// Start auto-delete background worker
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.startAutoDeleteWorker(botCtx)
	}()

	// Start auto-delete warning worker
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.startAutoDeleteWarningWorker(botCtx)
	}()

	log.Println("Bot started. Waiting for messages...")
	b.api.Start(botCtx)
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
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/stats", bot.MatchTypeExact, b.handleStatsCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/dashboard", bot.MatchTypeExact, b.handleDashboardCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/autodelete-interval", bot.MatchTypePrefix, b.handleAutoDeleteIntervalCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/autodelete", bot.MatchTypePrefix, b.handleAutoDeleteCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/keep", bot.MatchTypePrefix, b.handleKeepCommand)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "/unkeep", bot.MatchTypePrefix, b.handleUnkeepCommand)

	// Message handlers for links
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "magnet:?", bot.MatchTypeContains, b.handleMagnetLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "http://", bot.MatchTypePrefix, b.handleHosterLink)
	b.api.RegisterHandler(bot.HandlerTypeMessageText, "https://", bot.MatchTypePrefix, b.handleHosterLink)
}

// Stop gracefully stops the bot and closes the database connection
func (b *Bot) Stop() {
	log.Println("Bot stopping...")

	// Cancel the bot context to signal all workers to stop
	if b.cancel != nil {
		b.cancel()
	}

	// Wait for background workers to finish
	b.wg.Wait()

	db.Close(b.db)
	log.Println("Bot stopped")
}

// SetTokenStore sets the token store for dashboard access
func (b *Bot) SetTokenStore(ts *web.TokenStore) {
	b.tokenStore = ts
}

// defaultHandler ignores unhandled updates
func defaultHandler(_ context.Context, _ *bot.Bot, _ *models.Update) {
	// Silently ignore
}

// UserInfo holds extracted user information from an update
type UserInfo struct {
	ChatID          int64
	MessageThreadID int
	Username        string
	FirstName       string
	LastName        string
	LanguageCode    string
	IsBot           bool
	IsPremium       bool
	UserID          int64
}

// getUserFromUpdate extracts user information from an update
func getUserFromUpdate(update *models.Update) UserInfo {
	var info UserInfo
	var from *models.User
	if update.Message != nil {
		info.ChatID = update.Message.Chat.ID
		if update.Message.MessageThreadID != 0 {
			info.MessageThreadID = update.Message.MessageThreadID
		}
		from = update.Message.From
	} else if update.CallbackQuery != nil {
		if update.CallbackQuery.Message.Message != nil {
			info.ChatID = update.CallbackQuery.Message.Message.Chat.ID
			if update.CallbackQuery.Message.Message.MessageThreadID != 0 {
				info.MessageThreadID = update.CallbackQuery.Message.Message.MessageThreadID
			}
		}
		from = &update.CallbackQuery.From
	}
	if from != nil {
		info.Username = from.Username
		info.FirstName = from.FirstName
		info.LastName = from.LastName
		info.UserID = from.ID
		info.LanguageCode = from.LanguageCode
		info.IsBot = from.IsBot
		info.IsPremium = from.IsPremium
	}

	if info.Username == "" {
		info.Username = info.FirstName
	}
	return info
}

// getChatFromUpdate extracts chat info from an update
func getChatFromUpdate(update *models.Update) (chatID int64, title, chatUsername, chatType string, isForum bool) {
	var chat *models.Chat
	if update.Message != nil {
		chat = &update.Message.Chat
	} else if update.CallbackQuery != nil && update.CallbackQuery.Message.Message != nil {
		chat = &update.CallbackQuery.Message.Message.Chat
	}
	if chat != nil {
		chatID = chat.ID
		title = chat.Title
		chatUsername = chat.Username
		chatType = string(chat.Type)
		isForum = chat.IsForum
		if title == "" {
			title = chat.Username
		}
		if title == "" {
			title = strings.TrimSpace(chat.FirstName + " " + chat.LastName)
		}
	}
	return
}

// withAuth is a middleware to check authorization and execute the handler
func (b *Bot) withAuth(ctx context.Context, update *models.Update, handler func(ctx context.Context, chatID int64, chatPK int64, messageThreadID int, isSuperAdmin bool, user *db.User)) {
	userInfo := getUserFromUpdate(update)
	_, title, chatUsername, chatType, isForum := getChatFromUpdate(update)

	isAllowed, isSuperAdmin := b.middleware.CheckAuthorization(userInfo.ChatID, userInfo.UserID)

	chatPK := int64(0)
	chat, err := b.chatRepo.GetOrCreateChat(ctx, userInfo.ChatID, title, chatUsername, chatType, isForum)
	if err != nil {
		log.Printf("Warning: failed to automatically log chat ID: %v", err)
	}
	if chat != nil {
		chatPK = chat.ID
	}

	var user *db.User
	if userInfo.UserID != 0 {
		user, err = b.userRepo.GetOrCreateUser(ctx, userInfo.UserID, userInfo.Username, userInfo.FirstName, userInfo.LastName, userInfo.LanguageCode, userInfo.IsBot, userInfo.IsPremium, isSuperAdmin)
		if err != nil {
			log.Printf("Error getting/creating user: %v", err)
			if userInfo.ChatID != 0 {
				if err2 := b.middleware.WaitForRateLimit(); err2 != nil {
					log.Printf("Rate limit error: %v", err2)
				}
				_, _ = b.api.SendMessage(ctx, &bot.SendMessageParams{
					ChatID:          userInfo.ChatID,
					Text:            "[ERROR] An internal error occurred. Please try again later.",
					MessageThreadID: userInfo.MessageThreadID,
				})
			}
			return
		}
	} else {
		log.Printf("Warning: missing user ID in update, skipping user tracking")
	}

	if !isAllowed {
		b.middleware.LogUnauthorized(userInfo.Username, userInfo.ChatID, userInfo.UserID)
		b.sendUnauthorizedMessage(ctx, userInfo.ChatID, userInfo.MessageThreadID, userInfo.UserID)
		if user != nil {
			if err := b.activityRepo.LogActivity(ctx, "", user.ID, chatPK, userInfo.Username, db.ActivityTypeUnauthorized, "", 0, userInfo.MessageThreadID, false, "Unauthorized access attempt", nil); err != nil {
				log.Printf("Warning: failed to log unauthorized activity: %v", err)
			}
		}
		return
	}

	// Check topic restrictions if configured
	if !b.config.IsAllowedTopic(userInfo.ChatID, userInfo.MessageThreadID) {
		log.Printf("Topic %d not allowed for chat %d (config topics: %v)", userInfo.MessageThreadID, userInfo.ChatID, b.config.Telegram.AllowedTopicIDs[fmt.Sprintf("%d", userInfo.ChatID)])
		b.middleware.LogUnauthorized(userInfo.Username, userInfo.ChatID, userInfo.UserID)
		return
	}

	handler(ctx, userInfo.ChatID, chatPK, userInfo.MessageThreadID, isSuperAdmin, user)
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
func maskUsername(username string) string {
	if len(username) <= 5 {
		return "*****"
	}
	return "*****" + username[5:]
}

// performIPTests checks the bot's outbound IP. With cfg.StremThruURL set, it also
// queries /v0/health/__debug__ to log StremThru's outbound IP (exposed["*"] or machine).
// With cfg.ProxyURL set, confirms StremThru sees the proxy as the caller.
// On StremThru unreachability, retries indefinitely: exponential backoff 2s-5min, +-20% jitter.
func performIPTests(cfg IPTestConfig) error {
	ipTestURL := "https://api.ipify.org?format=json"
	if cfg.TestURL != "" {
		ipTestURL = cfg.TestURL
	}

	primaryIP := fetchPrimaryIP(buildIPTestClient(cfg.ProxyURL), ipTestURL)

	if cfg.StremThruURL == "" {
		return nil
	}

	stOutboundIP, err := queryStremThruOutboundIP(cfg)
	if err != nil {
		return err
	}

	if primaryIP != "" && primaryIP != stOutboundIP {
		return fmt.Errorf(
			"IP mismatch: bot uses %s but StremThru proxies from %s; configure a proxy so both IPs match",
			primaryIP, stOutboundIP,
		)
	}
	log.Printf("IP check passed: bot and StremThru both use %s", stOutboundIP)
	return nil
}

func buildIPTestClient(proxyURL string) *http.Client {
	if proxyURL == "" {
		log.Println("No proxy configured. Performing direct IP test...")
		return &http.Client{Timeout: 10 * time.Second}
	}
	log.Println("Proxy configured. Performing IP test...")
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		log.Printf("Warning: invalid proxy URL for IP test: %v. Skipping proxy.", err)
		return &http.Client{Timeout: 10 * time.Second}
	}
	return &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(parsed)},
		Timeout:   10 * time.Second,
	}
}

func fetchPrimaryIP(client *http.Client, testURL string) string {
	resp, err := client.Get(testURL)
	if err != nil {
		log.Printf("Warning: failed to perform primary IP test: %v", err)
		return ""
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("Warning: failed to close response body: %v", cerr)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Warning: failed to read primary IP test response: %v", err)
		return ""
	}
	var ipResponse struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(body, &ipResponse); err != nil {
		log.Printf("Warning: failed to parse primary IP test response: %v", err)
		return ""
	}
	log.Printf("Primary IP detected: %s", ipResponse.IP)
	return ipResponse.IP
}

func queryStremThruOutboundIP(cfg IPTestConfig) (string, error) {
	const (
		initialBackoff = 2 * time.Second
		maxBackoff     = 5 * time.Minute
		jitterFactor   = 0.2 // +-20%
	)
	verifyURL := strings.TrimRight(cfg.StremThruURL, "/") + "/v0/health/__debug__"
	// StremThru is always dialed directly; the local proxy routes bot traffic only.
	stClient := &http.Client{Timeout: 10 * time.Second}
	backoff := initialBackoff

	for attempt := 1; ; attempt++ {
		log.Printf("Performing StremThru IP verification test (attempt %d)...", attempt)

		req, err := http.NewRequest(http.MethodGet, verifyURL, http.NoBody)
		if err != nil {
			return "", fmt.Errorf("failed to create StremThru verify request: %w", err)
		}
		if cfg.StremThruAuth != "" {
			encoded := base64.StdEncoding.EncodeToString([]byte(cfg.StremThruAuth))
			req.Header.Set("X-StremThru-Authorization", "Basic "+encoded)
		}

		resp, err := stClient.Do(req)
		if err != nil {
			waitDuration := jitteredBackoff(backoff, jitterFactor, initialBackoff)
			log.Printf("StremThru not available (attempt %d): %v. Retrying in %s...", attempt, err, waitDuration.Round(time.Millisecond))
			time.Sleep(waitDuration)
			backoff = time.Duration(math.Min(float64(backoff*2), float64(maxBackoff)))
			continue
		}

		return parseStremThruOutboundIP(resp)
	}
}

func parseStremThruOutboundIP(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if cerr := resp.Body.Close(); cerr != nil {
		log.Printf("Warning: failed to close StremThru response body: %v", cerr)
	}
	if err != nil {
		return "", fmt.Errorf("failed to read StremThru verify response: %w", err)
	}
	var stResp struct {
		Data struct {
			IP struct {
				Machine string            `json:"machine"`
				Exposed map[string]string `json:"exposed"`
			} `json:"ip"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &stResp); err != nil {
		return "", fmt.Errorf("failed to parse StremThru verify response: %w", err)
	}
	// exposed["*"] is tunnel outbound; machine is bare server. RealDebrid sees this IP.
	outboundIP := stResp.Data.IP.Machine
	if ip := stResp.Data.IP.Exposed["*"]; ip != "" {
		outboundIP = ip
	}
	if outboundIP == "" {
		return "", fmt.Errorf("StremThru verify response: no usable outbound IP")
	}
	log.Printf("StremThru outbound IP (seen by upstream services): %s", outboundIP)
	return outboundIP, nil
}

// jitteredBackoff returns backoff +- (factor * 100)% using crypto/rand.
// Falls back to plain backoff if the random read fails.
func jitteredBackoff(backoff time.Duration, factor float64, minDuration time.Duration) time.Duration {
	var rb [8]byte
	if _, err := cryptorand.Read(rb[:]); err != nil {
		return backoff
	}
	ratio := float64(binary.BigEndian.Uint64(rb[:]))/float64(^uint64(0))*2 - 1 // -1.0 to +1.0
	d := backoff + time.Duration(float64(backoff)*factor*ratio)
	if d < minDuration {
		return minDuration
	}
	return d
}
