// Package buildinfo carries release metadata injected by GoReleaser.
package buildinfo

// Values are set with -ldflags at release time.
var (
	Version = "v0.2.0"
	Commit  = "none"
	Date    = "unknown"
)
