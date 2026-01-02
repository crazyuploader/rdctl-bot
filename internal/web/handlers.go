package web

import (
	"log"
	"strconv"

	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/gofiber/fiber/v2"
)

// GetStatus retrieves the Real-Debrid account status
func (d *Dependencies) GetStatus(c *fiber.Ctx) error {
	user, err := d.RDClient.GetUser()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "data": user})
}

// GetTorrents retrieves the list of active torrents
func (d *Dependencies) GetTorrents(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	result, err := d.RDClient.GetTorrentsWithCount(limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
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
func (d *Dependencies) GetTorrentInfo(c *fiber.Ctx) error {
	id := c.Params("id")
	torrent, err := d.RDClient.GetTorrentInfo(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	torrent.Status = realdebrid.FormatStatus(torrent.Status)
	return c.JSON(fiber.Map{"success": true, "data": torrent})
}

// AddTorrent adds a new torrent from a magnet link
func (d *Dependencies) AddTorrent(c *fiber.Ctx) error {
	var body struct {
		Magnet string `json:"magnet"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid request body"})
	}

	if body.Magnet == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Magnet link is required"})
	}

	resp, err := d.RDClient.AddMagnet(body.Magnet)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	// Automatically select all files
	if err := d.RDClient.SelectAllFiles(resp.ID); err != nil {
		log.Printf("Failed to select files for torrent %s: %v", resp.ID, err)
		// Non-fatal, just log it
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": resp})
}

// DeleteTorrent deletes a torrent
func (d *Dependencies) DeleteTorrent(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := d.RDClient.DeleteTorrent(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Torrent deleted successfully"})
}

// GetDownloads retrieves the download history
func (d *Dependencies) GetDownloads(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	result, err := d.RDClient.GetDownloadsWithCount(limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"success":     true,
		"data":        result.Downloads,
		"total_count": result.TotalCount,
	})
}

// UnrestrictLink unrestricts a hoster link
func (d *Dependencies) UnrestrictLink(c *fiber.Ctx) error {
	var body struct {
		Link string `json:"link"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid request body"})
	}

	if body.Link == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Link is required"})
	}

	unrestricted, err := d.RDClient.UnrestrictLink(body.Link)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "data": unrestricted})
}

// DeleteDownload deletes a download from history
func (d *Dependencies) DeleteDownload(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := d.RDClient.DeleteDownload(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Download link removed successfully"})
}

// GetUserStats retrieves statistics for a user
func (d *Dependencies) GetUserStats(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid user ID"})
	}

	stats, err := d.CommandRepo.GetUserStats(c.Context(), uint(userID))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "data": stats})
}
