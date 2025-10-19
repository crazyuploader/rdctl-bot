package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/bot"
	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version information (set via ldflags during build)
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"

	cfgFile string
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

// init registers CLI flags, binds them to viper configuration keys, and attaches subcommands.
// It defines persistent flags for config file and debug mode, root-local flags for shutdown
// timeout and configuration validation, binds those flags to `app.debug` and
// `app.shutdown_timeout` viper keys, and adds the version subcommand.
func init() {
	cobra.OnInitialize(initConfig)

	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "enable debug mode with verbose logging")

	// Local flags (only for root command)
	rootCmd.Flags().Duration("shutdown-timeout", 10*time.Second, "timeout for graceful shutdown")
	rootCmd.Flags().Bool("validate-config", false, "validate configuration and exit")

	// Bind flags to viper
	viper.BindPFlag("app.debug", rootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("app.shutdown_timeout", rootCmd.Flags().Lookup("shutdown-timeout"))

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
}

// initConfig sets Viper's configuration file path when cfgFile is not empty.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
}

// main is the program entry point for the CLI application.
// It executes the root cobra command and, on execution error, prints the error to stderr and exits with status 1.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runBot starts the Telegram Real‑Debrid bot: it loads and optionally validates configuration, initializes the bot, and orchestrates runtime signal- and error-driven graceful shutdown with a configurable timeout.
// cmd is the Cobra command invoking the action; args are the positional command-line arguments passed to the command.
func runBot(cmd *cobra.Command, args []string) {
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)

	log.Println("Starting rdctl-bot...")
	log.Printf("Version: %s, Build: %s, Commit: %s", Version, BuildDate, GitCommit)

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Println("Configuration loaded successfully")

	// Validate config and exit if flag is set
	validateOnly, _ := cmd.Flags().GetBool("validate-config")
	if validateOnly {
		log.Println("Configuration is valid!")
		log.Printf("Allowed chat IDs: %v", cfg.Telegram.AllowedChatIDs)
		log.Printf("Super admin IDs: %v", cfg.Telegram.SuperAdminIDs)
		log.Printf("Database: %s@%s:%d/%s", cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
		log.Printf("Real-Debrid Base URL: %s", cfg.RealDebrid.BaseURL)
		log.Println("Exiting after validation")
		return
	}

	// Log configuration details
	log.Printf("Allowed chat IDs: %v", cfg.Telegram.AllowedChatIDs)
	log.Printf("Super admin IDs: %v", cfg.Telegram.SuperAdminIDs)
	log.Printf("Rate limit: %d messages/sec (burst: %d)", cfg.App.RateLimit.MessagesPerSecond, cfg.App.RateLimit.Burst)
	log.Printf("Database: %s:%d/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)

	if cfg.RealDebrid.Proxy != "" {
		log.Printf("Using proxy: %s", cfg.RealDebrid.Proxy)
	}

	// Create bot instance
	log.Println("Initializing bot...")
	b, err := bot.NewBot(cfg, cfg.RealDebrid.Proxy, cfg.RealDebrid.IpTestURL, cfg.RealDebrid.IpVerifyURL)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to listen for OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Channel to listen for errors from bot
	errCh := make(chan error, 1)

	// Start bot in goroutine
	go func() {
		log.Println("Bot started successfully! Listening for messages...")
		if err := b.Start(ctx); err != nil {
			errCh <- fmt.Errorf("bot error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		log.Printf("Received shutdown signal: %v", sig)
		log.Println("Initiating graceful shutdown...")
	case err := <-errCh:
		log.Printf("Bot encountered an error: %v", err)
		log.Println("Initiating shutdown due to error...")
	}

	// Stop signal notification (allows force quit with second Ctrl+C)
	signal.Stop(sigCh)
	log.Println("Press Ctrl+C again to force shutdown")

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

		log.Println("Stopping bot...")
		cancel() // Cancel bot context

		// Allow bot to cleanup
		time.Sleep(500 * time.Millisecond)

		b.Stop() // Close database and other resources

		log.Println("Cleanup completed")
	}()

	// Wait for shutdown to complete or timeout
	select {
	case <-shutdownComplete:
		log.Println("Bot stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded, forcing exit")
	case <-sigCh:
		log.Println("Force shutdown requested")
		os.Exit(130) // Standard exit code for Ctrl+C
	}

	log.Println("Bot exited successfully")
}