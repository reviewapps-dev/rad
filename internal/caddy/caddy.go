package caddy

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Manager handles per-app Caddy site configs and reloads.
type Manager struct {
	// ConfigDir is the directory where per-app Caddy configs are written.
	// The main Caddyfile should `import {ConfigDir}/*.caddy` to pick them up.
	ConfigDir string

	// CaddyBin is the path to the caddy binary. Defaults to "caddy".
	CaddyBin string

	// Enabled controls whether Caddy operations are performed.
	// In dev mode this is false — apps are accessed directly via localhost:{port}.
	Enabled bool
}

// NewManager creates a Caddy manager. If configDir is empty or enabled is false,
// all operations become no-ops.
func NewManager(configDir string, enabled bool) *Manager {
	bin := "caddy"
	// Try to find caddy in common locations
	for _, p := range []string{"/opt/homebrew/bin/caddy", "/usr/bin/caddy", "/usr/local/bin/caddy"} {
		if _, err := os.Stat(p); err == nil {
			bin = p
			break
		}
	}

	return &Manager{
		ConfigDir: configDir,
		CaddyBin:  bin,
		Enabled:   enabled,
	}
}

// SiteConfig holds the parameters needed to generate a Caddy site block.
type SiteConfig struct {
	AppID     string
	Subdomain string // e.g. "pr-42-myapp" → pr-42-myapp.srv.reviewapps.dev
	Port      int
	LogDir    string
}

// WriteSiteConfig writes a per-app Caddy config file.
// The file is named {app_id}.caddy and contains a reverse proxy block.
func (m *Manager) WriteSiteConfig(cfg SiteConfig) error {
	if !m.Enabled {
		return nil
	}

	if err := os.MkdirAll(m.ConfigDir, 0755); err != nil {
		return fmt.Errorf("caddy: create config dir: %w", err)
	}

	hostname := cfg.Subdomain + ".srv.reviewapps.dev"

	content := fmt.Sprintf(`%s {
	reverse_proxy localhost:%d {
		header_up X-Forwarded-Proto {scheme}
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Host {host}
	}
	log {
		output file %s
	}
}
`, hostname, cfg.Port, filepath.Join(cfg.LogDir, cfg.AppID+".access.log"))

	path := filepath.Join(m.ConfigDir, cfg.AppID+".caddy")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("caddy: write site config: %w", err)
	}

	log.Printf("caddy: wrote site config %s → %s:%d", path, hostname, cfg.Port)
	return nil
}

// RemoveSiteConfig removes the per-app Caddy config file.
func (m *Manager) RemoveSiteConfig(appID string) error {
	if !m.Enabled {
		return nil
	}

	path := filepath.Join(m.ConfigDir, appID+".caddy")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("caddy: remove site config: %w", err)
	}

	log.Printf("caddy: removed site config %s", path)
	return nil
}

// Reload tells Caddy to reload its configuration.
// Uses `caddy reload` which sends to the admin API internally.
func (m *Manager) Reload() error {
	if !m.Enabled {
		return nil
	}

	// Find the main Caddyfile — look in the parent of ConfigDir
	caddyfile := filepath.Join(filepath.Dir(m.ConfigDir), "Caddyfile")
	if _, err := os.Stat(caddyfile); os.IsNotExist(err) {
		// Try common locations
		for _, p := range []string{"/etc/caddy/Caddyfile", "/opt/reviewapps/etc/caddy/Caddyfile"} {
			if _, err := os.Stat(p); err == nil {
				caddyfile = p
				break
			}
		}
	}

	cmd := exec.Command(m.CaddyBin, "reload", "--config", caddyfile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("caddy reload: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	log.Printf("caddy: reloaded configuration")
	return nil
}

// IsRunning checks if Caddy's admin API is responding.
func (m *Manager) IsRunning() bool {
	if !m.Enabled {
		return false
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:2019/config/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
