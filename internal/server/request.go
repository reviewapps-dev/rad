package server

type DeployRequest struct {
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
	Hooks           *DeployHooks      `json:"hooks,omitempty"`
}

type DeployHooks struct {
	AfterClone     []string `json:"after_clone,omitempty"`
	BeforeBuild    []string `json:"before_build,omitempty"`
	AfterBuild     []string `json:"after_build,omitempty"`
	BeforeMigrate  []string `json:"before_migrate,omitempty"`
	AfterDeploy    []string `json:"after_deploy,omitempty"`
	BeforeTeardown []string `json:"before_teardown,omitempty"`
	OnFailure      []string `json:"on_failure,omitempty"`
}

type ExecRequest struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds, default 30
}

type UpdateRequest struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum,omitempty"` // SHA-256 hex
}
