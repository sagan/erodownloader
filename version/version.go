package version

import (
	"strings"
)

var (
	Version = ""
	Commit  = "unknown"
	Date    = "unknown"
)

func init() {
	if !strings.HasPrefix(Version, "v") {
		Version = "v" + Version
	}
}
