package version

import (
	"runtime/debug"
	"strings"
)

// Version is the application version string, typically set via -ldflags by goreleaser to the tag value (e.g., v1.2.3).
// It defaults to empty for local builds.
var Version string //nolint:gochecknoglobals

// Commit is the short git commit SHA. It may be set via -ldflags by goreleaser. Defaults to empty for local builds.
var Commit string //nolint:gochecknoglobals

// Date is the build date (RFC3339) typically set by goreleaser via -ldflags. Defaults to empty for local builds.
var Date string //nolint:gochecknoglobals

// String returns a human-friendly version string for the CLI.
// Priority:
//  1. Version (if provided via ldflags, e.g., a tag like v1.2.3)
//  2. Commit (if provided via ldflags), + "dev-" prefix
//  3. Go module version (if no "-" / "+" indicating dirty/unreleased)
//  4. Short SHA from Go build info (vcs.revision), if present, + "dev-" prefix
//  5. "dev"
func String() string {
	if v := strings.TrimSpace(Version); v != "" {
		return v
	}

	if c := strings.TrimSpace(Commit); c != "" {
		return shaVersion(c)
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	goVersion := buildInfo.Main.Version

	if goVersion != "" && !strings.Contains(goVersion, "-") && !strings.Contains(goVersion, "+") {
		return strings.TrimPrefix(goVersion, "v")
	}

	for _, s := range buildInfo.Settings {
		if s.Key == "vcs.revision" {
			if s.Value != "" {
				return shaVersion(s.Value)
			}
		}
	}

	return "dev"
}

func shaVersion(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) >= 7 {
		return "dev-" + sha[:7]
	}

	return "dev-" + sha
}
