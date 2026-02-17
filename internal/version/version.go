package version

import "fmt"

// Set via -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func String() string {
	return fmt.Sprintf("devbot version %s\n  Commit: %s\n  Built:  %s", Version, Commit, BuildTime)
}
