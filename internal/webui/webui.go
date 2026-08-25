// Package webui embeds the production frontend build and exposes it as an fs.FS.
package webui

import (
	"embed"
	"io/fs"
)

// dist holds the Vite production build. The Docker image build copies web/dist
// here before compiling; the committed placeholder keeps local `go test` /
// `go build` working without a frontend build.
//
//go:embed all:dist
var dist embed.FS

// FS returns the embedded frontend assets rooted at the build output directory.
func FS() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
