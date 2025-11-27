// Package version provides version information for the ndiff binary.
// This package is in the 'pkg' directory because it could theoretically be imported
// by other projects, unlike the 'internal' packages which are private to this project.
package version

import "fmt"

// These variables are set at build time using ldflags
// See the Makefile for how these are injected during the build process
var (
	// Version is the semantic version of the binary (e.g., "1.0.0")
	// Set via: -ldflags "-X github.com/wlame/ndiff/pkg/version.Version=1.0.0"
	Version = "dev"

	// Commit is the git commit hash the binary was built from
	// Set via: -ldflags "-X github.com/wlame/ndiff/pkg/version.Commit=abc123"
	Commit = "unknown"

	// BuildTime is when the binary was built (RFC3339 format)
	// Set via: -ldflags "-X github.com/wlame/ndiff/pkg/version.BuildTime=2025-11-26T10:30:00Z"
	BuildTime = "unknown"
)

// Info represents the version information for the application
// This struct is useful for serializing version info to JSON or other formats
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// Get returns the version information as a structured Info object
// Example usage:
//   info := version.Get()
//   fmt.Printf("Version: %s\n", info.Version)
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
	}
}

// String returns a human-readable version string
// Format: "ndiff version <version> (commit: <commit>, built: <buildtime>)"
// Example output: "ndiff version 1.0.0 (commit: abc123, built: 2025-11-26T10:30:00Z)"
func String() string {
	return fmt.Sprintf("ndiff version %s (commit: %s, built: %s)",
		Version, Commit, BuildTime)
}

// Short returns a short version string with just the version number
// Example: "1.0.0" or "dev"
func Short() string {
	return Version
}
