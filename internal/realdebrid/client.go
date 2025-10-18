package realdebrid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client represents a Real-Debrid API client
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// APIError represents an error from the Real-Debrid API
type APIError struct {
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error"`
	Message      string `json:"message"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("RD API error %d: %s - %s", e.ErrorCode, e.ErrorMessage, e.Message)
	}
	return fmt.Sprintf("RD API error %d: %s", e.ErrorCode, e.ErrorMessage)
}

// NewClient creates a new Real-Debrid API client
func NewClient(baseURL, apiToken string, timeout time.Duration) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// doRequest performs an HTTP request with proper authentication
func (c *Client) doRequest(method, endpoint string, body interface{}, queryParams map[string]string) ([]byte, error) {
	fullURL := c.baseURL + endpoint

	// Add query parameters
	if len(queryParams) > 0 {
		params := url.Values{}
		for k, v := range queryParams {
			params.Add(k, v)
		}
		fullURL += "?" + params.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return nil, &apiErr
	}

	return respBody, nil
}

// GET performs a GET request
func (c *Client) GET(endpoint string, queryParams map[string]string) ([]byte, error) {
	return c.doRequest(http.MethodGet, endpoint, nil, queryParams)
}

// POST performs a POST request
func (c *Client) POST(endpoint string, body interface{}) ([]byte, error) {
	return c.doRequest(http.MethodPost, endpoint, body, nil)
}

// DELETE performs a DELETE request
func (c *Client) DELETE(endpoint string) ([]byte, error) {
	return c.doRequest(http.MethodDelete, endpoint, nil, nil)
}

// POSTForm performs a POST request with form data
func (c *Client) POSTForm(endpoint string, formData map[string]string) ([]byte, error) {
	fullURL := c.baseURL + endpoint

	data := url.Values{}
	for k, v := range formData {
		data.Set(k, v)
	}

	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return nil, &apiErr
	}

	return respBody, nil
}
