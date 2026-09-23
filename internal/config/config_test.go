package config_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
)

// env builds a getenv function over a map, so tests stay pure.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		vars    map[string]string
		wantErr string
		check   func(t *testing.T, got config.Config)
	}{
		{
			name: "empty environment uses defaults",
			vars: map[string]string{},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, "/var/lib/wherefolk", got.DataDir)
				assert.Equal(t, 8080, got.Port)
			},
		},
		{
			name: "data directory is overridden",
			vars: map[string]string{"WHEREFOLK_DATA": "/tmp/wf"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, "/tmp/wf", got.DataDir)
				assert.Equal(t, 8080, got.Port, "port keeps its default")
			},
		},
		{
			name: "port is overridden",
			vars: map[string]string{"WHEREFOLK_PORT": "9090"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, 9090, got.Port)
			},
		},
		{
			name:    "unparseable port is fatal",
			vars:    map[string]string{"WHEREFOLK_PORT": "http"},
			wantErr: "WHEREFOLK_PORT",
		},
		{
			name:    "port below range is fatal",
			vars:    map[string]string{"WHEREFOLK_PORT": "0"},
			wantErr: "WHEREFOLK_PORT",
		},
		{
			name: "log level defaults to info",
			vars: map[string]string{},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, slog.LevelInfo, got.LogLevel)
			},
		},
		{
			name: "log level is overridden",
			vars: map[string]string{"WHEREFOLK_LOG_LEVEL": "debug"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, slog.LevelDebug, got.LogLevel)
			},
		},
		{
			name: "log level is case-insensitive",
			vars: map[string]string{"WHEREFOLK_LOG_LEVEL": "WARN"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, slog.LevelWarn, got.LogLevel)
			},
		},
		{
			name: "log level accepts error",
			vars: map[string]string{"WHEREFOLK_LOG_LEVEL": "error"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, slog.LevelError, got.LogLevel)
			},
		},
		{
			name:    "unknown log level is fatal",
			vars:    map[string]string{"WHEREFOLK_LOG_LEVEL": "chatty"},
			wantErr: "WHEREFOLK_LOG_LEVEL",
		},
		{
			name:    "port above range is fatal",
			vars:    map[string]string{"WHEREFOLK_PORT": "70000"},
			wantErr: "WHEREFOLK_PORT",
		},
		{
			name:    "signed port is fatal",
			vars:    map[string]string{"WHEREFOLK_PORT": "+8080"},
			wantErr: "WHEREFOLK_PORT",
		},
		{
			name:    "zero-padded port is fatal",
			vars:    map[string]string{"WHEREFOLK_PORT": "08080"},
			wantErr: "WHEREFOLK_PORT",
		},
		{
			name: "lowest valid port is accepted",
			vars: map[string]string{"WHEREFOLK_PORT": "1"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, 1, got.Port)
			},
		},
		{
			name: "highest valid port is accepted",
			vars: map[string]string{"WHEREFOLK_PORT": "65535"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, 65535, got.Port)
			},
		},
		{
			name: "document path derives from the data directory",
			vars: map[string]string{"WHEREFOLK_DATA": "/tmp/wf"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, "/tmp/wf/directory.json", got.DocumentPath())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.Load(env(tt.vars))

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			tt.check(t, got)
		})
	}
}
