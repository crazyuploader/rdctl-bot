package realdebrid

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Torrent represents a Real-Debrid torrent
type Torrent struct {
	ID       string    `json:"id"`
	Filename string    `json:"filename"`
	Hash     string    `json:"hash"`
	Bytes    int64     `json:"bytes"`
	Progress float64   `json:"progress"`
	Status   string    `json:"status"`
	Added    time.Time `json:"added"`
	Files    []File    `json:"files,omitempty"`
	Links    []string  `json:"links,omitempty"`
	Speed    int64     `json:"speed,omitempty"`
	Seeders  int       `json:"seeders,omitempty"`
}

// File represents a file in a torrent
type File struct {
	ID       int    `json:"id"`
	Path     string `json:"path"`
	Bytes    int64  `json:"bytes"`
	Selected int    `json:"selected"`
}

// AddMagnetResponse represents the response from adding a magnet
type AddMagnetResponse struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

// InstantAvailability represents instant availability check response
type InstantAvailability map[string]interface{}

// GetTorrents retrieves all torrents
func (c *Client) GetTorrents(limit, offset int) ([]Torrent, error) {
	params := make(map[string]string)
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if offset > 0 {
		params["offset"] = fmt.Sprintf("%d", offset)
	}

	data, err := c.GET("/torrents", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}

	var torrents []Torrent
	if err := json.Unmarshal(data, &torrents); err != nil {
		return nil, fmt.Errorf("failed to parse torrents: %w", err)
	}

	return torrents, nil
}

// GetTorrentInfo retrieves detailed information about a torrent
func (c *Client) GetTorrentInfo(torrentID string) (*Torrent, error) {
	data, err := c.GET("/torrents/info/"+torrentID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent info: %w", err)
	}

	var torrent Torrent
	if err := json.Unmarshal(data, &torrent); err != nil {
		return nil, fmt.Errorf("failed to parse torrent info: %w", err)
	}

	return &torrent, nil
}

// AddMagnet adds a magnet link
func (c *Client) AddMagnet(magnetURL string) (*AddMagnetResponse, error) {
	formData := map[string]string{
		"magnet": magnetURL,
	}

	data, err := c.POSTForm("/torrents/addMagnet", formData)
	if err != nil {
		return nil, fmt.Errorf("failed to add magnet: %w", err)
	}

	var response AddMagnetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse add magnet response: %w", err)
	}

	return &response, nil
}

// SelectFiles selects which files to download from a torrent
func (c *Client) SelectFiles(torrentID string, fileIDs []int) error {
	fileIDsStr := make([]string, len(fileIDs))
	for i, id := range fileIDs {
		fileIDsStr[i] = fmt.Sprintf("%d", id)
	}

	formData := map[string]string{
		"files": strings.Join(fileIDsStr, ","),
	}

	_, err := c.POSTForm("/torrents/selectFiles/"+torrentID, formData)
	if err != nil {
		return fmt.Errorf("failed to select files: %w", err)
	}

	return nil
}

// SelectAllFiles selects all files in a torrent
func (c *Client) SelectAllFiles(torrentID string) error {
	formData := map[string]string{
		"files": "all",
	}

	_, err := c.POSTForm("/torrents/selectFiles/"+torrentID, formData)
	if err != nil {
		return fmt.Errorf("failed to select all files: %w", err)
	}

	return nil
}

// DeleteTorrent deletes a torrent
func (c *Client) DeleteTorrent(torrentID string) error {
	_, err := c.DELETE("/torrents/delete/" + torrentID)
	if err != nil {
		return fmt.Errorf("failed to delete torrent: %w", err)
	}

	return nil
}

// CheckInstantAvailability checks if torrents are instantly available (cached)
func (c *Client) CheckInstantAvailability(hashes []string) (InstantAvailability, error) {
	hashList := strings.Join(hashes, "/")
	data, err := c.GET("/torrents/instantAvailability/"+hashList, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check instant availability: %w", err)
	}

	var availability InstantAvailability
	if err := json.Unmarshal(data, &availability); err != nil {
		return nil, fmt.Errorf("failed to parse availability: %w", err)
	}

	return availability, nil
}

// FormatSize formats bytes to human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatStatus formats torrent status to user-friendly text
func FormatStatus(status string) string {
	switch status {
	case "magnet_error":
		return "❌ Magnet Error"
	case "magnet_conversion":
		return "🔄 Converting Magnet"
	case "waiting_files_selection":
		return "⏳ Waiting for File Selection"
	case "queued":
		return "⏸️ Queued"
	case "downloading":
		return "⬇️ Downloading"
	case "downloaded":
		return "✅ Downloaded"
	case "error":
		return "❌ Error"
	case "virus":
		return "🦠 Virus Detected"
	case "dead":
		return "💀 Dead"
	default:
		return status
	}
}
