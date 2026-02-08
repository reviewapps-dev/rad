package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Auth     AuthConfig     `toml:"auth"`
	API      APIConfig      `toml:"api"`
	Paths    PathsConfig    `toml:"paths"`
	Caddy    CaddyConfig    `toml:"caddy"`
	Defaults DefaultsConfig `toml:"defaults"`

	// Runtime flags (not from TOML)
	Dev bool `toml:"-"`
}

type CaddyConfig struct {
	Enabled   bool   `toml:"enabled"`
	ConfigDir string `toml:"config_dir"`
}

type ServerConfig struct {
	Listen string `toml:"listen"`
	TLS    bool   `toml:"tls"`
}

type AuthConfig struct {
	TokenHash   string `toml:"token_hash"`
	Token       string `toml:"-"` // plain token for dev mode, set via flag
	StreamToken string `toml:"-"` // read-only token for WebSocket log streaming
}

type APIConfig struct {
	Endpoint string `toml:"endpoint"`
	ServerID string `toml:"server_id"`
	APIKey   string `toml:"api_key"`
}

type PathsConfig struct {
	AppsDir string `toml:"apps_dir"`
	LogDir  string `toml:"log_dir"`
}

type DefaultsConfig struct {
	RubyVersion     string `toml:"ruby_version"`
	DatabaseAdapter string `toml:"database_adapter"`
}

func DefaultDev() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Dev: true,
		Server: ServerConfig{
			Listen: "localhost:7890",
			TLS:    false,
		},
		Auth: AuthConfig{},
		API: APIConfig{
			Endpoint: "http://localhost:3000/api/v1",
		},
		Paths: PathsConfig{
			AppsDir: filepath.Join(home, ".reviewapps", "apps"),
			LogDir:  filepath.Join(home, ".reviewapps", "log"),
		},
		Caddy: CaddyConfig{
			Enabled:   false, // No Caddy in dev mode — access apps via localhost:{port}
			ConfigDir: filepath.Join(home, ".reviewapps", "caddy"),
		},
		Defaults: DefaultsConfig{
			RubyVersion:     "3.4.1",
			DatabaseAdapter: "sqlite",
		},
	}
}

func DefaultProd() *Config {
	return &Config{
		Dev: false,
		Server: ServerConfig{
			Listen: "0.0.0.0:7890",
			TLS:    true,
		},
		Auth: AuthConfig{},
		API: APIConfig{
			Endpoint: "https://reviewapps.dev/api/v1",
		},
		Paths: PathsConfig{
			AppsDir: "/opt/reviewapps/apps",
			LogDir:  "/opt/reviewapps/log",
		},
		Caddy: CaddyConfig{
			Enabled:   true,
			ConfigDir: "/opt/reviewapps/etc/caddy/sites",
		},
		Defaults: DefaultsConfig{
			RubyVersion:     "3.4.1",
			DatabaseAdapter: "sqlite",
		},
	}
}

func Load(path string, dev bool) (*Config, error) {
	var cfg *Config
	if dev {
		cfg = DefaultDev()
	} else {
		cfg = DefaultProd()
	}

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, cfg); err != nil {
				return nil, fmt.Errorf("config: parse %s: %w", path, err)
			}
		}
	}

	return cfg, nil
}

func (c *Config) EnsureDirs() error {
	for _, dir := range []string{c.Paths.AppsDir, c.Paths.LogDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("config: create dir %s: %w", dir, err)
		}
	}
	return nil
}
