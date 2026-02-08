package deploy

import (
	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/caddy"
	"github.com/reviewapps-dev/rad/internal/config"
	"github.com/reviewapps-dev/rad/internal/logging"
	"github.com/reviewapps-dev/rad/internal/port"
	"github.com/reviewapps-dev/rad/internal/reviewappsyml"
)

type Step interface {
	Name() string
	Run(ctx *StepContext) error
}

type StepContext struct {
	AppState  *app.AppState
	Config    *config.Config
	Logger    *logging.DeployLogger
	Ports     *port.Allocator
	Store     *app.Store
	Caddy     *caddy.Manager

	// Enriched during pipeline
	AppDir       string
	RepoDir      string
	RubyPath     string
	NodePath     string
	ReviewConfig *reviewappsyml.Config
	EnvMap       map[string]string
	Port         int
	PID          int

	// Process declarations: name → command. "web" is the primary process.
	// Populated from reviewapps.yml or defaulted to rails server.
	Processes map[string]string

	// For JS package manager detection
	JSPackageManager string // "npm", "yarn", "pnpm", "bun", or ""

	// Redeploy mode — update in place instead of fresh deploy
	Redeploy bool
}
