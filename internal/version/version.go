// Package version exposes build metadata, set via -ldflags at build time.
package version

var (
	Version = "dev"
	Commit  = "none"
)
