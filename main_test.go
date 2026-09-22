package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStartupFailures(t *testing.T) {
	// A directory holding no document, to exercise the missing-store path.
	empty := t.TempDir()

	// A directory holding a document the binary is too old to read.
	tooNew := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tooNew, "directory.json"),
		[]byte(`{"schema": 99, "households": []}`),
		0o600,
	))

	tests := []struct {
		name    string
		vars    map[string]string
		wantErr string
	}{
		{
			name:    "invalid port",
			vars:    map[string]string{"WHEREFOLK_PORT": "http", "WHEREFOLK_DATA": empty},
			wantErr: "WHEREFOLK_PORT",
		},
		{
			name:    "missing document",
			vars:    map[string]string{"WHEREFOLK_DATA": empty},
			wantErr: "read document",
		},
		{
			name:    "document newer than the binary",
			vars:    map[string]string{"WHEREFOLK_DATA": tooNew},
			wantErr: "newer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer

			err := run(func(key string) string { return tt.vars[key] }, &logs)

			require.Error(t, err, "startup misconfiguration must be fatal")
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
