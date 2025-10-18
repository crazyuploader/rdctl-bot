package realdebrid

import (
	"encoding/json"
	"fmt"
)

// UnrestrictedLink represents an unrestricted link
type UnrestrictedLink struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
	Link     string `json:"link"`
	Host     string `json:"host"`
	Chunks   int    `json:"chunks"`
	Download string `json:"download"`
}

// Download represents a download entry
type Download struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	Filesize  int64  `json:"filesize"`
	Link      string `json:"link"`
	Host      string `json:"host"`
	Generated string `json:"generated"`
}

// UnrestrictLink unrestricts a hoster link
func (c *Client) UnrestrictLink(link string) (*UnrestrictedLink, error) {
	formData := map[string]string{
		"link": link,
	}

	data, err := c.POSTForm("/unrestrict/link", formData)
	if err != nil {
		return nil, fmt.Errorf("failed to unrestrict link: %w", err)
	}

	var unrestricted UnrestrictedLink
	if err := json.Unmarshal(data, &unrestricted); err != nil {
		return nil, fmt.Errorf("failed to parse unrestricted link: %w", err)
	}

	return &unrestricted, nil
}

// GetDownloads retrieves download history
func (c *Client) GetDownloads(limit, offset int) ([]Download, error) {
	params := make(map[string]string)
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if offset > 0 {
		params["offset"] = fmt.Sprintf("%d", offset)
	}

	data, err := c.GET("/downloads", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get downloads: %w", err)
	}

	var downloads []Download
	if err := json.Unmarshal(data, &downloads); err != nil {
		return nil, fmt.Errorf("failed to parse downloads: %w", err)
	}

	return downloads, nil
}

// DeleteDownload removes a download from history
func (c *Client) DeleteDownload(downloadID string) error {
	_, err := c.DELETE("/downloads/delete/" + downloadID)
	if err != nil {
		return fmt.Errorf("failed to delete download: %w", err)
	}

	return nil
}
