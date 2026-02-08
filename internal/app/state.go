package app

import (
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusCloning   Status = "cloning"
	StatusBuilding  Status = "building"
	StatusStarting  Status = "starting"
	StatusRunning   Status = "running"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
	StatusTeardown  Status = "teardown"
)

type Hooks struct {
	AfterClone     []string `json:"after_clone,omitempty"`
	BeforeBuild    []string `json:"before_build,omitempty"`
	AfterBuild     []string `json:"after_build,omitempty"`
	BeforeMigrate  []string `json:"before_migrate,omitempty"`
	AfterDeploy    []string `json:"after_deploy,omitempty"`
	BeforeTeardown []string `json:"before_teardown,omitempty"`
	OnFailure      []string `json:"on_failure,omitempty"`
}

type ProcessInfo struct {
	Name string `json:"name"`
	PID  int    `json:"pid"`
	Port int    `json:"port,omitempty"` // Only the web process gets a port
}

type AppState struct {
	AppID           string            `json:"app_id"`
	RepoURL         string            `json:"repo_url"`
	Branch          string            `json:"branch"`
	CommitSHA       string            `json:"commit_sha"`
	RubyVersion     string            `json:"ruby_version"`
	NodeVersion     string            `json:"node_version"`
	DatabaseAdapter string            `json:"database_adapter"`
	Databases       map[string]string `json:"databases,omitempty"`
	EnvVars         map[string]string `json:"env_vars,omitempty"`
	BuildCommand    string            `json:"build_command,omitempty"`
	SeedCommand     string            `json:"seed_command,omitempty"`
	Subdomain       string            `json:"subdomain"`
	CallbackURL     string            `json:"callback_url"`
	Hooks           *Hooks            `json:"hooks,omitempty"` // From deploy request


	// Runtime state
	Status         Status                 `json:"status"`
	Port           int                    `json:"port,omitempty"`
	PID            int                    `json:"pid,omitempty"`            // Primary (web) process PID for backward compat
	Processes      map[string]ProcessInfo `json:"processes,omitempty"`      // All managed processes
	ProcessCommands map[string]string     `json:"process_commands,omitempty"` // Process name → command for restart
	AppDir         string                 `json:"app_dir,omitempty"`
	Error          string                 `json:"error,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	BuildLog       []string               `json:"build_log,omitempty"`
}
