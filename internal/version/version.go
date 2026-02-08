package version

import (
	"runtime"
)

// Set via ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func GoVersion() string {
	return runtime.Version()
}
