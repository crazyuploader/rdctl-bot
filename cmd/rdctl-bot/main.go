package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/crazyuploader/rdctl-bot/internal/bot"
	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "telegram-rd-bot",
		Short: "A Telegram bot for managing Real-Debrid torrents and links",
		Long: `Telegram Real-Debrid Bot is a powerful Telegram bot that allows you to
manage your Real-Debrid torrents and hoster links directly from Telegram.

Features:
- Add and manage torrents via magnet links
- Unrestrict hoster links
- View torrent status and progress
- Delete torrents and downloads
- Chat ID and super admin restrictions
- Rate limiting to respect Telegram API limits`,
		Run: runBot,
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	// Add flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.Flags().BoolP("debug", "d", false, "Enable debug mode")

	// Bind flags to viper
	viper.BindPFlag("app.debug", rootCmd.Flags().Lookup("debug"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runBot(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Println("Configuration loaded successfully")
	log.Printf("Allowed chat IDs: %v", cfg.Telegram.AllowedChatIDs)
	log.Printf("Super admin IDs: %v", cfg.Telegram.SuperAdminIDs)

	// Create bot
	b, err := bot.NewBot(cfg)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start bot in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := b.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigCh:
		log.Println("Received shutdown signal")
		cancel()
		b.Stop()
	case err := <-errCh:
		log.Fatalf("Bot error: %v", err)
	}

	log.Println("Bot stopped successfully")
}
