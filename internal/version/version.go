package version

import (
	"fmt"
	"runtime"
)

// Set via -ldflags at build time. Defaults are for `go install` / dev builds.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("version: %s\ncommit: %s\nbuilt: %s\ngo: %s\nplatform: %s/%s",
		Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
