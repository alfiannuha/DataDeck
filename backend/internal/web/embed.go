// Package web serves the production frontend from an embedded filesystem.
//
// The release build (scripts/build-embedded.sh) copies `frontend/out` into
// `internal/web/dist`; `//go:embed all:dist` then bakes it into the Go binary.
// Development builds keep a placeholder dist so this package always compiles;
// when no real index.html is embedded the handler reports an explicit 404
// instead of silently serving stale or partial output.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Frontend returns the embedded frontend filesystem root and whether a built
// `index.html` is present. When false, handlers must not serve static content.
func Frontend() (fs.FS, bool) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return sub, false
	}
	return sub, true
}
