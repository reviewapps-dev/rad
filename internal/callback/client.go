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

	c.postWithRetry(callbackURL, body)
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

	// Logs are best-effort, single attempt (they batch every 5s so missing one is fine)
	resp, err := c.httpClient.Post(logsURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	return nil
}

// postWithRetry attempts a POST up to 3 times with exponential backoff.
// Never returns an error — callbacks are best-effort and must not fail deploys.
func (c *Client) postWithRetry(url string, body []byte) {
	delays := []time.Duration{0, 2 * time.Second, 5 * time.Second}

	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}

		resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("callback: POST %s attempt %d failed: %v", url, attempt+1, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 500 {
			if resp.StatusCode >= 400 {
				log.Printf("callback: POST %s returned %d", url, resp.StatusCode)
			}
			return
		}

		log.Printf("callback: POST %s attempt %d returned %d", url, attempt+1, resp.StatusCode)
	}

	log.Printf("callback: POST %s failed after 3 attempts, giving up", url)
}
