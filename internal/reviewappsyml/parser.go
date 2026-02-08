package reviewappsyml

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Ruby      string            `yaml:"ruby"`
	Node      string            `yaml:"node"`
	Database  string            `yaml:"database"`
	Databases map[string]string `yaml:"databases"`
	AppPath   string            `yaml:"app_path"`
	Build     struct {
		Command string `yaml:"command"`
	} `yaml:"build"`
	Setup struct {
		Command string `yaml:"command"`
	} `yaml:"setup"`
	Seed struct {
		Command string `yaml:"command"`
	} `yaml:"seed"`
	Hooks struct {
		AfterClone     []string `yaml:"after_clone"`
		BeforeBuild    []string `yaml:"before_build"`
		AfterBuild     []string `yaml:"after_build"`
		BeforeMigrate  []string `yaml:"before_migrate"`
		AfterDeploy    []string `yaml:"after_deploy"`
		BeforeTeardown []string `yaml:"before_teardown"`
		OnFailure      []string `yaml:"on_failure"`
	} `yaml:"hooks"`
	Branches struct {
		Only   string `yaml:"only"`
		Ignore string `yaml:"ignore"`
	} `yaml:"branches"`
	Env map[string]string `yaml:"env"`
	HealthCheck struct {
		Path     string `yaml:"path"`
		Timeout  int    `yaml:"timeout"`
		Interval int    `yaml:"interval"`
	} `yaml:"health_check"`
	Processes      map[string]string `yaml:"processes"`
	SystemPackages []string          `yaml:"system_packages"`
}

func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reviewapps.yml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("reviewapps.yml parse: %w", err)
	}

	// Defaults
	if cfg.HealthCheck.Timeout == 0 {
		cfg.HealthCheck.Timeout = 30
	}
	if cfg.HealthCheck.Interval == 0 {
		cfg.HealthCheck.Interval = 2
	}

	return &cfg, nil
}
