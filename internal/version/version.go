package version

import (
	"runtime/debug"
)

// Version holds the current application version, set at build time via ldflags.
var Version = "dev"

// Get returns the effective version: either the build-time Version variable,
// or a VCS revision derived from debug.ReadBuildInfo(), or "dev".
func Get() string {
	if Version != "" && Version != "dev" {
		return Version
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		var rev string
		var modified bool
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					modified = true
				}
			}
		}
		if rev != "" {
			if len(rev) > 7 {
				rev = rev[:7]
			}
			if modified {
				return rev + "-dirty"
			}
			return rev
		}
	}

	return Version
}
