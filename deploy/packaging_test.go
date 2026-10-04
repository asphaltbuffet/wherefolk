package deploy_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readRepoFile reads a file relative to the repository root. Tests run in the
// package directory, which sits one level below it.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("..", rel))
	require.NoError(t, err)

	return string(b)
}

// readDeployFile reads a file in deploy/, the test's working directory.
func readDeployFile(t *testing.T, name string) string {
	t.Helper()

	b, err := os.ReadFile(name)
	require.NoError(t, err)

	return string(b)
}

func TestPackaging(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		check func(t *testing.T, content string)
	}{
		{
			name: "the image creates its data directory, owned by the service user, before declaring the volume",
			file: "Dockerfile",
			check: func(t *testing.T, content string) {
				t.Helper()

				create := strings.Index(content, "install -d -o nonroot -g nonroot /var/lib/wherefolk")
				volume := strings.Index(content, "VOLUME [")

				require.GreaterOrEqual(t, create, 0, "a new named volume copies ownership from the image; "+
					"without this the volume is root-owned and the service cannot save")
				require.GreaterOrEqual(t, volume, 0)
				assert.Less(t, create, volume, "the directory must exist before VOLUME freezes its contents")
			},
		},
		{
			name: "the secrets file is ignored by version control",
			file: ".gitignore",
			check: func(t *testing.T, content string) {
				t.Helper()

				assert.Regexp(t, `(?m)^deploy/\.env$`, content)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, readRepoFile(t, tt.file))
		})
	}
}
