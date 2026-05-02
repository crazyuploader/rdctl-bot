package web

import (
	"log"
	"strconv"
	"strings"

	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/gofiber/fiber/v3"
)

// GetAuthInfo returns information about the current authenticated user
func (d *Dependencies) GetAuthInfo(c fiber.Ctx) error {
	authType, _ := c.Locals(ContextKeyAuthType).(string)
	role := GetRole(c)
	token := GetToken(c)

	response := fiber.Map{
		"success":   true,
		"auth_type": authType,
		"role":      role,
		"is_admin":  role == RoleAdmin,
	}

	if token != nil {
		response["user_id"] = token.UserID
		response["username"] = token.Username
		response["first_name"] = token.FirstName
		response["expires_at"] = token.ExpiresAt
	}

	return c.JSON(response)
}

// GetStatus retrieves the Real-Debrid account status
func (d *Dependencies) GetStatus(c fiber.Ctx) error {
	user, err := d.RDClient.GetUser()
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "data": user})
}

// GetTorrents retrieves the list of active torrents
func (d *Dependencies) GetTorrents(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	result, err := d.RDClient.GetTorrentsWithCount(limit, offset)
	if err != nil {
		return err
	}

	// Format status and size for frontend convenience
	for i := range result.Torrents {
		result.Torrents[i].Status = realdebrid.FormatStatus(result.Torrents[i].Status)
	}

	return c.JSON(fiber.Map{
		"success":     true,
		"data":        result.Torrents,
		"total_count": result.TotalCount,
	})
}

// GetTorrentInfo retrieves detailed information about a single torrent
func (d *Dependencies) GetTorrentInfo(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Torrent ID is required")
	}
	torrent, err := d.RDClient.GetTorrentInfo(id)
	if err != nil {
		return err
	}
	torrent.Status = realdebrid.FormatStatus(torrent.Status)
	return c.JSON(fiber.Map{"success": true, "data": torrent})
}

// AddTorrent adds a new torrent from a magnet link
func (d *Dependencies) AddTorrent(c fiber.Ctx) error {
	var body struct {
		Magnet string `json:"magnet"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Magnet == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Magnet link is required")
	}

	resp, err := d.RDClient.AddMagnet(body.Magnet)
	if err != nil {
		return err
	}

	// Automatically select all files
	if err := d.RDClient.SelectAllFiles(resp.ID); err != nil {
		log.Printf("Failed to select files for torrent %s: %v", resp.ID, err)
		// Non-fatal, just log it
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": resp})
}

// DeleteTorrent deletes a torrent
func (d *Dependencies) DeleteTorrent(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "id parameter is required")
	}

	if err := d.RDClient.DeleteTorrent(id); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Torrent deleted successfully"})
}

// GetDownloads retrieves the download history
func (d *Dependencies) GetDownloads(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	result, err := d.RDClient.GetDownloadsWithCount(limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success":     true,
		"data":        result.Downloads,
		"total_count": result.TotalCount,
	})
}

// CheckDomain checks if a domain is supported
func (d *Dependencies) CheckDomain(c fiber.Ctx) error {
	domain := strings.ToLower(strings.TrimSpace(c.Query("domain")))
	if domain == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Domain is required")
	}

	supported, checkedDomain, err := d.RDClient.IsDomainSupported(domain)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"success": true, "supported": supported, "domain": domain, "checked_domain": checkedDomain})
}

// UnrestrictLink unrestricts a hoster link
func (d *Dependencies) UnrestrictLink(c fiber.Ctx) error {
	var body struct {
		Link string `json:"link"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Link == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Link is required")
	}

	unrestricted, err := d.RDClient.UnrestrictLink(body.Link)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"success": true, "data": unrestricted})
}

// DeleteDownload deletes a download from history
func (d *Dependencies) DeleteDownload(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "id is required")
	}
	if err := d.RDClient.DeleteDownload(id); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Download link removed successfully"})
}

// GetStats returns aggregate torrent and download statistics
func (d *Dependencies) GetStats(c fiber.Ctx) error {
	ctx := c.Context()

	// Total torrent count + paginate ALL torrents for accurate size/status
	torrentsResult, err := d.RDClient.GetTorrentsWithCount(1, 0)
	if err != nil {
		return err
	}
	totalCount := torrentsResult.TotalCount

	// Active torrent count
	activeCount, _ := d.RDClient.GetActiveCount()

	// Paginate all torrents to get accurate size + status breakdown
	var totalBytes int64
	downloadingCount := 0
	downloadedCount := 0
	const pageSize = 2500
	for offset := 0; ; offset += pageSize {
		page, err := d.RDClient.GetTorrents(pageSize, offset)
		if err != nil {
			break
		}
		for _, t := range page {
			totalBytes += t.Bytes
			switch realdebrid.FormatStatus(t.Status) {
			case "Downloading":
				downloadingCount++
			case "Downloaded":
				downloadedCount++
			}
		}
		if len(page) < pageSize {
			break
		}
	}

	// Total downloads count
	downloadsResult, _ := d.RDClient.GetDownloadsWithCount(1, 0)

	// Kept torrents count
	keptTorrents, _ := d.KeptRepo.ListKeptTorrents(ctx)

	totalDownloads := 0
	if downloadsResult != nil {
		totalDownloads = downloadsResult.TotalCount
	}

	activeNb := 0
	activeLimit := 0
	if activeCount != nil {
		activeNb = activeCount.Nb
		activeLimit = activeCount.Limit
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"torrents_total":       totalCount,
			"torrents_active":      activeNb,
			"torrents_limit":       activeLimit,
			"torrents_downloading": downloadingCount,
			"torrents_downloaded":  downloadedCount,
			"torrents_kept":        len(keptTorrents),
			"torrents_bytes":       totalBytes,
			"torrents_sample":      totalCount, // now a full count, no estimate
			"downloads_total":      totalDownloads,
		},
	})
}

// GetUserStats retrieves statistics for a user
func (d *Dependencies) GetUserStats(c fiber.Ctx) error {
	userID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
	}

	stats, err := d.CommandRepo.GetUserStats(c.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fiber.NewError(fiber.StatusNotFound, "User stats not found")
		}
		return err
	}
	return c.JSON(fiber.Map{"success": true, "data": stats})
}

// ExchangeToken exchanges a short-lived code for a real token
func (d *Dependencies) ExchangeToken(c fiber.Ctx) error {
	var body struct {
		Code string `json:"code"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Code == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Exchange code is required")
	}

	tokenID, err := d.TokenStore.ExchangeToken(body.Code)
	if err != nil {
		return err
	}

	if tokenID == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid or expired exchange code")
	}

	return c.JSON(fiber.Map{
		"success": true,
		"token":   tokenID,
	})
}

// GetKeptTorrents returns all kept torrents
func (d *Dependencies) GetKeptTorrents(c fiber.Ctx) error {
	keptTorrents, err := d.KeptRepo.ListKeptTorrents(c.Context())
	if err != nil {
		return err
	}

	type keptTorrentResponse struct {
		db.KeptTorrent
		CurrentTorrent *realdebrid.Torrent `json:"CurrentTorrent,omitempty"`
	}

	response := make([]keptTorrentResponse, 0, len(keptTorrents))
	for _, keptTorrent := range keptTorrents {
		item := keptTorrentResponse{
			KeptTorrent: keptTorrent,
		}

		// Best-effort enrichment so the kept tab can show the same live
		// metadata as the main torrent list when the torrent still exists upstream.
		torrent, err := d.RDClient.GetTorrentInfo(keptTorrent.TorrentID)
		if err == nil {
			torrent.Status = realdebrid.FormatStatus(torrent.Status)
			if torrent.Filename == "" {
				torrent.Filename = keptTorrent.Filename
			}
			item.CurrentTorrent = torrent
		}

		response = append(response, item)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// KeepTorrent marks a torrent as kept (excluded from auto-delete)
func (d *Dependencies) KeepTorrent(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Torrent ID is required")
	}

	// Get torrent info for filename
	torrent, err := d.RDClient.GetTorrentInfo(id)
	if err != nil {
		return err
	}

	// Get user ID from token or context
	userID := int64(0)
	if token := GetToken(c); token != nil {
		userID = token.UserID
	}

	// Determine the keep limit (0 = unlimited for admins)
	role := GetRole(c)
	maxKept := 0
	if role != RoleAdmin {
		maxKept = d.Config.App.MaxKeptTorrents
	}

	// Keep torrent (limit is enforced atomically inside the transaction)
	if err := d.KeptRepo.KeepTorrent(c.Context(), id, torrent.Filename, userID, maxKept); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Torrent marked as kept",
	})
}

// UnkeepTorrent removes the keep mark from a torrent
func (d *Dependencies) UnkeepTorrent(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Torrent ID is required")
	}

	// Get user ID from token or context
	userID := int64(0)
	if token := GetToken(c); token != nil {
		userID = token.UserID
	}

	role := GetRole(c)
	isAdmin := role == RoleAdmin

	if err := d.KeptRepo.UnkeepTorrent(c.Context(), id, userID, isAdmin); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Torrent unmarked as kept",
	})
}

// GetAutoDeleteSetting returns the current auto-delete setting
func (d *Dependencies) GetAutoDeleteSetting(c fiber.Ctx) error {
	value, err := d.SettingRepo.GetSetting(c.Context(), "auto_delete_days")
	if err != nil {
		return err
	}

	if value == "" {
		if d.Config.App.AutoDeleteDays > 0 {
			value = strconv.Itoa(d.Config.App.AutoDeleteDays)
		} else {
			value = "0"
		}
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    value,
	})
}

// SetAutoDeleteSetting updates the auto-delete setting
func (d *Dependencies) SetAutoDeleteSetting(c fiber.Ctx) error {
	var body struct {
		Value string `json:"value"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Validate the value is a valid integer
	days, err := strconv.Atoi(body.Value)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Value must be a valid integer")
	}

	// Validate range: must be non-negative and within reasonable upper bound
	if days < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "Value must be non-negative (>= 0)")
	}

	if days > 3650 {
		return fiber.NewError(fiber.StatusBadRequest, "Value must not exceed 3650 days")
	}

	// Store the validated value (as string to match existing interface)
	userID := int64(0)
	if token := GetToken(c); token != nil {
		userID = token.UserID
	}
	if err := d.SettingRepo.SetSettingWithAudit(c.Context(), "auto_delete_days", body.Value, userID, 0); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Auto-delete setting updated",
	})
}
