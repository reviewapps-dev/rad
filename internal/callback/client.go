package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type StatusPayload struct {
	AppID     string `json:"app_id"`
	Status    string `json:"status"`
	Port      int    `json:"port,omitempty"`
	URL       string `json:"url,omitempty"`
	Error     string `json:"error,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
}

func (c *Client) SendStatus(callbackURL string, payload StatusPayload) error {
	if callbackURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("callback marshal: %w", err)
	}

	resp, err := c.httpClient.Post(callbackURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("callback: POST %s failed: %v", callbackURL, err)
		return nil // Don't fail deploy on callback failure
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("callback: POST %s returned %d", callbackURL, resp.StatusCode)
	}

	return nil
}

type LogPayload struct {
	AppID string   `json:"app_id"`
	Lines []string `json:"lines"`
}

func (c *Client) SendLogs(logsURL string, payload LogPayload) error {
	if logsURL == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	resp, err := c.httpClient.Post(logsURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	return nil
}
