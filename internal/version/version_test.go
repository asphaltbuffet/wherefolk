package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFullVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		gitCommit string
		want      string
	}{
		{name: "defaults", version: "dev", gitCommit: unknownBuildValue, want: "dev+unknown"},
		{name: "released", version: "v1.2.3", gitCommit: "abc1234", want: "v1.2.3+abc1234"},
		{name: "commit only", version: "dev", gitCommit: "deadbee", want: "dev+deadbee"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := set(tt.version, tt.gitCommit, BuildDate)
			t.Cleanup(restore)

			assert.Equal(t, tt.want, FullVersion())
		})
	}
}

func TestShortVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "default", version: "dev", want: "dev"},
		{name: "released", version: "v1.2.3", want: "v1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := set(tt.version, GitCommit, BuildDate)
			t.Cleanup(restore)

			assert.Equal(t, tt.want, ShortVersion())
		})
	}
}

func TestBuildInfo(t *testing.T) {
	tests := []struct {
		name      string
		gitCommit string
		buildDate string
		want      string
	}{
		{
			name:      "no build metadata at all",
			gitCommit: unknownBuildValue,
			buildDate: unknownBuildValue,
			want:      "",
		},
		{
			name:      "commit without date",
			gitCommit: "abc1234",
			buildDate: unknownBuildValue,
			want:      "abc1234",
		},
		{
			name:      "commit and date",
			gitCommit: "abc1234",
			buildDate: "2026-09-22T00:00:00Z",
			want:      "abc1234 (2026-09-22T00:00:00Z)",
		},
		{
			name:      "date without commit",
			gitCommit: unknownBuildValue,
			buildDate: "2026-09-22T00:00:00Z",
			want:      "unknown (2026-09-22T00:00:00Z)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := set(Version, tt.gitCommit, tt.buildDate)
			t.Cleanup(restore)

			assert.Equal(t, tt.want, BuildInfo())
		})
	}
}

// set overwrites the ldflag-injected variables and returns a function restoring
// their previous values. It stands in for the linker so tests can exercise the
// shapes a real build produces.
func set(version, gitCommit, buildDate string) func() {
	prevVersion, prevCommit, prevDate := Version, GitCommit, BuildDate
	Version, GitCommit, BuildDate = version, gitCommit, buildDate

	return func() {
		Version, GitCommit, BuildDate = prevVersion, prevCommit, prevDate
	}
}
