package version

import "fmt"

var (
	Version   = "0.0.0-dev"
	Commit    = "none"
	BuildTime = "unknown"
	GoVersion = "unknown"
	Platform  = "unknown"
)

func Get() string {
	return Version
}

func FullString() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildTime)
}

func UserAgent() string {
	return "cs-cloud/" + Version
}
