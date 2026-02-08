package version

import (
	"runtime"
)

// Set via ldflags at build time.
var (
	Version   = "0.0.1"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func GoVersion() string {
	return runtime.Version()
}
