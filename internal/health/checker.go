package health

import (
	"fmt"
	"net/http"
	"time"
)

var DefaultPaths = []string{"/up", "/health", "/healthz", "/"}

// Check polls the app for a healthy response. If customPath is non-empty,
// it's tried first before falling back to the default cascade.
func Check(port int, host string, timeout time.Duration, interval time.Duration, customPath string) error {
	paths := DefaultPaths
	if customPath != "" {
		// Custom path goes first, then fall back to defaults
		paths = append([]string{customPath}, DefaultPaths...)
	}

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 5 * time.Second}

	for time.Now().Before(deadline) {
		for _, path := range paths {
			url := fmt.Sprintf("http://localhost:%d%s", port, path)
			req, _ := http.NewRequest("GET", url, nil)
			if host != "" {
				req.Host = host
			}

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
		}
		time.Sleep(interval)
	}

	return fmt.Errorf("health check failed after %s", timeout)
}
