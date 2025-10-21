package bot

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

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
			"<b>Welcome to the Real-Debrid Telegram Bot</b>\n\n"+
				"This bot helps you manage your Real-Debrid torrents and hoster links.\n\n"+
				"Your Chat ID is: <code>%d</code>\n\n"+
				"Use /help to see a list of all available commands.",
			chatID,
		)

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		// Log command
		if user != nil {
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "start", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandStart, "start", messageThreadID, true, "", nil); err != nil {
				log.Printf("Warning: failed to log activity: %v", err)
			}
		}
	})
}

// handleHelpCommand handles the /help command
func (b *Bot) handleHelpCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "help")

		text := "<b>🧭 Available Commands</b>\n\n" +
			"<b>🎬 Torrent Management:</b>\n" +
			"• <code>/list</code> — List all active torrents\n" +
			"• <code>/add &lt;magnet&gt;</code> — Add a new torrent via magnet link\n" +
			"• <code>/info &lt;id&gt;</code> — Get detailed information about a torrent\n" +
			"• <code>/delete &lt;id&gt;</code> — Delete a torrent <i>(superadmin only)</i>\n\n" +
			"<b>📦 Hoster Link Management:</b>\n" +
			"• <code>/unrestrict &lt;link&gt;</code> — Unrestrict a hoster link\n" +
			"• <code>/downloads</code> — List recent downloads\n" +
			"• <code>/removelink &lt;id&gt;</code> — Remove a download from history <i>(superadmin only)</i>\n\n" +
			"<b>⚙️ General Commands:</b>\n" +
			"• <code>/status</code> — Show your Real-Debrid account status\n" +
			"• <code>/help</code> — Display this help message"

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		// Log command
		if user != nil {
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "help", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log help command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandHelp, "help", messageThreadID, true, "", nil); err != nil {
				log.Printf("Warning: failed to log help activity: %v", err)
			}
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
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to retrieve torrents: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "list", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0); err != nil {
					log.Printf("Warning: failed to log list command error: %v", err)
				}
				if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentList, "list", messageThreadID, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log list activity error: %v", err)
				}
			}
			return
		}

		if len(torrents) == 0 {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "No torrents found.", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "list", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0); err != nil {
					log.Printf("Warning: failed to log list command success: %v", err)
				}
				if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentList, "list", messageThreadID, true, "", map[string]any{"torrent_count": 0}); err != nil {
					log.Printf("Warning: failed to log list activity success: %v", err)
				}
			}
			return
		}

		var text strings.Builder
		text.WriteString("<b>Your Recent Torrents</b>\n\n")

		maxTorrents := min(len(torrents), 10)
		const maxMsgLen = 4000
		torrentsShown := 0
		hitLengthLimit := false

		for i := range maxTorrents {
			t := torrents[i]
			entry := strings.Builder{}
			status := realdebrid.FormatStatus(t.Status)
			size := realdebrid.FormatSize(t.Bytes)
			progress := fmt.Sprintf("%.1f%%", t.Progress)
			added := t.Added.Format("2006-01-02 15:04")

			entry.WriteString(fmt.Sprintf("<i>File:</i> <code>%s</code>\n", html.EscapeString(t.Filename)))
			entry.WriteString(fmt.Sprintf("<i>ID:</i> <code>%s</code>\n", t.ID))
			entry.WriteString(fmt.Sprintf("<i>Status:</i> %s\n", status))
			entry.WriteString(fmt.Sprintf("<i>Size:</i> %s\n", size))
			entry.WriteString(fmt.Sprintf("<i>Progress:</i> %s\n", progress))
			entry.WriteString(fmt.Sprintf("<i>Added:</i> %s\n", added))

			if t.Speed > 0 {
				speed := realdebrid.FormatSize(t.Speed) + "/s"
				entry.WriteString(fmt.Sprintf("<i>Speed:</i> %s\n", speed))
			}
			if t.Seeders > 0 {
				entry.WriteString(fmt.Sprintf("<i>Seeders:</i> %d\n", t.Seeders))
			}
			entry.WriteString("\n")

			if text.Len()+entry.Len() > maxMsgLen {
				hitLengthLimit = true
				break
			}
			text.WriteString(entry.String())
			torrentsShown++
		}

		if hitLengthLimit {
			text.WriteString(fmt.Sprintf("<i>Showing the first %d torrents to avoid exceeding message length limits.</i>\n\n", torrentsShown))
		}

		text.WriteString("Use <code>/info &lt;id&gt;</code> for more details on a specific torrent.")
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text.String(), update.Message.ID)

		if user != nil {
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "list", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text.String())); err != nil {
				log.Printf("Warning: failed to log list summary command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentList, "list", messageThreadID, true, "", map[string]any{"torrent_count": len(torrents)}); err != nil {
				log.Printf("Warning: failed to log list activity summary: %v", err)
			}
		}
	})
}

// handleAddCommand handles the /add command
func (b *Bot) handleAddCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "add")

		parts := strings.Fields(update.Message.Text)
		if len(parts) < 2 {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>Usage:</b> /add &lt;magnet_link&gt;", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0); err != nil {
					log.Printf("Warning: failed to log add command missing args: %v", err)
				}
			}
			return
		}

		magnetLink := strings.Join(parts[1:], " ")
		if !strings.HasPrefix(magnetLink, "magnet:?") {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>[ERROR]</b> Invalid magnet link provided.", update.Message.ID)
			if user != nil {
				if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, "", "", "", magnetLink, "add", "", 0, 0, false, "Invalid magnet link", nil); err != nil {
					log.Printf("Warning: failed to log invalid magnet: %v", err)
				}
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Invalid magnet link", 0); err != nil {
					log.Printf("Warning: failed to log add command invalid magnet: %v", err)
				}
			}
			return
		}

		response, err := b.rdClient.AddMagnet(magnetLink)
		if err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to add torrent: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, "", "", "", magnetLink, "add", "error", 0, 0, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log torrent error: %v", err)
				}
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0); err != nil {
					log.Printf("Warning: failed to log add error command: %v", err)
				}
			}
			return
		}

		if err := b.rdClient.SelectAllFiles(response.ID); err != nil {
			log.Printf("Error selecting files for torrent %s: %v", response.ID, err)
		}

		text := fmt.Sprintf(
			"<b>Torrent Added Successfully</b>\n\n"+
				"<i>ID:</i> <code>%s</code>\n\n"+
				"Use <code>/info %s</code> to check its status.",
			response.ID, response.ID,
		)
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		if user != nil {
			if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, response.ID, "", "", magnetLink, "add", "waiting_files_selection", 0, 0, true, "", nil); err != nil {
				log.Printf("Warning: failed to log torrent activity: %v", err)
			}
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "add", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log add success command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentAdd, "add", messageThreadID, true, "", map[string]any{"torrent_id": response.ID}); err != nil {
				log.Printf("Warning: failed to log torrent add activity: %v", err)
			}
		}
	})
}

// handleInfoCommand handles the /info command
func (b *Bot) handleInfoCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "info")

		parts := strings.Fields(update.Message.Text)
		if len(parts) < 2 {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>Usage:</b> /info &lt;torrent_id&gt;", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "info", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0); err != nil {
					log.Printf("Warning: failed to log info missing args: %v", err)
				}
			}
			return
		}
		torrentID := parts[1]
		b.sendTorrentInfo(ctx, chatID, messageThreadID, torrentID, user, update.Message.ID)

		if user != nil {
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentInfo, "info", messageThreadID, true, "", map[string]any{"torrent_id": torrentID}); err != nil {
				log.Printf("Warning: failed to log torrent info activity: %v", err)
			}
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "info", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0); err != nil {
				log.Printf("Warning: failed to log info command success: %v", err)
			} // Response length logged in sendTorrentInfo
		}
	})
}

// sendTorrentInfo sends detailed torrent information
func (b *Bot) sendTorrentInfo(ctx context.Context, chatID int64, messageThreadID int, torrentID string, user *db.User, messageID int) {
	torrent, err := b.rdClient.GetTorrentInfo(torrentID)
	if err != nil {
		text := fmt.Sprintf("<b>[ERROR]</b> Could not retrieve torrent info: %s", html.EscapeString(err.Error()))
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, messageID)
		if user != nil {
			if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "info", "error", 0, 0, false, err.Error(), nil); err != nil {
				log.Printf("Warning: failed to log torrent info error: %v", err)
			}
		}
		return
	}

	status := realdebrid.FormatStatus(torrent.Status)
	size := realdebrid.FormatSize(torrent.Bytes)
	progress := fmt.Sprintf("%.1f%%", torrent.Progress)

	var text strings.Builder
	text.WriteString("<b>Torrent Details</b>\n\n")
	text.WriteString(fmt.Sprintf("<i>Name:</i> <code>%s</code>\n", html.EscapeString(torrent.Filename)))
	text.WriteString(fmt.Sprintf("<i>ID:</i> <code>%s</code>\n", torrent.ID))
	text.WriteString(fmt.Sprintf("<i>Status:</i> %s\n", status))
	text.WriteString(fmt.Sprintf("<i>Size:</i> %s\n", size))
	text.WriteString(fmt.Sprintf("<i>Progress:</i> %s\n", progress))
	text.WriteString(fmt.Sprintf("<i>Hash:</i> <code>%s</code>\n", torrent.Hash))

	if torrent.Speed > 0 {
		speed := realdebrid.FormatSize(torrent.Speed) + "/s"
		text.WriteString(fmt.Sprintf("<i>Speed:</i> %s\n", speed))
	}
	if torrent.Seeders > 0 {
		text.WriteString(fmt.Sprintf("<i>Seeders:</i> %d\n", torrent.Seeders))
	}

	// Send message
	b.sendHTMLMessage(ctx, chatID, messageThreadID, text.String(), messageID)

	if user != nil {
		if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, torrent.Hash, torrent.Filename, "", "info", torrent.Status, torrent.Bytes, torrent.Progress, true, "", nil); err != nil {
			log.Printf("Warning: failed to log torrent info success: %v", err)
		}
	}
}

// handleDeleteCommand handles the /delete command
func (b *Bot) handleDeleteCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "delete")

		if !isSuperAdmin {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>[ERROR]</b> Access Denied. This command is for superadmins only.", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Unauthorized - not superadmin", 0); err != nil {
					log.Printf("Warning: failed to log unauthorized delete command: %v", err)
				}
			}
			return
		}

		parts := strings.Fields(update.Message.Text)
		if len(parts) < 2 {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>Usage:</b> /delete &lt;torrent_id&gt;", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0); err != nil {
					log.Printf("Warning: failed to log delete missing args: %v", err)
				}
			}
			return
		}

		torrentID := parts[1]
		if err := b.rdClient.DeleteTorrent(torrentID); err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to delete torrent: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "delete", "error", 0, 0, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log delete torrent error: %v", err)
				}
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0); err != nil {
					log.Printf("Warning: failed to log delete error command: %v", err)
				}
			}
			return
		}

		text := fmt.Sprintf("<b>[OK]</b> Torrent <code>%s</code> has been deleted successfully.", html.EscapeString(torrentID))
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		if user != nil {
			if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, torrentID, "", "", "", "delete", "deleted", 0, 0, true, "", nil); err != nil {
				log.Printf("Warning: failed to log torrent delete success: %v", err)
			}
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "delete", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log delete command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeTorrentDelete, "delete", messageThreadID, true, "", map[string]any{"torrent_id": torrentID}); err != nil {
				log.Printf("Warning: failed to log delete activity: %v", err)
			}
		}
	})
}

// handleUnrestrictCommand handles the /unrestrict command
func (b *Bot) handleUnrestrictCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "unrestrict")

		parts := strings.Fields(update.Message.Text)
		if len(parts) < 2 {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>Usage:</b> /unrestrict &lt;link&gt;", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "unrestrict", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0); err != nil {
					log.Printf("Warning: failed to log unrestrict missing argument command: %v", err)
				}
			}
			return
		}

		link := strings.Join(parts[1:], " ")
		unrestricted, err := b.rdClient.UnrestrictLink(link)
		if err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to unrestrict link: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.downloadRepo.LogDownloadActivity(user.ID, chatID, "", link, "", "", "unrestrict", 0, false, err.Error(), nil, nil); err != nil {
					log.Printf("Warning: failed to log download unrestrict error: %v", err)
				}
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "unrestrict", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0); err != nil {
					log.Printf("Warning: failed to log unrestrict error command: %v", err)
				}
			}
			return
		}

		size := realdebrid.FormatSize(unrestricted.Filesize)
		text := fmt.Sprintf(
			"<b>Link Unrestricted Successfully</b>\n\n"+
				"<i>File:</i> <code>%s</code>\n"+
				"<i>Size:</i> %s\n"+
				"<i>Host:</i> %s",
			html.EscapeString(unrestricted.Filename),
			size,
			html.EscapeString(unrestricted.Host),
		)
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		if user != nil {
			if err := b.downloadRepo.LogDownloadActivity(user.ID, chatID, unrestricted.ID, link, unrestricted.Filename, unrestricted.Host, "unrestrict", unrestricted.Filesize, true, "", nil, nil); err != nil {
				log.Printf("Warning: failed to log successful unrestrict download: %v", err)
			}
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "unrestrict", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log unrestrict success command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadUnrestrict, "unrestrict", messageThreadID, true, "", map[string]any{"download_id": unrestricted.ID, "filename": unrestricted.Filename}); err != nil {
				log.Printf("Warning: failed to log download unrestrict activity: %v", err)
			}
		}
	})
}

// handleDownloadsCommand handles the /downloads command
func (b *Bot) handleDownloadsCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "downloads")

		downloads, err := b.rdClient.GetDownloads(10, 0)
		if err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to retrieve downloads: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "downloads", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0); err != nil {
					log.Printf("Warning: failed to log downloads error command: %v", err)
				}
				if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadList, "downloads", messageThreadID, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log downloads activity error: %v", err)
				}
			}
			return
		}

		if len(downloads) == 0 {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "No recent downloads found.", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "downloads", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", 0); err != nil {
					log.Printf("Warning: failed to log downloads no-results command: %v", err)
				}
				if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadList, "downloads", messageThreadID, true, "", map[string]any{"download_count": 0}); err != nil {
					log.Printf("Warning: failed to log downloads activity empty success: %v", err)
				}
			}
			return
		}

		var text strings.Builder
		text.WriteString("<b>Recent Downloads</b>\n\n")

		const maxMsgLen = 4000
		downloadsShown := 0

		for _, d := range downloads {
			entry := strings.Builder{}
			size := realdebrid.FormatSize(d.Filesize)
			entry.WriteString(fmt.Sprintf("<i>File:</i> <code>%s</code>\n", html.EscapeString(d.Filename)))
			entry.WriteString(fmt.Sprintf("<i>ID:</i> <code>%s</code>\n", d.ID))
			entry.WriteString(fmt.Sprintf("<i>Size:</i> %s\n", size))
			entry.WriteString(fmt.Sprintf("<i>Host:</i> %s\n", html.EscapeString(d.Host)))
			if !d.Generated.IsZero() {
				entry.WriteString(fmt.Sprintf("<i>Generated:</i> %s\n", d.Generated.Format("2006-01-02 15:04")))
			}
			entry.WriteString("\n")

			if text.Len()+entry.Len() > maxMsgLen {
				text.WriteString(fmt.Sprintf("<i>Showing the first %d downloads to avoid exceeding message limits.</i>\n\n", downloadsShown))
				break
			}
			text.WriteString(entry.String())
			downloadsShown++
		}

		text.WriteString("Use <code>/removelink &lt;id&gt;</code> to remove an item from this list.")
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text.String(), update.Message.ID)

		if user != nil {
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "downloads", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text.String())); err != nil {
				log.Printf("Warning: failed to log downloads success command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadList, "downloads", messageThreadID, true, "", map[string]any{"download_count": len(downloads)}); err != nil {
				log.Printf("Warning: failed to log downloads activity success: %v", err)
			}
		}
	})
}

// handleRemoveLinkCommand handles the /removelink command
func (b *Bot) handleRemoveLinkCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "removelink")

		if !isSuperAdmin {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>[ERROR]</b> Access Denied. This command is for superadmins only.", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Unauthorized - not superadmin", 0); err != nil {
					log.Printf("Warning: failed to log unauthorized removelink command: %v", err)
				}
			}
			return
		}

		parts := strings.Fields(update.Message.Text)
		if len(parts) < 2 {
			b.sendHTMLMessage(ctx, chatID, messageThreadID, "<b>Usage:</b> /removelink &lt;download_id&gt;", update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, "Missing arguments", 0); err != nil {
					log.Printf("Warning: failed to log removelink missing args: %v", err)
				}
			}
			return
		}

		downloadID := parts[1]
		if err := b.rdClient.DeleteDownload(downloadID); err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to remove download: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.downloadRepo.LogDownloadActivity(user.ID, chatID, downloadID, "", "", "", "delete", 0, false, err.Error(), nil, nil); err != nil {
					log.Printf("Warning: failed to log remove download error: %v", err)
				}
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0); err != nil {
					log.Printf("Warning: failed to log removelink error command: %v", err)
				}
			}
			return
		}

		text := fmt.Sprintf("<b>[OK]</b> Download <code>%s</code> removed from history.", html.EscapeString(downloadID))
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		if user != nil {
			if err := b.downloadRepo.LogDownloadActivity(user.ID, chatID, downloadID, "", "", "", "delete", 0, true, "", nil, nil); err != nil {
				log.Printf("Warning: failed to log delete download success: %v", err)
			}
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "removelink", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log removelink success command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeDownloadDelete, "removelink", messageThreadID, true, "", map[string]any{"download_id": downloadID}); err != nil {
				log.Printf("Warning: failed to log download delete activity: %v", err)
			}
		}
	})
}

// handleStatusCommand handles the /status command
func (b *Bot) handleStatusCommand(ctx context.Context, tgBot *bot.Bot, update *models.Update) {
	b.withAuth(ctx, update, func(ctx context.Context, chatID int64, messageThreadID int, isSuperAdmin bool, user *db.User) {
		startTime := time.Now()
		b.middleware.LogCommand(update, "status")

		rdUser, err := b.rdClient.GetUser()
		if err != nil {
			text := fmt.Sprintf("<b>[ERROR]</b> Could not retrieve account status: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "status", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), false, err.Error(), 0); err != nil {
					log.Printf("Warning: failed to log status command error: %v", err)
				}
				if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandStatus, "status", messageThreadID, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log status command activity error: %v", err)
				}
			}
			return
		}

		var text strings.Builder
		text.WriteString("<b>Account Status</b>\n\n")
		text.WriteString(fmt.Sprintf("<i>Username:</i> <code>%s</code>\n", html.EscapeString(b.maskUsername(rdUser.Username))))
		text.WriteString(fmt.Sprintf("<i>Email:</i> <code>%s</code>\n", html.EscapeString(rdUser.Email)))
		text.WriteString(fmt.Sprintf("<i>Account Type:</i> %s\n", html.EscapeString(cases.Title(language.English).String(rdUser.Type))))

		if rdUser.Points > 0 {
			text.WriteString(fmt.Sprintf("<i>Fidelity Points:</i> %d\n", rdUser.Points))
		}

		if rdUser.Premium > 0 {
			duration := rdUser.GetPremiumDuration()
			days := int(duration.Hours() / 24)
			hours := int(duration.Hours()) % 24
			text.WriteString(fmt.Sprintf("<i>Premium Remaining:</i> %d days, %d hours\n", days, hours))
		}

		if expTime, err := rdUser.GetExpirationTime(); err == nil && !expTime.IsZero() {
			text.WriteString(fmt.Sprintf("<i>Expires On:</i> %s\n", expTime.Local().Format("2006-01-02 15:04 MST")))
		}

		b.sendHTMLMessage(ctx, chatID, messageThreadID, text.String(), update.Message.ID)

		if user != nil {
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "status", update.Message.Text, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text.String())); err != nil {
				log.Printf("Warning: failed to log status command success: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeCommandStatus, "status", messageThreadID, true, "", nil); err != nil {
				log.Printf("Warning: failed to log status command success activity: %v", err)
			}
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
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to add torrent: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, "", "", "", magnetLink, "add", "error", 0, 0, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log magnet link error: %v", err)
				}
				if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeMagnetLink, "magnet_link", messageThreadID, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log magnet link activity error: %v", err)
				}
			}
			return
		}

		if err := b.rdClient.SelectAllFiles(response.ID); err != nil {
			log.Printf("Error selecting files for torrent %s: %v", response.ID, err)
		}

		text := fmt.Sprintf(
			"<b>Torrent Added Successfully</b>\n\n"+
				"<i>ID:</i> <code>%s</code>\n\n"+
				"Use <code>/info %s</code> to check its status.",
			response.ID, response.ID,
		)
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		if user != nil {
			if err := b.torrentRepo.LogTorrentActivity(user.ID, chatID, response.ID, "", "", magnetLink, "add", "waiting_files_selection", 0, 0, true, "", nil); err != nil {
				log.Printf("Warning: failed to log magnet link success: %v", err)
			}
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "magnet_link", magnetLink, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log magnet link command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeMagnetLink, "magnet_link", messageThreadID, true, "", map[string]any{"torrent_id": response.ID}); err != nil {
				log.Printf("Warning: failed to log magnet link success activity: %v", err)
			}
		}
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
			text := fmt.Sprintf("<b>[ERROR]</b> Failed to unrestrict link: %s", html.EscapeString(err.Error()))
			b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)
			if user != nil {
				if err := b.downloadRepo.LogDownloadActivity(user.ID, chatID, "", link, "", "", "unrestrict", 0, false, err.Error(), nil, nil); err != nil {
					log.Printf("Warning: failed to log hoster unrestrict error: %v", err)
				}
				if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeHosterLink, "hoster_link", messageThreadID, false, err.Error(), nil); err != nil {
					log.Printf("Warning: failed to log hoster link activity error: %v", err)
				}
			}
			return
		}

		size := realdebrid.FormatSize(unrestricted.Filesize)
		text := fmt.Sprintf(
			"<b>Link Unrestricted Successfully</b>\n\n"+
				"<i>File:</i> <code>%s</code>\n"+
				"<i>Size:</i> %s\n"+
				"<i>Host:</i> %s",
			html.EscapeString(unrestricted.Filename),
			size,
			html.EscapeString(unrestricted.Host),
		)
		b.sendHTMLMessage(ctx, chatID, messageThreadID, text, update.Message.ID)

		if user != nil {
			if err := b.downloadRepo.LogDownloadActivity(user.ID, chatID, unrestricted.ID, link, unrestricted.Filename, unrestricted.Host, "unrestrict", unrestricted.Filesize, true, "", nil, nil); err != nil {
				log.Printf("Warning: failed to log hoster unrestrict success: %v", err)
			}
			if err := b.commandRepo.LogCommand(user.ID, chatID, user.Username, "hoster_link", link, messageThreadID, time.Since(startTime).Milliseconds(), true, "", len(text)); err != nil {
				log.Printf("Warning: failed to log hoster link command: %v", err)
			}
			if err := b.activityRepo.LogActivity(user.ID, chatID, user.Username, db.ActivityTypeHosterLink, "hoster_link", messageThreadID, true, "", map[string]any{"download_id": unrestricted.ID, "filename": unrestricted.Filename}); err != nil {
				log.Printf("Warning: failed to log hoster link activity success: %v", err)
			}
		}
	})
}

// --- Helper Functions ---

func (b *Bot) sendHTMLMessage(ctx context.Context, chatID int64, messageThreadID int, text string, replyToMessageID int) {
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if messageThreadID != 0 {
		params.MessageThreadID = messageThreadID
	}
	if replyToMessageID != 0 {
		params.ReplyParameters = &models.ReplyParameters{
			MessageID: replyToMessageID,
		}
	}
	if err := b.middleware.WaitForRateLimit(); err != nil {
		log.Printf("Rate limit error: %v", err)
	}
	if _, err := b.api.SendMessage(ctx, params); err != nil {
		log.Printf("Error sending HTML message: %v", err)
	}
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
