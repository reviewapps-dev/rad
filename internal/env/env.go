package env

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
)

type buildOptions struct {
	devMode bool
}

type BuildOption func(*buildOptions)

func WithDevMode(dev bool) BuildOption {
	return func(o *buildOptions) { o.devMode = dev }
}

func Build(appID, subdomain string, userEnv map[string]string, dbEnv map[string]string, opts ...BuildOption) map[string]string {
	o := buildOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	host := subdomain + ".srv.reviewapps.dev"
	ssl := "true"
	if o.devMode {
		host = "localhost"
		ssl = "false"
	}

	env := map[string]string{
		"REVIEWAPPS":                "true",
		"REVIEWAPPS_HOST":           host,
		"REVIEWAPPS_SSL":            ssl,
		"RAILS_ENV":                 "production",
		"NODE_ENV":                  "production",
		"RAILS_SERVE_STATIC_FILES":  "true",
		"RAILS_LOG_TO_STDOUT":       "true",
		"DISABLE_SPRING":            "1",
		"WEB_CONCURRENCY":           "0",
		"RAILS_MAX_THREADS":         "3",
		"SECRET_KEY_BASE":           generateSecret(),
		"ACTION_CABLE_ADAPTER":      "async",
	}

	// Database URLs (lower priority)
	for k, v := range dbEnv {
		env[k] = v
	}

	// User env vars (highest priority)
	for k, v := range userEnv {
		env[k] = v
	}

	return env
}

func WriteFile(path string, envMap map[string]string) error {
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, envMap[k]))
	}

	return os.WriteFile(path, []byte(sb.String()), 0600)
}

func generateSecret() string {
	b := make([]byte, 64)
	rand.Read(b)
	return hex.EncodeToString(b)
}
