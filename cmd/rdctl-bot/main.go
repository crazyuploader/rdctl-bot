package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/bot"
	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/crazyuploader/rdctl-bot/internal/web"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information (set via ldflags during build)
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"

	// Configuration file path
	cfgFile string

	// Root command
	rootCmd = &cobra.Command{
		Use:   "rdctl-bot",
		Short: "A Telegram bot for managing Real-Debrid torrents and links",
		Long: `Telegram Real-Debrid Bot is a powerful Telegram bot that allows you to
manage your Real-Debrid torrents and hoster links directly from Telegram.

Features:
  • Add and manage torrents via magnet links
  • Unrestrict and download hoster links
  • View torrent status, progress, and detailed information
  • List all torrents with filtering and pagination
  • Delete torrents and downloads (superadmin only)
  • Real-time torrent information with refresh buttons
  • Automatic file selection for added torrents
  • Chat ID and super admin access restrictions
  • Rate limiting to respect Telegram API limits
  • PostgreSQL database logging for all activities
  • Track user interactions and command history
  • Comprehensive activity auditing
  • Proxy support for Real-Debrid API
  • IP verification and testing
  • Graceful shutdown handling
  • Thread/topic support for Telegram groups
  
Commands:
  /start     - Initialize bot and get your Chat ID
  /help      - Display all available commands
  /list      - List all torrents with details
  /add       - Add magnet link to Real-Debrid
  /info      - Get detailed torrent information
  /delete    - Delete torrent (superadmin only)
  /unrestrict - Unrestrict hoster link
  /downloads - List recent downloads
  /removelink - Remove download from history (superadmin only)
  /status    - Show Real-Debrid account status

The bot also supports direct message handling:
  • Send magnet links directly (auto-adds to Real-Debrid)
  • Send hoster links directly (auto-unrestricts)`,
		Run: runBot,
	}

	// Version command
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Display the version, build date, and git commit of the bot",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("rdctl-bot version %s\n", Version)
			fmt.Printf("Build date: %s\n", BuildDate)
			fmt.Printf("Git commit: %s\n", GitCommit)
		},
	}
)

// init configures CLI flags, binds them to viper configuration keys, and registers subcommands.
func init() {
	// Initialize Cobra
	cobra.OnInitialize(initConfig)

	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "enable debug mode with verbose logging")

	// Local flags (only for root command)
	rootCmd.Flags().Duration("shutdown-timeout", 10*time.Second, "timeout for graceful shutdown")
	rootCmd.Flags().Bool("validate-config", false, "validate configuration and exit")
	rootCmd.Flags().Bool("web-only", false, "enable web-only mode (disable Telegram bot)")

	// Bind flags to viper
	if err := viper.BindPFlag("app.debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		log.Printf("Warning: failed to bind debug flag: %v", err)
	}

	// Bind flags to viper
	if err := viper.BindPFlag("app.shutdown_timeout", rootCmd.Flags().Lookup("shutdown-timeout")); err != nil {
		log.Printf("Warning: failed to bind shutdown-timeout flag: %v", err)
	}

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
}

// initConfig sets Viper's configuration file to the path provided in the
// package-level cfgFile variable when cfgFile is not empty.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
}

// main is the program entry point. It executes the root Cobra command and exits with status 1 on failure.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runBot executes the main lifecycle: loads configuration, starts the bot and web server,
// and manages graceful shutdown with a configurable timeout.
func runBot(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override log level if debug flag is set
	if viper.GetBool("app.debug") {
		cfg.App.LogLevel = "debug"
		log.Printf("Debug mode enabled via flag. Log level set to: %s", cfg.App.LogLevel)
	}

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
	log.Println("Configuration loaded successfully")

	// Check web-only mode early for validation
	webOnly, _ := cmd.Flags().GetBool("web-only")

	// Validate configuration
	if err := cfg.Validate(webOnly); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Validate config and exit if flag is set
	validateOnly, _ := cmd.Flags().GetBool("validate-config")
	if validateOnly {
		log.Println("Configuration is valid!")
		if !webOnly {
			log.Printf("Allowed chat IDs: %v", cfg.Telegram.AllowedChatIDs)
			log.Printf("Super admin IDs: %v", cfg.Telegram.SuperAdminIDs)
		}
		log.Printf("Database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
		log.Printf("Real-Debrid Base URL: %s", cfg.RealDebrid.BaseURL)
		log.Println("Exiting after validation")
		return
	}

	// Initialize database
	database, err := db.Init(cfg.Database.GetDSN())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Log configuration details
	log.Printf("Allowed chat IDs: %v", cfg.Telegram.AllowedChatIDs)
	log.Printf("Super admin IDs: %v", cfg.Telegram.SuperAdminIDs)
	log.Printf("Rate limit: %d messages/sec (burst: %d)", cfg.App.RateLimit.MessagesPerSecond, cfg.App.RateLimit.Burst)
	log.Printf("Database: %s:%d/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)

	// Log Real-Debrid configuration
	if cfg.RealDebrid.Proxy != "" {
		log.Printf("Using proxy: %s", cfg.RealDebrid.Proxy)
	}

	// Log web-only mode
	if webOnly {
		log.Println("Web only mode enabled. Telegram bot will NOT be started.")
	}

	// Create token store for dashboard authentication
	tokenStore := web.NewTokenStore(cfg.Web.TokenExpiryMinutes)

	// Initialize bot
	var b *bot.Bot
	if !webOnly {
		// Create bot instance
		log.Println("Initializing bot...")
		var err error
		b, err = bot.NewBot(cfg, database, cfg.RealDebrid.Proxy, cfg.RealDebrid.IpTestURL, cfg.RealDebrid.IpVerifyURL)
		if err != nil {
			log.Fatalf("Failed to create bot: %v", err)
		}
		// Connect token store to bot for /dashboard command
		b.SetTokenStore(tokenStore)
	}

	// Initialize dependencies for web handlers
	deps := web.Dependencies{
		RDClient:     realdebrid.NewClient(cfg.RealDebrid.BaseURL, cfg.RealDebrid.APIToken, cfg.RealDebrid.Proxy, time.Duration(cfg.RealDebrid.Timeout)*time.Second),
		UserRepo:     db.NewUserRepository(database),
		ActivityRepo: db.NewActivityRepository(database),
		TorrentRepo:  db.NewTorrentRepository(database),
		DownloadRepo: db.NewDownloadRepository(database),
		CommandRepo:  db.NewCommandRepository(database),
		Config:       cfg,
		TokenStore:   tokenStore,
	}

	// Initialize web server
	webServer := web.NewServer(deps)

	// Setup graceful shutdown using context with signal notification
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Channel to listen for errors from bot and web server
	errCh := make(chan error, 2)

	// Start web server in goroutine
	go func() {
		if err := webServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("web server error: %w", err)
		}
	}()

	if !webOnly {
		// Start bot in goroutine
		go func() {
			log.Println("Bot started successfully! Listening for messages...")
			if err := b.Start(ctx); err != nil {
				errCh <- fmt.Errorf("bot error: %w", err)
			}
		}()
	}

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		log.Printf("Received shutdown signal: %v", ctx.Err())
		log.Println("Initiating graceful shutdown...")
	case err := <-errCh:
		log.Printf("Bot encountered an error: %v", err)
		log.Println("Initiating shutdown due to error...")
	}

	// Get shutdown timeout from flags
	shutdownTimeout, _ := cmd.Flags().GetDuration("shutdown-timeout")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Channel for shutdown completion
	shutdownComplete := make(chan struct{})

	// Perform graceful shutdown
	go func() {
		defer close(shutdownComplete)
		log.Println("Stopping components...")

		// Shutdown web server with context for timeout
		if err := webServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down web server: %v", err)
		} else {
			log.Println("Web server stopped gracefully")
		}

		// Stop bot and close database if bot was running
		if !webOnly && b != nil {
			b.Stop()
			log.Println("Bot cleanup completed")
		} else {
			// Close database explicitly when bot is not running
			if sqlDB, err := database.DB(); err == nil {
				if err := sqlDB.Close(); err != nil {
					log.Printf("Error closing database: %v", err)
				} else {
					log.Println("Database connection closed")
				}
			}
		}

		log.Println("Cleanup completed")
	}()

	// Wait for shutdown to complete or timeout
	select {
	case <-shutdownComplete:
		log.Println("Application stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded, forcing exit")
	}

	log.Println("Exited successfully")
}
