package deploy

import (
	"github.com/reviewapps-dev/rad/internal/database"
)

type SetupDatabaseStep struct{}

func (s *SetupDatabaseStep) Name() string { return "setup-database" }

func (s *SetupDatabaseStep) Run(ctx *StepContext) error {
	databases := ctx.AppState.Databases
	if len(databases) == 0 {
		// Single database
		databases = map[string]string{
			"primary": ctx.AppState.DatabaseAdapter,
		}
	}

	for name, adapter := range databases {
		dbCfg := &database.DBConfig{
			AppID:   ctx.AppState.AppID,
			Name:    name,
			Adapter: adapter,
		}

		if ctx.Redeploy {
			ctx.Logger.Log("redeploy: reusing %s database (%s): %s", name, adapter, dbCfg.DBName())
		} else {
			ctx.Logger.Log("setting up %s database (%s): %s", name, adapter, dbCfg.DBName())
			if adapter == "postgresql" || adapter == "postgres" {
				if err := database.CreatePostgresDB(dbCfg.DBName()); err != nil {
					return err
				}
			}
		}

		// Set env var for this database
		ctx.EnvMap[dbCfg.EnvKey()] = dbCfg.URL(ctx.Config.Paths.AppsDir)
		ctx.Logger.Log("  %s=%s", dbCfg.EnvKey(), dbCfg.URL(ctx.Config.Paths.AppsDir))
	}

	return nil
}
