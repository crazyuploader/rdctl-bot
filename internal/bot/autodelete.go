package bot

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	// settingAutoDeleteDays is the DB key for the auto-delete configuration
	settingAutoDeleteDays = "auto_delete_days"

	// autoDeleteCheckInterval defines how often the worker checks for old torrents
	autoDeleteCheckInterval = 1 * time.Hour

	// maxAutoDeleteDays is the maximum allowed value for auto-delete days
	maxAutoDeleteDays = 3650
)

// handleAutoDeleteCommand handles the /autodelete command (superadmin only)
func (b *Bot) handleAutoDeleteCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "autodelete")

		if !isSuperAdmin {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>[ERROR]</b> Access Denied. This command is for superadmins only.", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(ctx, user.ID, chatID, user.Username, "autodelete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Unauthorized - not superadmin", 0); err != nil {
					log.Printf("Warning: failed to log unauthorized autodelete command: %v", err)
				}
			}
			return
		}

		parts := strings.Fields(update.Message.Text)

		// If no argument, show current setting
		if len(parts) < 2 {
			currentValue, err := b.settingRepo.GetSetting(ctx, settingAutoDeleteDays)
			if err != nil {
				text := fmt.Sprintf("<b>[ERROR]</b> Failed to read setting: %s", html.EscapeString(err.Error()))
				b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
				return
			}

			var text string
			switch currentValue {
			case "":
				if b.config.App.AutoDeleteDays > 0 {
					text = fmt.Sprintf(
						"<b>⏳ Auto-Delete</b>\n\n"+
							"Torrents older than <b>%d days</b> are automatically deleted (using config.yaml default).\n\n"+
							"<b>Usage:</b> <code>/autodelete &lt;days&gt;</code>\n"+
							"Set to <code>0</code> to disable.",
						b.config.App.AutoDeleteDays,
					)
				} else {
					text = "<b>⏳ Auto-Delete</b>\n\n" +
						"Auto-delete is currently <b>disabled</b>.\n\n" +
						"<b>Usage:</b> <code>/autodelete &lt;days&gt;</code>\n" +
						"Set to <code>0</code> to disable."
				}
			case "0":
				text = "<b>⏳ Auto-Delete</b>\n\n" +
					"Auto-delete is currently <b>disabled</b>.\n\n" +
					"<b>Usage:</b> <code>/autodelete &lt;days&gt;</code>\n" +
					"Set to <code>0</code> to disable."
			default:
				text = fmt.Sprintf(
					"<b>⏳ Auto-Delete</b>\n\n"+
						"Torrents older than <b>%s days</b> are automatically deleted.\n\n"+
						"<b>Usage:</b> <code>/autodelete &lt;days&gt;</code>\n"+
						"Set to <code>0</code> to disable.",
					currentValue,
				)
			}
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			b.logCommandHelper(ctx, user, chatID, messageThreadID, "autodelete", update.Message.Text, startTime, true, "", len(text))
			return
		}

		// Parse the days argument
		daysStr := parts[1]
		days, err := strconv.Atoi(daysStr)
		if err != nil || days < 0 || days > maxAutoDeleteDays {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, fmt.Sprintf("<b>[ERROR]</b> Please provide a valid number of days (0 to %d).", maxAutoDeleteDays), update.Message.ID)
			b.logCommandHelper(ctx, user, chatID, messageThreadID, "autodelete", update.Message.Text, startTime, false, "Invalid days value", 0)
			return
		}

		// Save setting to DB
		if err := b.settingRepo.SetSetting(ctx, settingAutoDeleteDays, strconv.Itoa(days)); err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to save setting: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			b.logCommandHelper(ctx, user, chatID, messageThreadID, "autodelete", update.Message.Text, startTime, false, err.Error(), 0)
			return
		}

		var text string
		if days == 0 {
			text = "<b>⏳ Auto-Delete Disabled</b>\n\nTorrents will no longer be automatically deleted."
		} else {
			text = fmt.Sprintf(
				"<b>⏳ Auto-Delete Configured</b>\n\n"+
					"Torrents older than <b>%d days</b> will be automatically deleted.\n"+
					"The cleanup runs every hour.",
				days,
			)
		}

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
		b.logCommandHelper(ctx, user, chatID, messageThreadID, "autodelete", update.Message.Text, startTime, true, "", len(text))
	})
}

// startAutoDeleteWorker runs a background goroutine that periodically deletes old torrents.
// It reads the auto_delete_days setting from the DB on each tick. The worker stops when ctx is cancelled.
func (b *Bot) startAutoDeleteWorker(ctx context.Context) {
	ticker := time.NewTicker(autoDeleteCheckInterval)
	defer ticker.Stop()

	log.Println("Auto-delete worker started (checking every hour)")

	// Run first check immediately on startup
	b.runAutoDeleteCheck(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Auto-delete worker stopped")
			return
		case <-ticker.C:
			b.runAutoDeleteCheck(ctx)
		}
	}
}

// runAutoDeleteCheck performs a single auto-delete check cycle
func (b *Bot) runAutoDeleteCheck(ctx context.Context) {
	daysStr, err := b.settingRepo.GetSetting(ctx, settingAutoDeleteDays)
	if err != nil {
		log.Printf("Auto-delete: failed to read setting: %v", err)
		return
	}

	var days int
	switch daysStr {
	case "":
		// Use fallback
		days = b.config.App.AutoDeleteDays
		if days <= 0 {
			return // Auto-delete is disabled
		}
	case "0":
		return // Explicitly disabled
	default:
		var parseErr error
		days, parseErr = strconv.Atoi(daysStr)
		if parseErr != nil || days <= 0 {
			return
		}
	}

	// Get kept torrent IDs to skip them during deletion
	keptTorrentIDs, err := b.keptRepo.GetKeptTorrentIDs(ctx)
	if err != nil {
		log.Printf("Auto-delete: failed to get kept torrent IDs: %v", err)
		// Continue anyway, but we won't be able to skip kept torrents
		keptTorrentIDs = make(map[string]bool)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	log.Printf("Auto-delete: checking for torrents older than %d days (before %s)", days, cutoff.Format("2006-01-02 15:04"))

	// Fetch torrents in batches to handle large lists
	const batchSize = 100
	offset := 0
	var oldTorrents []realdebrid.Torrent
	totalDeleted := 0
	totalSkipped := 0

	for {
		torrents, err := b.rdClient.GetTorrents(batchSize, offset)
		if err != nil {
			log.Printf("Auto-delete: failed to fetch torrents (offset=%d): %v", offset, err)
			break
		}

		if len(torrents) == 0 {
			break
		}

		for _, t := range torrents {
			// Skip if torrent is marked as kept
			if keptTorrentIDs[t.ID] {
				totalSkipped++
				continue
			}

			if t.Added.Before(cutoff) {
				oldTorrents = append(oldTorrents, t)
			}
		}

		// If we got fewer results than batch size, we've reached the end
		if len(torrents) < batchSize {
			break
		}

		offset += batchSize
	}

	for i, t := range oldTorrents {
		// Retry deletion with exponential backoff for transient errors
		var deleteErr error
		maxRetries := 3
		baseDelay := 1 * time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			deleteErr = b.rdClient.DeleteTorrent(t.ID)
			if deleteErr == nil {
				// Success - break out of retry loop
				break
			}

			// Check if error is retryable (rate limit or server error)
			errStr := deleteErr.Error()
			isRetryable := strings.Contains(errStr, "429") ||
				strings.Contains(errStr, "500") ||
				strings.Contains(errStr, "502") ||
				strings.Contains(errStr, "503") ||
				strings.Contains(errStr, "504")

			if !isRetryable || attempt == maxRetries-1 {
				// Not retryable or last attempt - break
				break
			}

			// Exponential backoff: wait 1s, 2s, 4s...
			backoffDelay := baseDelay * time.Duration(1<<uint(attempt))
			log.Printf("Auto-delete: retry %d/%d for torrent %s after error: %v (waiting %v)",
				attempt+1, maxRetries, t.ID, deleteErr, backoffDelay)
			time.Sleep(backoffDelay)
		}

		if deleteErr != nil {
			log.Printf("Auto-delete: failed to delete torrent %s (%s) after %d attempts: %v",
				t.ID, t.Filename, maxRetries, deleteErr)
			continue
		}

		log.Printf("Auto-delete: deleted torrent %s (%s), added on %s", t.ID, t.Filename, t.Added.Format("2006-01-02"))
		totalDeleted++

		// Log the deletion to the DB for auditing (use system user ID)
		if err := b.torrentRepo.LogTorrentActivity(ctx, b.systemUserID, 0, t.ID, t.Hash, t.Filename, "", "delete", "auto_deleted", t.Bytes, t.Progress, true, "", map[string]interface{}{"auto_delete_days": days}); err != nil {
			log.Printf("Auto-delete: failed to log torrent deletion: %v", err)
		}

		// Add a small delay between successful deletes to avoid rate limiting
		// Skip delay on the last torrent
		if i != len(oldTorrents)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if totalDeleted > 0 {
		log.Printf("Auto-delete: completed, deleted %d torrent(s)", totalDeleted)
	}
	if totalSkipped > 0 {
		log.Printf("Auto-delete: skipped %d kept torrent(s)", totalSkipped)
	}
}