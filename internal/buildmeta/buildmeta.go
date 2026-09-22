package buildmeta

import "fmt"

const unknownBuildValue = "unknown"

// These are variables rather than constants because the linker's -X flag can
// only overwrite string variables; a constant would be inlined at compile time
// and the injected value silently discarded.
var (
	// Version is the semantic version (injected via ldflags).
	Version = "dev"

	// GitCommit is the short git commit hash (injected via ldflags).
	GitCommit = unknownBuildValue

	// BuildDate is the build timestamp ISO 8601 (injected via ldflags).
	BuildDate = unknownBuildValue
)

// FullVersion returns the version string including commit hash.
func FullVersion() string {
	return fmt.Sprintf("%s+%s", Version, GitCommit)
}

// ShortVersion returns only the semantic version.
func ShortVersion() string {
	return Version
}

// BuildInfo returns build metadata including commit and timestamp. It returns
// the empty string when nothing was injected, so callers can omit the field
// entirely rather than render "unknown" to an Operator.
func BuildInfo() string {
	if GitCommit == unknownBuildValue && BuildDate == unknownBuildValue {
		return ""
	}

	if BuildDate == unknownBuildValue {
		return GitCommit
	}

	return fmt.Sprintf("%s (%s)", GitCommit, BuildDate)
}
