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

	// warningCheckInterval defines how often to check for torrents needing warning
	warningCheckInterval = 1 * time.Hour
)

// handleAutoDeleteCommand handles the /autodelete command (superadmin only)
func (b *Bot) handleAutoDeleteCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
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
							"Torrents and downloads older than <b>%d days</b> are automatically deleted (using config.yaml default).\n\n"+
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
						"Torrents and downloads older than <b>%s days</b> are automatically deleted.\n\n"+
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
		if err := b.settingRepo.SetSettingWithAudit(ctx, settingAutoDeleteDays, strconv.Itoa(days), user.UserID, chatID); err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to save setting: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			b.logCommandHelper(ctx, user, chatID, messageThreadID, "autodelete", update.Message.Text, startTime, false, err.Error(), 0)
			return
		}

		var text string
		if days == 0 {
			text = "<b>⏳ Auto-Delete Disabled</b>\n\nTorrents and downloads will no longer be automatically deleted."
		} else {
			text = fmt.Sprintf(
				"<b>⏳ Auto-Delete Configured</b>\n\n"+
					"Torrents and downloads older than <b>%d days</b> will be automatically deleted.\n"+
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

	// Offset delete cutoff by warning hours so every torrent passes through the
	// warning window before it becomes eligible for deletion.
	hoursBefore := b.config.App.AutoDeleteWarning.HoursBefore
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	if hoursBefore > 0 {
		cutoff = cutoff.Add(-time.Duration(hoursBefore) * time.Hour)
	}
	log.Printf("Auto-delete: checking for torrents older than %d days + %d hours (before %s)", days, hoursBefore, cutoff.Format("2006-01-02 15:04"))

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
		// Re-validate if the torrent was kept since we captured the initial IDs snapshot
		isKept, err := b.keptRepo.IsKept(ctx, t.ID)
		if err == nil && isKept {
			log.Printf("Auto-delete: skipped %s as it was recently marked to be kept", t.ID)
			totalSkipped++
			continue
		}

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

			if !isRetryableHTTPError(deleteErr) || attempt == maxRetries-1 {
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
		if err := b.torrentRepo.LogTorrentActivity(ctx, "", b.systemUserID, 0, t.ID, t.Hash, t.Filename, "", "delete", "auto_deleted", t.Bytes, t.Progress, true, "", map[string]interface{}{"auto_delete_days": days}); err != nil {
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
		b.sendAutoDeleteLogMessage(ctx, oldTorrents, totalDeleted)
	}
	if totalSkipped > 0 {
		log.Printf("Auto-delete: skipped %d kept torrent(s)", totalSkipped)
	}

	// Auto-delete old downloads
	b.runAutoDeleteDownloads(ctx, days)
}

// sendAutoDeleteLogMessage sends a message to the configured auto-delete warning chat
// showing the torrents that were deleted and providing options to keep them.
func (b *Bot) sendAutoDeleteLogMessage(ctx context.Context, deletedTorrents []realdebrid.Torrent, totalDeleted int) {
	chatID := b.config.App.AutoDeleteWarning.ChatID
	if chatID == 0 {
		return
	}
	topicID := b.config.App.AutoDeleteWarning.TopicID

	if len(deletedTorrents) == 0 {
		return
	}

	// Build the message
	header := fmt.Sprintf("<b>🗑️ Auto-Delete Completed</b>\n\n"+
		"The following <b>%d torrent(s)</b> have been automatically deleted.\n\n", totalDeleted)

	const footer = "\n<i>These torrents can no longer be kept.</i>"

	const maxMessageLength = 4000
	var messages []string
	var currentBatch strings.Builder

	currentBatch.WriteString(header)
	batchCount := 0

	for _, t := range deletedTorrents {
		filename := html.EscapeString(t.Filename)
		addedDays := int(time.Since(t.Added).Hours() / 24)
		line := fmt.Sprintf(
			"• <code>%s</code> — <i>%s</i> (<b>%d</b> days old)\n",
			t.ID,
			filename,
			addedDays,
		)

		if currentBatch.Len()+len(line)+len(footer)+100 > maxMessageLength {
			currentBatch.WriteString(footer)
			messages = append(messages, currentBatch.String())
			currentBatch.Reset()
			currentBatch.WriteString(header)
			batchCount = 0
		}

		currentBatch.WriteString(line)
		batchCount++
	}

	if batchCount > 0 {
		currentBatch.WriteString(footer)
		messages = append(messages, currentBatch.String())
	}

	// Send each batch
	for i, msg := range messages {
		if err := b.sendHTMLMessageWithErr(ctx, chatID, topicID, msg, 0); err != nil {
			log.Printf("Auto-delete log: failed to send batch %d/%d to chat %d: %v", i+1, len(messages), chatID, err)
		} else {
			log.Printf("Auto-delete log: successfully sent batch %d/%d for %d deleted torrent(s) to chat %d", i+1, len(messages), totalDeleted, chatID)
		}
	}
}

// isRetryableHTTPError reports whether err represents a transient HTTP error worth retrying.
func isRetryableHTTPError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") ||
		strings.Contains(s, "500") ||
		strings.Contains(s, "502") ||
		strings.Contains(s, "503") ||
		strings.Contains(s, "504")
}

// runAutoDeleteDownloads performs auto-delete for downloads
func (b *Bot) runAutoDeleteDownloads(ctx context.Context, days int) {
	hoursBefore := b.config.App.AutoDeleteWarning.HoursBefore
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	if hoursBefore > 0 {
		cutoff = cutoff.Add(-time.Duration(hoursBefore) * time.Hour)
	}
	log.Printf("Auto-delete: checking for downloads older than %d days + %d hours (before %s)", days, hoursBefore, cutoff.Format("2006-01-02 15:04"))

	const batchSize = 100
	offset := 0
	var oldDownloads []realdebrid.Download

	for {
		downloads, err := b.rdClient.GetDownloads(batchSize, offset)
		if err != nil {
			log.Printf("Auto-delete: failed to fetch downloads (offset=%d): %v", offset, err)
			break
		}

		if len(downloads) == 0 {
			break
		}

		for _, d := range downloads {
			if d.Generated.Before(cutoff) {
				oldDownloads = append(oldDownloads, d)
			}
		}

		if len(downloads) < batchSize {
			break
		}

		offset += batchSize
	}

	var successfullyDeleted []realdebrid.Download

	for i, d := range oldDownloads {
		var deleteErr error
		maxRetries := 3
		baseDelay := 1 * time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			deleteErr = b.rdClient.DeleteDownload(d.ID)
			if deleteErr == nil {
				break
			}

			if !isRetryableHTTPError(deleteErr) || attempt == maxRetries-1 {
				break
			}

			backoffDelay := baseDelay * time.Duration(1<<uint(attempt))
			log.Printf("Auto-delete: retry %d/%d for download %s after error: %v (waiting %v)",
				attempt+1, maxRetries, d.ID, deleteErr, backoffDelay)
			time.Sleep(backoffDelay)
		}

		if deleteErr != nil {
			log.Printf("Auto-delete: failed to delete download %s (%s) after %d attempts: %v",
				d.ID, d.Filename, maxRetries, deleteErr)
			continue
		}

		log.Printf("Auto-delete: deleted download %s (%s), generated on %s", d.ID, d.Filename, d.Generated.Format("2006-01-02"))
		successfullyDeleted = append(successfullyDeleted, d)

		if err := b.downloadRepo.LogDownloadActivity(ctx, "", b.systemUserID, 0, d.ID, "", d.Filename, "", "delete", d.Filesize, true, "auto_deleted", nil, nil); err != nil {
			log.Printf("Auto-delete: failed to log download deletion: %v", err)
		}

		if i != len(oldDownloads)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(successfullyDeleted) > 0 {
		log.Printf("Auto-delete: completed, deleted %d download(s)", len(successfullyDeleted))
		b.sendAutoDeleteDownloadsLogMessage(ctx, successfullyDeleted)
	}
}

// sendAutoDeleteDownloadsLogMessage sends a message about deleted downloads
func (b *Bot) sendAutoDeleteDownloadsLogMessage(ctx context.Context, deletedDownloads []realdebrid.Download) {
	chatID := b.config.App.AutoDeleteWarning.ChatID
	if chatID == 0 {
		return
	}
	topicID := b.config.App.AutoDeleteWarning.TopicID

	if len(deletedDownloads) == 0 {
		return
	}

	header := fmt.Sprintf("<b>🗑️ Auto-Delete Completed</b>\n\n"+
		"The following <b>%d download link(s)</b> have been automatically cleared.\n\n", len(deletedDownloads))

	const footer = "\n<i>These download links have been removed.</i>"

	const maxMessageLength = 4000
	var messages []string
	var currentBatch strings.Builder
	hasItems := false

	currentBatch.WriteString(header)

	for _, d := range deletedDownloads {
		filename := html.EscapeString(d.Filename)
		generatedDays := int(time.Since(d.Generated).Hours() / 24)
		line := fmt.Sprintf(
			"• <code>%s</code> — <i>%s</i> (<b>%d</b> days old)\n",
			d.ID,
			filename,
			generatedDays,
		)

		if currentBatch.Len()+len(line)+len(footer)+100 > maxMessageLength {
			currentBatch.WriteString(footer)
			messages = append(messages, currentBatch.String())
			currentBatch.Reset()
			currentBatch.WriteString(header)
		}

		currentBatch.WriteString(line)
		hasItems = true
	}

	if hasItems {
		currentBatch.WriteString(footer)
		messages = append(messages, currentBatch.String())
	}

	for i, msg := range messages {
		if err := b.sendHTMLMessageWithErr(ctx, chatID, topicID, msg, 0); err != nil {
			log.Printf("Auto-delete downloads log: failed to send batch %d/%d to chat %d: %v", i+1, len(messages), chatID, err)
		} else {
			log.Printf("Auto-delete downloads log: successfully sent batch %d/%d for %d deleted download(s) to chat %d", i+1, len(messages), len(deletedDownloads), chatID)
		}
	}
}

// startAutoDeleteWarningWorker runs a background goroutine that periodically sends warnings
// for torrents about to be auto-deleted. It stops when ctx is cancelled.
func (b *Bot) startAutoDeleteWarningWorker(ctx context.Context) {
	ticker := time.NewTicker(warningCheckInterval)
	defer ticker.Stop()

	log.Println("Auto-delete warning worker started (checking every hour)")

	// Run first check after a short delay; scan the full warning window so existing
	// at-risk torrents are always notified, not just newly-entered ones.
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
		b.runAutoDeleteWarningCheck(ctx, true)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Auto-delete warning worker stopped")
			return
		case <-ticker.C:
			b.runAutoDeleteWarningCheck(ctx, false)
		}
	}
}

// runAutoDeleteWarningCheck performs a single warning check cycle.
// fullScan=true warns about all torrents currently in the warning window (used on startup);
// fullScan=false only warns about torrents that newly entered the window since the last run.
func (b *Bot) runAutoDeleteWarningCheck(ctx context.Context, fullScan bool) {
	// Check if warning is configured
	chatID := b.config.App.AutoDeleteWarning.ChatID
	if chatID == 0 {
		return
	}

	topicID := b.config.App.AutoDeleteWarning.TopicID
	hoursBefore := b.config.App.AutoDeleteWarning.HoursBefore

	// Get auto-delete days setting
	daysStr, err := b.settingRepo.GetSetting(ctx, settingAutoDeleteDays)
	if err != nil {
		log.Printf("Auto-delete warning: failed to read setting: %v", err)
		return
	}

	var days int
	switch daysStr {
	case "":
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

	// Get kept torrent IDs to skip them during warning
	keptTorrentIDs, err := b.keptRepo.GetKeptTorrentIDs(ctx)
	if err != nil {
		log.Printf("Auto-delete warning: failed to get kept torrent IDs: %v", err)
		keptTorrentIDs = make(map[string]bool)
	}

	// Validate configuration: warning hours must be less than retention window
	if hoursBefore >= days*24 {
		log.Printf("Auto-delete warning: invalid configuration - warning window (%d hours) must be less than retention window (%d hours). Skipping warning check.", hoursBefore, days*24)
		return
	}

	// Calculate the cutoff time for deletion and the warning window
	deleteCutoff := time.Now().UTC().AddDate(0, 0, -days)
	warningCutoff := deleteCutoff.Add(time.Duration(hoursBefore) * time.Hour)

	log.Printf("Auto-delete warning: checking for torrents to be deleted in %d hours (before %s)", hoursBefore, deleteCutoff.Format("2006-01-02 15:04"))

	// On a full scan (startup), warn about everything in the window; otherwise
	// only warn about torrents that newly entered the window since the last run.
	var previousWarningCutoff time.Time
	if fullScan {
		previousWarningCutoff = deleteCutoff
	} else {
		previousWarningCutoff = warningCutoff.Add(-warningCheckInterval)
	}

	// Fetch torrents in batches
	const batchSize = 100
	offset := 0
	var torrentsToWarn []realdebrid.Torrent

	for {
		torrents, err := b.rdClient.GetTorrents(batchSize, offset)
		if err != nil {
			log.Printf("Auto-delete warning: failed to fetch torrents (offset=%d): %v", offset, err)
			break
		}

		if len(torrents) == 0 {
			break
		}

		for _, t := range torrents {
			if keptTorrentIDs[t.ID] {
				continue
			}

			// Check if torrent entered the warning window since the last run
			if t.Added.Before(warningCutoff) && !t.Added.Before(previousWarningCutoff) && !t.Added.Before(deleteCutoff) {
				torrentsToWarn = append(torrentsToWarn, t)
			}
		}

		if len(torrents) < batchSize {
			break
		}

		offset += batchSize
	}

	if len(torrentsToWarn) == 0 {
		return
	}

	// Build warning messages in batches to stay under Telegram's 4096 character limit
	const maxMessageLength = 4000 // Leave some margin for safety
	var messages []string
	var currentBatch strings.Builder

	header := fmt.Sprintf("<b>⚠️ Torrents Scheduled for Auto-Deletion</b>\n\n"+
		"The following torrents will be deleted in approximately <b>%d hours</b>.\n"+
		"Use <code>/keep &lt;id&gt;</code> to keep any torrent from being deleted.\n\n", hoursBefore)

	currentBatch.WriteString(header)
	batchCount := 0

	for _, t := range torrentsToWarn {
		filename := html.EscapeString(t.Filename)
		addedDays := int(time.Since(t.Added).Hours() / 24)
		line := fmt.Sprintf(
			"• <code>/keep %s</code> — <i>%s</i> (<b>%d</b> days old)\n",
			t.ID,
			filename,
			addedDays,
		)

		// Check if adding this line would exceed the limit
		if currentBatch.Len()+len(line)+100 > maxMessageLength { // +100 for footer
			// Finalize current batch
			footer := fmt.Sprintf("\n<i>Batch %d of torrents scheduled for deletion</i>", len(messages)+1)
			currentBatch.WriteString(footer)
			messages = append(messages, currentBatch.String())

			// Start new batch
			currentBatch.Reset()
			currentBatch.WriteString(header)
			batchCount = 0
		}

		currentBatch.WriteString(line)
		batchCount++
	}

	// Add final batch if it has content
	if batchCount > 0 {
		footer := fmt.Sprintf("\n<i>%d torrent(s) scheduled for deletion</i>", len(torrentsToWarn))
		currentBatch.WriteString(footer)
		messages = append(messages, currentBatch.String())
	}

	// Send each batch individually and check for errors
	successCount := 0
	for i, msg := range messages {
		if err := b.sendHTMLMessageWithErr(ctx, chatID, topicID, msg, 0); err != nil {
			log.Printf("Auto-delete warning: failed to send batch %d/%d to chat %d: %v", i+1, len(messages), chatID, err)
		} else {
			successCount++
			log.Printf("Auto-delete warning: successfully sent batch %d/%d for %d torrent(s) to chat %d", i+1, len(messages), len(torrentsToWarn), chatID)
		}
	}

	if successCount == len(messages) {
		log.Printf("Auto-delete warning: successfully sent all %d batch(es) for %d torrent(s) to chat %d", len(messages), len(torrentsToWarn), chatID)
	} else {
		log.Printf("Auto-delete warning: sent %d/%d batches successfully for %d torrent(s) to chat %d", successCount, len(messages), len(torrentsToWarn), chatID)
	}

	// Also check for downloads to warn about
	b.runAutoDeleteDownloadsWarning(ctx, chatID, topicID, days, hoursBefore, fullScan)
}

// runAutoDeleteDownloadsWarning sends warnings for downloads about to be auto-deleted
func (b *Bot) runAutoDeleteDownloadsWarning(ctx context.Context, chatID int64, topicID int, days int, hoursBefore int, fullScan bool) {
	if chatID == 0 {
		return
	}

	deleteCutoff := time.Now().UTC().AddDate(0, 0, -days)
	warningCutoff := deleteCutoff.Add(time.Duration(hoursBefore) * time.Hour)
	var previousWarningCutoff time.Time
	if fullScan {
		previousWarningCutoff = deleteCutoff
	} else {
		previousWarningCutoff = warningCutoff.Add(-warningCheckInterval)
	}

	log.Printf("Auto-delete warning: checking for downloads to be cleared in %d hours (before %s)", hoursBefore, deleteCutoff.Format("2006-01-02 15:04"))

	const batchSize = 100
	offset := 0
	var downloadsToWarn []realdebrid.Download

	for {
		downloads, err := b.rdClient.GetDownloads(batchSize, offset)
		if err != nil {
			log.Printf("Auto-delete warning: failed to fetch downloads (offset=%d): %v", offset, err)
			break
		}

		if len(downloads) == 0 {
			break
		}

		for _, d := range downloads {
			if d.Generated.Before(warningCutoff) && !d.Generated.Before(previousWarningCutoff) && !d.Generated.Before(deleteCutoff) {
				downloadsToWarn = append(downloadsToWarn, d)
			}
		}

		if len(downloads) < batchSize {
			break
		}

		offset += batchSize
	}

	if len(downloadsToWarn) == 0 {
		return
	}

	const maxMessageLength = 4000
	var messages []string
	var currentBatch strings.Builder

	header := fmt.Sprintf("<b>⚠️ Download Links Scheduled for Auto-Deletion</b>\n\n"+
		"The following download links will be cleared in approximately <b>%d hours</b>.\n\n", hoursBefore)

	currentBatch.WriteString(header)
	hasItems := false

	for _, d := range downloadsToWarn {
		filename := html.EscapeString(d.Filename)
		generatedDays := int(time.Since(d.Generated).Hours() / 24)
		line := fmt.Sprintf(
			"• <code>%s</code> — <i>%s</i> (<b>%d</b> days old)\n",
			d.ID,
			filename,
			generatedDays,
		)

		if currentBatch.Len()+len(line)+100 > maxMessageLength {
			footer := fmt.Sprintf("\n<i>Batch %d of download links scheduled for deletion</i>", len(messages)+1)
			currentBatch.WriteString(footer)
			messages = append(messages, currentBatch.String())
			currentBatch.Reset()
			currentBatch.WriteString(header)
		}

		currentBatch.WriteString(line)
		hasItems = true
	}

	if hasItems {
		footer := fmt.Sprintf("\n<i>%d download link(s) scheduled for deletion</i>", len(downloadsToWarn))
		currentBatch.WriteString(footer)
		messages = append(messages, currentBatch.String())
	}

	successCount := 0
	for i, msg := range messages {
		if err := b.sendHTMLMessageWithErr(ctx, chatID, topicID, msg, 0); err != nil {
			log.Printf("Auto-delete downloads warning: failed to send batch %d/%d to chat %d: %v", i+1, len(messages), chatID, err)
		} else {
			successCount++
			log.Printf("Auto-delete downloads warning: successfully sent batch %d/%d for %d download(s) to chat %d", i+1, len(messages), len(downloadsToWarn), chatID)
		}
	}

	if successCount == len(messages) {
		log.Printf("Auto-delete downloads warning: successfully sent all %d batch(es) for %d download(s) to chat %d", len(messages), len(downloadsToWarn), chatID)
	} else {
		log.Printf("Auto-delete downloads warning: sent %d/%d batches successfully for %d download(s) to chat %d", successCount, len(messages), len(downloadsToWarn), chatID)
	}
}
