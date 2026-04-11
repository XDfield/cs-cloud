package cli

import (
	"fmt"

	"cs-cloud/internal/version"
)

func printVersion() {
	printTitle("cs-cloud")
	fmt.Print(renderKV([][2]string{
		{"version", version.Get()},
		{"commit", version.Commit},
		{"built", version.BuildTime},
		{"go", version.GoVersion},
		{"platform", version.Platform},
	}))
}
