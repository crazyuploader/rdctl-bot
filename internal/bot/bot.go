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
func NewBot(cfg *config.Config, proxyURL, ipTestURL, ipVerifyURL string) (*Bot, error) {
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
		proxyURL,
		time.Duration(cfg.RealDebrid.Timeout)*time.Second,
	)

	// Determine the HTTP client to use for IP tests
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
		var ipResponse struct { IP string `json:"ip"` }
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
			log.Fatalf("Error: Cannot perform IP verification without a primary IP. Exiting.")
		}
		log.Println("Performing IP verification test...")
		verifyResp, verifyErr := ipTestClient.Get(ipVerifyURL)
		if verifyErr != nil {
			log.Fatalf("Error: Failed to perform IP verification test: %v", verifyErr)
		} else {
			defer verifyResp.Body.Close()
			var verifyIpResponse struct { IP string `json:"ip"` }
			verifyBody, _ := io.ReadAll(verifyResp.Body)
			if err := json.Unmarshal(verifyBody, &verifyIpResponse); err != nil {
				log.Fatalf("Error: Failed to parse IP verification test response: %v", err)
			} else {
				log.Printf("Verification IP detected: %s", verifyIpResponse.IP)
				if primaryIP != verifyIpResponse.IP {
					log.Fatalf("Error: Primary IP (%s) does not match verification IP (%s). Exiting.", primaryIP, verifyIpResponse.IP)
				}
				log.Println("Primary and verification IPs match. Continuing...")
			}
		}
	}

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
