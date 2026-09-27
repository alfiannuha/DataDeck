// Package version exposes build metadata for the DataDeck executable.
//
// Values are overridden at build time with -ldflags -X (see
// scripts/build-release.sh). The defaults are safe for development builds and
// deliberately look non-production.
package version

import "fmt"

var (
	// Version is the release version, sourced from the repository VERSION file.
	Version = "0.1.0-dev"
	// Commit is the source revision the binary was built from.
	Commit = "unknown"
	// BuildDate is the UTC build timestamp (ISO-8601) or "unknown".
	BuildDate = "unknown"
)

// String renders the human-readable build metadata line.
func String() string {
	return fmt.Sprintf("datadeck %s (commit %s, built %s)", Version, Commit, BuildDate)
}
