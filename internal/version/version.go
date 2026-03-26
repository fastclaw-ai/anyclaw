package version

import (
	"fmt"
	"runtime"
)

// Build-time variables, set via -ldflags.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info returns a formatted version string.
func Info() string {
	return fmt.Sprintf("anyclaw %s (%s) built %s %s/%s", Version, Commit, Date, runtime.GOOS, runtime.GOARCH)
}
