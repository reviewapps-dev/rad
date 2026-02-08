package heartbeat

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/config"
	"github.com/reviewapps-dev/rad/internal/version"
)

type Heartbeat struct {
	cfg    *config.Config
	store  *app.Store
	client *http.Client
	start  time.Time
	stop   chan struct{}
}

func New(cfg *config.Config, store *app.Store) *Heartbeat {
	return &Heartbeat{
		cfg:    cfg,
		store:  store,
		client: &http.Client{Timeout: 10 * time.Second},
		start:  time.Now(),
		stop:   make(chan struct{}),
	}
}

type Payload struct {
	Version      string `json:"version"`
	Uptime       int64  `json:"uptime"`
	AppCount     int    `json:"app_count"`
	MemoryMB     int    `json:"memory_mb"`
	RvVersion    string `json:"rv_version"`
	FnmVersion   string `json:"fnm_version"`
	CaddyVersion string `json:"caddy_version"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
}

func (h *Heartbeat) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.send()
			case <-h.stop:
				return
			}
		}
	}()
}

func (h *Heartbeat) Stop() {
	close(h.stop)
}

func (h *Heartbeat) send() {
	rvVer := "unknown"
	if out, err := exec.Command("/opt/homebrew/bin/rv", "--version").Output(); err == nil {
		rvVer = strings.TrimSpace(string(out))
	}
	fnmVer := "unknown"
	if out, err := exec.Command("fnm", "--version").Output(); err == nil {
		fnmVer = strings.TrimSpace(string(out))
	}
	caddyVer := "not installed"
	if out, err := exec.Command("caddy", "version").Output(); err == nil {
		caddyVer = strings.TrimSpace(strings.Fields(string(out))[0])
	}

	// Get current process memory usage
	var memMB int
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memMB = int(m.Sys / 1024 / 1024)

	payload := Payload{
		Version:      version.Version,
		Uptime:       int64(time.Since(h.start).Seconds()),
		AppCount:     h.store.Count(),
		MemoryMB:     memMB,
		RvVersion:    rvVer,
		FnmVersion:   fnmVer,
		CaddyVersion: caddyVer,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("heartbeat: marshal: %v", err)
		return
	}

	url := h.cfg.API.Endpoint + "/heartbeats"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if h.cfg.API.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.cfg.API.APIKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("heartbeat: %v", err)
		return
	}
	resp.Body.Close()
}
