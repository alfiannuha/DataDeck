// Package version exposes build metadata for the DataDeck executable.
//
// Values are overridden at build time with -ldflags -X (see
// scripts/build-release.sh and docker/Dockerfile). The defaults are safe for
// development builds: the application version is the canonical release version
// (kept identical across channels), and the channel defaults to "development"
// so an uninjected build is never mistaken for an official release.
package version

import "fmt"

// Channels distinguish an uninjected development build from an official release.
const (
	ChannelDevelopment = "development"
	ChannelRelease     = "release"
)

var (
	// Version is the canonical application version. The repository VERSION file
	// is the source of truth; every component must report the same value.
	Version = "0.1.0"
	// Commit is the source revision the binary was built from.
	Commit = "unknown"
	// BuildDate is the UTC build timestamp (RFC3339) or "unknown".
	BuildDate = "unknown"
	// Channel is "development" (default) or "release" when set at build time.
	Channel = ChannelDevelopment
)

// String renders the human-readable build metadata line.
func String() string {
	return fmt.Sprintf("datadeck %s (commit %s, built %s, channel %s)", Version, Commit, BuildDate, Channel)
}
