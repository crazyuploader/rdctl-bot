package bot

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleStartCommand handles the /start command
func (b *Bot) handleStartCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "start")

		text := fmt.Sprintf(
			"<b>Real-Debrid Telegram Bot</b>\n\n"+
				"Welcome! This bot helps you manage Real-Debrid torrents and hoster links.\n\n"+
				"Your Chat ID: <code>%d</code>\n\n"+
				"Use /help to see all available commands.",
			chatID,
		)

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text)

		// Log command
		if user != nil {
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "start", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text))
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandStart, "start", messageThreadID, true, "", nil)
		}
	})
}

// handleHelpCommand handles the /help command
func (b *Bot) handleHelpCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "help")

		text := "<b>Available Commands:</b>\n\n" +
			"<b>Torrent Management:</b>\n" +
			"/list - List all torrents\n" +
			"/add &lt;magnet&gt; - Add magnet link\n" +
			"/info &lt;id&gt; - Get torrent details\n" +
			"/delete &lt;id&gt; - Delete torrent (superadmin only)\n\n" +
			"<b>Hoster Links:</b>\n" +
			"/unrestrict &lt;link&gt; - Unrestrict hoster link\n" +
			"/downloads - List recent downloads\n" +
			"/removelink &lt;id&gt; - Remove download from history (superadmin only)\n\n" +
			"<b>General:</b>\n" +
			"/status - Show account status\n" +
			"/help - Show this help message"

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text)

		// Log command
		if user != nil {
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "help", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text))
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandHelp, "help", messageThreadID, true, "", nil)
		}
	})
}

// handleListCommand handles the /list command
func (b *Bot) handleListCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "list")

		torrents, err := b.rdClient.GetTorrents(10, 0)
		if err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log error
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "list", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0)
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentList, "list", messageThreadID, false, err.Error(), nil)
			}
			return
		}

		if len(torrents) == 0 {
			b.sendMessage(ctx, chatID, messageThreadID, "No torrents found.")

			// Log command
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "list", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentList, "list", messageThreadID, true, "", map[string]interface{}{"torrent_count": 0})
			}
			return
		}

		var text strings.Builder
		text.WriteString("List of Added Torrents:\n\n")

		maxTorrents := 10
		if len(torrents) < maxTorrents {
			maxTorrents = len(torrents)
		}

		const maxMsgLen = 4000
		torrentsShown := 0
		hitLengthLimit := false

		for i := 0; i < maxTorrents; i++ {
			t := torrents[i]
			entry := strings.Builder{}
			status := asciiStatus(t.Status)
			size := realdebrid.FormatSize(t.Bytes)
			progress := fmt.Sprintf("%.1f%%", t.Progress)
			added := t.Added.Format("2006-01-02 15:04")

			entry.WriteString(fmt.Sprintf(
				"• <i>Torrent name:</i> <code>%s</code>\n"+
					"  <i>ID:</i> <code>%s</code>\n"+
					"  <i>Status:</i> %s\n"+
					"  <i>Size:</i> %s\n"+
					"  <i>Progress:</i> %s\n"+
					"  <i>Added on:</i> %s\n"+
					"  <i>Hash:</i> <code>%s</code>\n",
				html.EscapeString(t.Filename), t.ID, status, size, progress, added, t.Hash,
			))

			if t.Seeders > 0 {
				entry.WriteString(fmt.Sprintf("  <i>Seeders:</i> %d\n", t.Seeders))
			}
			if t.Speed > 0 {
				speed := realdebrid.FormatSize(t.Speed) + "/s"
				entry.WriteString(fmt.Sprintf("  <i>Speed:</i> %s\n", speed))
			}
			if len(t.Files) > 0 {
				entry.WriteString(fmt.Sprintf("  <i>Files:</i> %d\n", len(t.Files)))
			}
			if t.Ended != nil && !t.Ended.IsZero() {
				entry.WriteString(fmt.Sprintf("  <i>Finished downloading on:</i> %s\n", t.Ended.Format("2006-01-02 15:04")))
			}
			entry.WriteString("\n")

			if text.Len()+entry.Len()+300 > maxMsgLen {
				hitLengthLimit = true
				break
			}
			text.WriteString(entry.String())
			torrentsShown++
		}

		if hitLengthLimit {
			text.WriteString(fmt.Sprintf("<i>Only showing the first %d torrents due to message length limit.</i>\n\n", torrentsShown))
		} else if len(torrents) > maxTorrents {
			text.WriteString(fmt.Sprintf("<i>Only showing the first %d torrents.</i>\n\n", maxTorrents))
		}

		text.WriteString("Use /info &lt;id&gt; for more details")

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text.String())

		// Log command
		if user != nil {
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "list", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text.String()))
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentList, "list", messageThreadID, true, "", map[string]interface{}{"torrent_count": len(torrents)})
		}
	})
}

// handleAddCommand handles the /add command
func (b *Bot) handleAddCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "add")

		args := strings.Fields(strings.TrimPrefix(update.Message.Text, "/add "))
		if len(args) == 0 {
			b.sendMessage(ctx, chatID, messageThreadID, "Usage: /add <magnet_link>")

			// Log failed command
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0)
			}
			return
		}

		magnetLink := strings.Join(args, " ")
		if !strings.HasPrefix(magnetLink, "magnet:?") {
			b.sendMessage(ctx, chatID, messageThreadID, "[Error] Invalid magnet link")

			// Log failed activity
			if user != nil {
				b.torrentRepo.LogTorrentActivity(user.ID, chatID, "", "", "", magnetLink, "add", "", 0, 0, false, "Invalid magnet link", nil)
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Invalid magnet link", 0)
			}
			return
		}

		response, err := b.rdClient.AddMagnet(magnetLink)
		if err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log failed activity
			if user != nil {
				b.torrentRepo.LogTorrentActivity(user.ID, chatID, "", "", "", magnetLink, "add", "error", 0, 0, false, err.Error(), nil)
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0)
			}
			return
		}

		if err := b.rdClient.SelectAllFiles(response.ID); err != nil {
			log.Printf("Error selecting files: %v", err)
		}

		// Log successful activity
		if user != nil {
			b.torrentRepo.LogTorrentActivity(user.ID, chatID, response.ID, "", "", magnetLink, "add", "waiting_files_selection", 0, 0, true, "", nil)
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentAdd, "add", messageThreadID, true, "", map[string]interface{}{"torrent_id": response.ID})
		}

		text := fmt.Sprintf(
			"[OK] <b>Torrent Added</b>\n\n"+
				"ID: <code>%s</code>\n\n"+
				"Use /info %s to check status",
			response.ID, response.ID,
		)

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text)
	})
}

// handleInfoCommand handles the /info command
func (b *Bot) handleInfoCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "info")

		cmdArgs := strings.TrimPrefix(update.Message.Text, "/info")
		args := strings.Fields(strings.TrimSpace(cmdArgs))
		if len(args) == 0 {
			b.sendMessage(ctx, chatID, messageThreadID, "Usage: /info <torrent_id>")

			// Log failed command
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "info", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0)
			}
			return
		}

		torrentID := args[0]

		// Log activity
		if user != nil {
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentInfo, "info", messageThreadID, true, "", map[string]interface{}{"torrent_id": torrentID})
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "info", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
		}

		b.sendTorrentInfo(ctx, chatID, messageThreadID, torrentID, user)
	})
}

// sendTorrentInfo sends detailed torrent information
func (b *Bot) sendTorrentInfo(ctx context.Context, chatID int64, messageThreadID int, torrentID string, user *db.User) {
	torrent, err := b.rdClient.GetTorrentInfo(torrentID)
	if err != nil {
		b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

		// Log error
		if user != nil {
			b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "info", "error", 0, 0, false, err.Error(), nil)
		}
		return
	}

	// Log successful info retrieval
	if user != nil {
		b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, torrent.Hash, torrent.Filename, "", "info", torrent.Status, torrent.Bytes, torrent.Progress, true, "", nil)
	}

	status := realdebrid.FormatStatus(torrent.Status)
	size := realdebrid.FormatSize(torrent.Bytes)
	progress := fmt.Sprintf("%.1f%%", torrent.Progress)

	var text strings.Builder
	text.WriteString("<b>Torrent Details:</b>\n\n")
	text.WriteString(fmt.Sprintf("<b>Name:</b> <code>%s</code>\n", html.EscapeString(torrent.Filename)))
	text.WriteString(fmt.Sprintf("<b>ID:</b> <code>%s</code>\n", torrent.ID))
	text.WriteString(fmt.Sprintf("<b>Status:</b> %s\n", status))
	text.WriteString(fmt.Sprintf("<b>Size:</b> %s\n", size))
	text.WriteString(fmt.Sprintf("<b>Progress:</b> %s\n", progress))
	text.WriteString(fmt.Sprintf("<b>Hash:</b> <code>%s</code>\n", torrent.Hash))

	if torrent.Speed > 0 {
		speed := realdebrid.FormatSize(torrent.Speed) + "/s"
		text.WriteString(fmt.Sprintf("<b>Speed:</b> %s\n", speed))
	}

	if torrent.Seeders > 0 {
		text.WriteString(fmt.Sprintf("<b>Seeders:</b> %d\n", torrent.Seeders))
	}

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Refresh", CallbackData: fmt.Sprintf("refresh_%s", torrentID)},
				{Text: "Delete", CallbackData: fmt.Sprintf("delete_%s", torrentID)},
			},
		},
	}

	params := &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text.String(),
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: keyboard,
	}

	if messageThreadID != 0 {
		params.MessageThreadID = messageThreadID
	}

	if err := b.middleware.WaitForRateLimit(); err != nil {
		log.Printf("Rate limit error: %v", err)
	}

	if _, err := b.api.SendMessage(ctx, params); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// handleDeleteCommand handles the /delete command
func (b *Bot) handleDeleteCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "delete")

		if !isSuperAdmin {
			b.sendMessage(ctx, chatID, messageThreadID, "[Error] This command is restricted to superadmins only")

			// Log unauthorized attempt
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Unauthorized - not superadmin", 0)
			}
			return
		}

		cmdText := strings.TrimPrefix(update.Message.Text, "/delete")
		args := strings.Fields(strings.TrimSpace(cmdText))
		if len(args) == 0 {
			b.sendMessage(ctx, chatID, messageThreadID, "Usage: /delete <torrent_id>")

			// Log failed command
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0)
			}
			return
		}

		torrentID := args[0]
		if err := b.rdClient.DeleteTorrent(torrentID); err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log failed activity
			if user != nil {
				b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "delete", "error", 0, 0, false, err.Error(), nil)
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0)
			}
			return
		}

		// Log successful activity
		if user != nil {
			b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "delete", "deleted", 0, 0, true, "", nil)
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentDelete, "delete", messageThreadID, true, "", map[string]interface{}{"torrent_id": torrentID})
		}

		b.sendMessage(ctx, chatID, messageThreadID, "[OK] Torrent deleted successfully")
	})
}

// handleUnrestrictCommand handles the /unrestrict command
func (b *Bot) handleUnrestrictCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "unrestrict")

		cmdArgs := strings.TrimPrefix(update.Message.Text, "/unrestrict")
		args := strings.Fields(strings.TrimSpace(cmdArgs))
		if len(args) == 0 {
			b.sendMessage(ctx, chatID, messageThreadID, "Usage: /unrestrict <link>")

			// Log failed command
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "unrestrict", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0)
			}
			return
		}

		link := strings.Join(args, " ")
		unrestricted, err := b.rdClient.UnrestrictLink(link)
		if err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log failed activity
			if user != nil {
				b.downloadRepo.LogDownloadActivity(user.ID, chatID, "", link, "", "", "unrestrict", 0, false, err.Error(), nil)
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "unrestrict", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0)
			}
			return
		}

		// Log successful activity
		if user != nil {
			b.downloadRepo.LogDownloadActivity(user.ID, chatID, unrestricted.ID, link, unrestricted.Filename, unrestricted.Host, "unrestrict", unrestricted.Filesize, true, "", nil)
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "unrestrict", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadUnrestrict, "unrestrict", messageThreadID, true, "", map[string]interface{}{"download_id": unrestricted.ID, "filename": unrestricted.Filename})
		}

		size := realdebrid.FormatSize(unrestricted.Filesize)
		text := fmt.Sprintf(
			"[OK] <b>Link Unrestricted</b>\n\n"+
				"<b>File:</b> <code>%s</code>\n"+
				"<b>Size:</b> %s\n"+
				"<b>Host:</b> %s",
			html.EscapeString(unrestricted.Filename),
			size,
			html.EscapeString(unrestricted.Host),
		)

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text)
	})
}

// handleDownloadsCommand handles the /downloads command
func (b *Bot) handleDownloadsCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "downloads")

		downloads, err := b.rdClient.GetDownloads(10, 0)
		if err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log error
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "downloads", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0)
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadList, "downloads", messageThreadID, false, err.Error(), nil)
			}
			return
		}

		if len(downloads) == 0 {
			b.sendMessage(ctx, chatID, messageThreadID, "No downloads found.")

			// Log command
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "downloads", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadList, "downloads", messageThreadID, true, "", map[string]interface{}{"download_count": 0})
			}
			return
		}

		var text strings.Builder
		text.WriteString("Recent Downloads:\n\n")

		const maxMsgLen = 4000
		downloadsShown := 0

		for _, d := range downloads {
			entry := strings.Builder{}
			size := realdebrid.FormatSize(d.Filesize)
			entry.WriteString(fmt.Sprintf(
				"• <i>File name:</i> <code>%s</code>\n"+
					"  <i>ID:</i> <code>%s</code>\n",
				html.EscapeString(d.Filename), d.ID,
			))
			if d.MimeType != "" {
				entry.WriteString(fmt.Sprintf("  <i>Mime type:</i> %s\n", html.EscapeString(d.MimeType)))
			}
			entry.WriteString(fmt.Sprintf("  <i>Size:</i> %s\n", size))
			entry.WriteString(fmt.Sprintf("  <i>Host:</i> %s\n", html.EscapeString(d.Host)))
			if d.Type != "" {
				entry.WriteString(fmt.Sprintf("  <i>Type:</i> %s\n", html.EscapeString(d.Type)))
			}
			if d.Chunks > 0 {
				entry.WriteString(fmt.Sprintf("  <i>Chunks:</i> %d\n", d.Chunks))
			}
			if !d.Generated.IsZero() {
				entry.WriteString(fmt.Sprintf("  <i>Generated on:</i> %s\n", d.Generated.Format("2006-01-02 15:04")))
			}
			entry.WriteString("\n")

			if text.Len()+entry.Len()+300 > maxMsgLen {
				text.WriteString(fmt.Sprintf("<i>Only showing the first %d downloads due to message length limit.</i>\n\n", downloadsShown))
				break
			}
			text.WriteString(entry.String())
			downloadsShown++
		}

		text.WriteString("Use /removelink &lt;id&gt; to remove from history")

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text.String())

		// Log command
		if user != nil {
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "downloads", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text.String()))
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadList, "downloads", messageThreadID, true, "", map[string]interface{}{"download_count": len(downloads)})
		}
	})
}

// handleRemoveLinkCommand handles the /removelink command
func (b *Bot) handleRemoveLinkCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "removelink")

		if !isSuperAdmin {
			b.sendMessage(ctx, chatID, messageThreadID, "[Error] This command is restricted to superadmins only")

			// Log unauthorized attempt
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Unauthorized - not superadmin", 0)
			}
			return
		}

		cmdArgs := strings.TrimPrefix(update.Message.Text, "/removelink")
		args := strings.Fields(strings.TrimSpace(cmdArgs))
		if len(args) == 0 {
			b.sendMessage(ctx, chatID, messageThreadID, "Usage: /removelink <download_id>")

			// Log failed command
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0)
			}
			return
		}

		downloadID := args[0]
		if err := b.rdClient.DeleteDownload(downloadID); err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log failed activity
			if user != nil {
				b.downloadRepo.LogDownloadActivity(user.ID, chatID, downloadID, "", "", "", "delete", 0, false, err.Error(), nil)
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0)
			}
			return
		}

		// Log successful activity
		if user != nil {
			b.downloadRepo.LogDownloadActivity(user.ID, chatID, downloadID, "", "", "", "delete", 0, true, "", nil)
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadDelete, "removelink", messageThreadID, true, "", map[string]interface{}{"download_id": downloadID})
		}

		b.sendMessage(ctx, chatID, messageThreadID, "[OK] Download removed from history")
	})
}

// handleStatusCommand handles the /status command
func (b *Bot) handleStatusCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "status")

		rdUser, err := b.rdClient.GetUser()
		if err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log error
			if user != nil {
				b.commandRepo.LogCommand(user.ID, chatID, user.Username, "status", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0)
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandStatus, "status", messageThreadID, false, err.Error(), nil)
			}
			return
		}

		var text strings.Builder
		text.WriteString("<b>Account Status:</b>\n\n")

		maskedUsername := maskString(rdUser.Username, 5)
		text.WriteString(fmt.Sprintf("<b>Username:</b> <code>%s</code>\n", html.EscapeString(maskedUsername)))
		text.WriteString(fmt.Sprintf("<b>Email:</b> <code>%s</code>\n", html.EscapeString(rdUser.Email)))
		text.WriteString(fmt.Sprintf("<b>Account Type:</b> %s\n", html.EscapeString(rdUser.Type)))

		if rdUser.Points > 0 {
			text.WriteString(fmt.Sprintf("<b>Fidelity Points:</b> %d\n", rdUser.Points))
		}

		if rdUser.Premium > 0 {
			duration := rdUser.GetPremiumDuration()
			days := int(duration.Hours() / 24)
			hours := int(duration.Hours()) % 24
			text.WriteString(fmt.Sprintf("<b>Premium Remaining:</b> %d days, %d hours\n", days, hours))
		}

		if rdUser.Expiration != "" {
			expTime, err := rdUser.GetExpirationTime()
			if err == nil {
				text.WriteString(fmt.Sprintf("<b>Expiration:</b> %s\n", expTime.Format("2006-01-02 15:04")))
			}
		}

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text.String())

		// Log command
		if user != nil {
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "status", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text.String()))
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandStatus, "status", messageThreadID, true, "", nil)
		}
	})
}

// handleMagnetLink handles magnet links sent as messages
func (b *Bot) handleMagnetLink(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "magnet_link")

		magnetLink := update.Message.Text
		response, err := b.rdClient.AddMagnet(magnetLink)
		if err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log failed activity
			if user != nil {
				b.torrentRepo.LogTorrentActivity(user.ID, chatID, "", "", "", magnetLink, "add", "error", 0, 0, false, err.Error(), nil)
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeMagnetLink, "magnet_link", messageThreadID, false, err.Error(), nil)
			}
			return
		}

		if err := b.rdClient.SelectAllFiles(response.ID); err != nil {
			log.Printf("Error selecting files: %v", err)
		}

		// Log successful activity
		if user != nil {
			b.torrentRepo.LogTorrentActivity(user.ID, chatID, response.ID, "", "", magnetLink, "add", "waiting_files_selection", 0, 0, true, "", nil)
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "magnet_link", magnetLink, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeMagnetLink, "magnet_link", messageThreadID, true, "", map[string]interface{}{"torrent_id": response.ID})
		}

		text := fmt.Sprintf(
			"[OK] <b>Torrent Added</b>\n\n"+
				"ID: <code>%s</code>\n\n"+
				"Use /info %s to check status",
			response.ID, response.ID,
		)

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text)
	})
}

// handleHosterLink handles hoster links sent as messages
func (b *Bot) handleHosterLink(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "hoster_link")

		link := update.Message.Text
		unrestricted, err := b.rdClient.UnrestrictLink(link)
		if err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log failed activity
			if user != nil {
				b.downloadRepo.LogDownloadActivity(user.ID, chatID, "", link, "", "", "unrestrict", 0, false, err.Error(), nil)
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeHosterLink, "hoster_link", messageThreadID, false, err.Error(), nil)
			}
			return
		}

		// Log successful activity
		if user != nil {
			b.downloadRepo.LogDownloadActivity(user.ID, chatID, unrestricted.ID, link, unrestricted.Filename, unrestricted.Host, "unrestrict", unrestricted.Filesize, true, "", nil)
			b.commandRepo.LogCommand(user.ID, chatID, user.Username, "hoster_link", link, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0)
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeHosterLink, "hoster_link", messageThreadID, true, "", map[string]interface{}{"download_id": unrestricted.ID, "filename": unrestricted.Filename})
		}

		size := realdebrid.FormatSize(unrestricted.Filesize)
		text := fmt.Sprintf(
			"[OK] <b>Link Unrestricted</b>\n\n"+
				"<b>File:</b> <code>%s</code>\n"+
				"<b>Size:</b> %s\n"+
				"<b>Host:</b> %s",
			html.EscapeString(unrestricted.Filename),
			size,
			html.EscapeString(unrestricted.Host),
		)

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text)
	})
}

// handleRefreshCallback handles the refresh button callback
func (b *Bot) handleRefreshCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		b.middleware.LogCommand(update, "refresh_callback")

		// Answer callback query
		b.api.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})

		torrentID := strings.TrimPrefix(update.CallbackQuery.Data, "refresh_")

		// Log activity
		if user != nil {
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentInfo, "refresh", messageThreadID, true, "", map[string]interface{}{"torrent_id": torrentID})
		}

		b.sendTorrentInfo(ctx, chatID, messageThreadID, torrentID, user)
	})
}

// handleDeleteCallback handles the delete button callback
func (b *Bot) handleDeleteCallback(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		b.middleware.LogCommand(update, "delete_callback")

		if !isSuperAdmin {
			b.api.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "This action is restricted to superadmins only",
				ShowAlert:       true,
			})

			// Log unauthorized attempt
			if user != nil {
				b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentDelete, "delete_callback", messageThreadID, false, "Unauthorized - not superadmin", nil)
			}
			return
		}

		// Answer callback query
		b.api.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})

		torrentID := strings.TrimPrefix(update.CallbackQuery.Data, "delete_")
		if err := b.rdClient.DeleteTorrent(torrentID); err != nil {
			b.sendMessage(ctx, chatID, messageThreadID, fmt.Sprintf("[Error] %v", err))

			// Log failed activity
			if user != nil {
				b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "delete", "error", 0, 0, false, err.Error(), nil)
			}
			return
		}

		// Log successful activity
		if user != nil {
			b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "delete", "deleted", 0, 0, true, "", nil)
			b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentDelete, "delete_callback", messageThreadID, true, "", map[string]interface{}{"torrent_id": torrentID})
		}

		// Delete the original message
		if update.CallbackQuery.Message.Message != nil {
			b.api.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: update.CallbackQuery.Message.Message.ID,
			})
		}

		b.sendMessage(ctx, chatID, messageThreadID, "[OK] Torrent deleted successfully")
	})
}

// Helper functions

func asciiStatus(status string) string {
	switch status {
	case "downloaded", "Downloaded", "complete", "Complete":
		return "[Downloaded]"
	case "downloading", "Downloading":
		return "[Downloading]"
	case "error", "Error":
		return "[Error]"
	case "magnet_error", "Magnet Error":
		return "[Magnet Error]"
	case "waiting_files_selection", "Waiting Files Selection":
		return "[Waiting Files Selection]"
	case "queued", "Queued":
		return "[Queued]"
	case "compressing", "Compressing":
		return "[Compressing]"
	case "uploading", "Uploading":
		return "[Uploading]"
	default:
		return "[" + status + "]"
	}
}

func (b *Bot) sendMessage(ctx context.Context, chatID int64, messageThreadID int, text string) {
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
		log.Printf("Error sending message: %v", err)
	}
}

func (b *Bot) sendHTMLMessage(ctx context.Context, chatID int64, messageThreadID int, text string) {
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
		log.Printf("Error sending message: %v", err)
	}
}

func maskString(s string, lastChars int) string {
	if len(s) <= lastChars {
		return s
	}

	visible := s[len(s)-lastChars:]
	masked := strings.Repeat("*", len(s)-lastChars)

	return masked + visible
}
