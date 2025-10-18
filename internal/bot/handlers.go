package bot

import (
	"fmt"
	"log"
	"strings"

	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleCommand processes bot commands
func (b *Bot) handleCommand(update tgbotapi.Update, isSuperAdmin bool) {
	msg := update.Message
	command := msg.Command()
	args := strings.Fields(msg.CommandArguments())

	b.middleware.LogCommand(update, command)

	switch command {
	case "start":
		b.handleStart(msg)
	case "help":
		b.handleHelp(msg)
	case "list":
		b.handleList(msg)
	case "add":
		b.handleAddMagnet(msg, args)
	case "info":
		b.handleInfo(msg, args)
	case "delete", "del":
		b.handleDelete(msg, args)
	case "unrestrict":
		b.handleUnrestrict(msg, args)
	case "downloads":
		b.handleDownloads(msg)
	case "removelink":
		b.handleRemoveLink(msg, args)
	case "status":
		b.handleStatus(msg)
	default:
		b.sendMessage(msg.Chat.ID, "Unknown command. Use /help to see available commands.")
	}
}

// handleStart handles the /start command
func (b *Bot) handleStart(msg *tgbotapi.Message) {
	text := fmt.Sprintf(
		"🤖 *Real-Debrid Telegram Bot*\n\n"+
			"Welcome! This bot helps you manage Real-Debrid torrents and hoster links.\n\n"+
			"Your Chat ID: `%d`\n\n"+
			"Use /help to see all available commands.",
		msg.Chat.ID,
	)
	b.sendMarkdownMessage(msg.Chat.ID, text)
}

// handleHelp handles the /help command
func (b *Bot) handleHelp(msg *tgbotapi.Message) {
	text := "*Available Commands:*\n\n" +
		"*Torrent Management:*\n" +
		"/list - List all torrents\n" +
		"/add <magnet> - Add magnet link\n" +
		"/info <id> - Get torrent details\n" +
		"/delete <id> - Delete torrent\n\n" +
		"*Hoster Links:*\n" +
		"/unrestrict <link> - Unrestrict hoster link\n" +
		"/downloads - List recent downloads\n" +
		"/removelink <id> - Remove download from history\n\n" +
		"*General:*\n" +
		"/status - Show account status\n" +
		"/help - Show this help message"

	b.sendMarkdownMessage(msg.Chat.ID, text)
}

// handleList handles the /list command
func (b *Bot) handleList(msg *tgbotapi.Message) {
	torrents, err := b.rdClient.GetTorrents(20, 0)
	if err != nil {
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	if len(torrents) == 0 {
		b.sendMessage(msg.Chat.ID, "No torrents found.")
		return
	}

	var text strings.Builder
	text.WriteString("*Your Torrents:*\n\n")

	for i, t := range torrents {
		status := realdebrid.FormatStatus(t.Status)
		size := realdebrid.FormatSize(t.Bytes)
		progress := fmt.Sprintf("%.1f%%", t.Progress)

		text.WriteString(fmt.Sprintf(
			"%d. *%s*\n"+
				"   ID: `%s`\n"+
				"   Status: %s\n"+
				"   Size: %s | Progress: %s\n\n",
			i+1, escapeMD(t.Filename), t.ID, status, size, progress,
		))
	}

	text.WriteString("Use /info <id> for more details")

	b.sendMarkdownMessage(msg.Chat.ID, text.String())
}

// handleAddMagnet handles the /add command
func (b *Bot) handleAddMagnet(msg *tgbotapi.Message, args []string) {
	if len(args) == 0 {
		b.sendMessage(msg.Chat.ID, "Usage: /add <magnet_link>")
		return
	}

	magnetLink := strings.Join(args, " ")
	if !strings.HasPrefix(magnetLink, "magnet:?") {
		b.sendMessage(msg.Chat.ID, "❌ Invalid magnet link")
		return
	}

	response, err := b.rdClient.AddMagnet(magnetLink)
	if err != nil {
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	// Auto-select all files
	if err := b.rdClient.SelectAllFiles(response.ID); err != nil {
		log.Printf("Error selecting files: %v", err)
	}

	text := fmt.Sprintf(
		"✅ *Torrent Added*\n\n"+
			"ID: `%s`\n\n"+
			"Use /info %s to check status",
		response.ID, response.ID,
	)

	b.sendMarkdownMessage(msg.Chat.ID, text)
}

// handleInfo handles the /info command
func (b *Bot) handleInfo(msg *tgbotapi.Message, args []string) {
	if len(args) == 0 {
		b.sendMessage(msg.Chat.ID, "Usage: /info <torrent_id>")
		return
	}

	torrentID := args[0]
	torrent, err := b.rdClient.GetTorrentInfo(torrentID)
	if err != nil {
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	status := realdebrid.FormatStatus(torrent.Status)
	size := realdebrid.FormatSize(torrent.Bytes)
	progress := fmt.Sprintf("%.1f%%", torrent.Progress)

	var text strings.Builder
	text.WriteString("*Torrent Details:*\n\n")
	text.WriteString(fmt.Sprintf("*Name:* %s\n", escapeMD(torrent.Filename)))
	text.WriteString(fmt.Sprintf("*ID:* `%s`\n", torrent.ID))
	text.WriteString(fmt.Sprintf("*Status:* %s\n", status))
	text.WriteString(fmt.Sprintf("*Size:* %s\n", size))
	text.WriteString(fmt.Sprintf("*Progress:* %s\n", progress))
	text.WriteString(fmt.Sprintf("*Hash:* `%s`\n", torrent.Hash))

	if torrent.Speed > 0 {
		speed := realdebrid.FormatSize(torrent.Speed) + "/s"
		text.WriteString(fmt.Sprintf("*Speed:* %s\n", speed))
	}

	if torrent.Seeders > 0 {
		text.WriteString(fmt.Sprintf("*Seeders:* %d\n", torrent.Seeders))
	}

	// Show download links if available
	if len(torrent.Links) > 0 {
		text.WriteString("\n*Download Links:*\n")
		for i, link := range torrent.Links {
			if i >= 5 { // Limit to 5 links
				text.WriteString(fmt.Sprintf("... and %d more\n", len(torrent.Links)-5))
				break
			}
			text.WriteString(fmt.Sprintf("%d. %s\n", i+1, link))
		}
	}

	// Add action buttons
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("refresh_%s", torrentID)),
			tgbotapi.NewInlineKeyboardButtonData("🗑️ Delete", fmt.Sprintf("delete_%s", torrentID)),
		),
	)

	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, text.String())
	replyMsg.ParseMode = "Markdown"
	replyMsg.ReplyMarkup = keyboard

	if err := b.middleware.WaitForRateLimit(); err != nil {
		log.Printf("Rate limit error: %v", err)
	}

	if _, err := b.api.Send(replyMsg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// handleDelete handles the /delete command
func (b *Bot) handleDelete(msg *tgbotapi.Message, args []string) {
	if len(args) == 0 {
		b.sendMessage(msg.Chat.ID, "Usage: /delete <torrent_id>")
		return
	}

	torrentID := args[0]
	if err := b.rdClient.DeleteTorrent(torrentID); err != nil {
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	b.sendMessage(msg.Chat.ID, "✅ Torrent deleted successfully")
}

// handleUnrestrict handles the /unrestrict command
func (b *Bot) handleUnrestrict(msg *tgbotapi.Message, args []string) {
	if len(args) == 0 {
		b.sendMessage(msg.Chat.ID, "Usage: /unrestrict <link>")
		return
	}

	link := strings.Join(args, " ")
	unrestricted, err := b.rdClient.UnrestrictLink(link)
	if err != nil {
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	size := realdebrid.FormatSize(unrestricted.Filesize)
	text := fmt.Sprintf(
		"✅ *Link Unrestricted*\n\n"+
			"*File:* %s\n"+
			"*Size:* %s\n"+
			"*Host:* %s\n\n"+
			"*Download:* %s",
		escapeMD(unrestricted.Filename),
		size,
		unrestricted.Host,
		unrestricted.Download,
	)

	b.sendMarkdownMessage(msg.Chat.ID, text)
}

// handleDownloads handles the /downloads command
func (b *Bot) handleDownloads(msg *tgbotapi.Message) {
	downloads, err := b.rdClient.GetDownloads(10, 0)
	if err != nil {
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	if len(downloads) == 0 {
		b.sendMessage(msg.Chat.ID, "No downloads found.")
		return
	}

	var text strings.Builder
	text.WriteString("*Recent Downloads:*\n\n")

	for i, d := range downloads {
		size := realdebrid.FormatSize(d.Filesize)
		text.WriteString(fmt.Sprintf(
			"%d. *%s*\n"+
				"   ID: `%s`\n"+
				"   Size: %s | Host: %s\n\n",
			i+1, escapeMD(d.Filename), d.ID, size, d.Host,
		))
	}

	text.WriteString("Use /removelink <id> to remove from history")

	b.sendMarkdownMessage(msg.Chat.ID, text.String())
}

// handleRemoveLink handles the /removelink command
func (b *Bot) handleRemoveLink(msg *tgbotapi.Message, args []string) {
	if len(args) == 0 {
		b.sendMessage(msg.Chat.ID, "Usage: /removelink <download_id>")
		return
	}

	downloadID := args[0]
	if err := b.rdClient.DeleteDownload(downloadID); err != nil {
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
		return
	}

	b.sendMessage(msg.Chat.ID, "✅ Download removed from history")
}

// handleStatus handles the /status command
func (b *Bot) handleStatus(msg *tgbotapi.Message) {
	// This would require implementing the user endpoint in the RD client
	b.sendMessage(msg.Chat.ID, "Status check not yet implemented")
}

// handleMessage processes non-command messages
func (b *Bot) handleMessage(update tgbotapi.Update) {
	msg := update.Message

	// Check if message contains a magnet link
	if strings.Contains(msg.Text, "magnet:?") {
		args := []string{msg.Text}
		b.handleAddMagnet(msg, args)
		return
	}

	// Check if message contains a URL (potential hoster link)
	if strings.HasPrefix(msg.Text, "http://") || strings.HasPrefix(msg.Text, "https://") {
		args := []string{msg.Text}
		b.handleUnrestrict(msg, args)
		return
	}
}

// handleCallbackQuery processes inline keyboard callbacks
func (b *Bot) handleCallbackQuery(update tgbotapi.Update) {
	query := update.CallbackQuery
	data := query.Data

	// Answer callback query first
	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("Error answering callback: %v", err)
	}

	parts := strings.Split(data, "_")
	if len(parts) < 2 {
		return
	}

	action := parts[0]
	torrentID := parts[1]

	switch action {
	case "refresh":
		// Create a pseudo message for handleInfo
		pseudoMsg := &tgbotapi.Message{
			Chat: query.Message.Chat,
		}
		b.handleInfo(pseudoMsg, []string{torrentID})

	case "delete":
		if err := b.rdClient.DeleteTorrent(torrentID); err != nil {
			b.sendMessage(query.Message.Chat.ID, fmt.Sprintf("❌ Error: %v", err))
			return
		}

		// Delete the original message
		deleteMsg := tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		b.api.Send(deleteMsg)

		b.sendMessage(query.Message.Chat.ID, "✅ Torrent deleted successfully")
	}
}

// Helper functions

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	if err := b.middleware.WaitForRateLimit(); err != nil {
		log.Printf("Rate limit error: %v", err)
	}

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (b *Bot) sendMarkdownMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	if err := b.middleware.WaitForRateLimit(); err != nil {
		log.Printf("Rate limit error: %v", err)
	}

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func escapeMD(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}
