package database

import (
	"fmt"
	"os/exec"
	"strings"
)

type DBConfig struct {
	AppID   string
	Name    string // e.g. "primary", "queue", "cache"
	Adapter string // "sqlite" or "postgresql"
}

func (d *DBConfig) DBName() string {
	return fmt.Sprintf("ra_%s_%s", sanitize(d.AppID), sanitize(d.Name))
}

func (d *DBConfig) URL(appsDir string) string {
	switch d.Adapter {
	case "postgresql", "postgres":
		return fmt.Sprintf("postgres://localhost/%s", d.DBName())
	default: // sqlite
		return fmt.Sprintf("sqlite3:%s/%s/%s.sqlite3", appsDir, d.AppID, d.Name)
	}
}

func (d *DBConfig) EnvKey() string {
	if d.Name == "primary" {
		return "DATABASE_URL"
	}
	return strings.ToUpper(d.Name) + "_DATABASE_URL"
}

func CreatePostgresDB(dbName string) error {
	cmd := exec.Command("createdb", dbName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore if already exists
		if strings.Contains(string(out), "already exists") {
			return nil
		}
		return fmt.Errorf("createdb %s: %w\n%s", dbName, err, string(out))
	}
	return nil
}

func DropPostgresDB(dbName string) error {
	cmd := exec.Command("dropdb", "--if-exists", dbName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dropdb %s: %w\n%s", dbName, err, string(out))
	}
	return nil
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, s)
}
