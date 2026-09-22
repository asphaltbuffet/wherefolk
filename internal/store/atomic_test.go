package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic(t *testing.T) {
	tests := []struct {
		name      string
		existing  string
		write     string
		checkFunc func(t *testing.T, path string)
	}{
		{
			name:  "creates a new file",
			write: `{"schema":1}`,
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				b, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, `{"schema":1}`, string(b))
			},
		},
		{
			name:     "replaces an existing file",
			existing: `{"schema":1,"old":true}`,
			write:    `{"schema":2}`,
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				b, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, `{"schema":2}`, string(b))
			},
		},
		{
			name:     "shrinking the file leaves no trailing bytes",
			existing: strings.Repeat("x", 4096),
			write:    "small",
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				b, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, "small", string(b))
			},
		},
		{
			name:  "leaves no temp files behind",
			write: `{"schema":1}`,
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				entries, err := os.ReadDir(filepath.Dir(path))
				require.NoError(t, err)
				assert.Len(t, entries, 1, "expected only the target file to remain")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "directory.json")

			if tt.existing != "" {
				require.NoError(t, os.WriteFile(path, []byte(tt.existing), 0o600))
			}

			require.NoError(t, writeFileAtomic(path, []byte(tt.write)))
			tt.checkFunc(t, path)
		})
	}
}

func TestWriteFileAtomicSetsPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directory.json")

	require.NoError(t, writeFileAtomic(path, []byte("data")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"the document holds relatives' addresses and must not be world-readable")
}

func TestWriteFileAtomicFailsOnMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "directory.json")

	err := writeFileAtomic(path, []byte("data"))
	require.Error(t, err)
}
