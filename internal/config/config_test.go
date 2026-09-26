package config_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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
		{
			name: "template directory defaults to the volume",
			vars: map[string]string{},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, "/var/lib/wherefolk/template", got.TemplateDir)
			},
		},
		{
			name: "template directory is overridden",
			vars: map[string]string{"WHEREFOLK_TEMPLATE": "./template"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, "./template", got.TemplateDir)
				assert.Equal(t, "/var/lib/wherefolk", got.DataDir, "data directory keeps its default")
			},
		},
		{
			name: "no passphrase leaves the Full tier unconfigured",
			vars: map[string]string{},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Empty(t, got.FullPassphrase.Reveal())
			},
		},
		{
			name: "passphrase is read",
			vars: map[string]string{"WHEREFOLK_FULL_PASSPHRASE": "correct horse battery"},
			check: func(t *testing.T, got config.Config) {
				t.Helper()

				assert.Equal(t, "correct horse battery", got.FullPassphrase.Reveal())
			},
		},
		{
			name:    "a short passphrase is fatal",
			vars:    map[string]string{"WHEREFOLK_FULL_PASSPHRASE": "abc1234"},
			wantErr: "WHEREFOLK_FULL_PASSPHRASE: shorter than 8 characters",
		},
		{
			name:    "surrounding whitespace is fatal rather than silently trimmed",
			vars:    map[string]string{"WHEREFOLK_FULL_PASSPHRASE": "correct horse battery "},
			wantErr: "WHEREFOLK_FULL_PASSPHRASE: has leading or trailing whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.Load(env(tt.vars))

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				if p := tt.vars["WHEREFOLK_FULL_PASSPHRASE"]; p != "" {
					assert.NotContains(t, err.Error(), p, "the passphrase never reaches an error message")
				}

				return
			}

			require.NoError(t, err)
			tt.check(t, got)
		})
	}
}

func TestSecretNeverPrints(t *testing.T) {
	const value = "correct horse battery"

	cfg := config.Config{FullPassphrase: config.Secret(value)}

	tests := []struct {
		name string
		out  string
	}{
		{name: "%v", out: fmt.Sprintf("%v", cfg)},
		{name: "%+v", out: fmt.Sprintf("%+v", cfg)},
		{name: "%#v", out: fmt.Sprintf("%#v", cfg)},
		{
			name: "%s of the secret",
			//nolint:staticcheck // S1025: verbatim %s verb is the point of this row
			out: fmt.Sprintf("%s", cfg.FullPassphrase),
		},
		{name: "slog", out: func() string {
			var b strings.Builder
			slog.New(slog.NewTextHandler(&b, nil)).Info("cfg", "passphrase", cfg.FullPassphrase)
			return b.String()
		}()},
		{name: "json.Marshal", out: func() string {
			//nolint:musttag // Config is never serialized in production; this only checks redaction.
			b, err := json.Marshal(cfg)
			require.NoError(t, err)
			return string(b)
		}()},
		{name: "slog JSONHandler", out: func() string {
			var b strings.Builder
			slog.New(slog.NewJSONHandler(&b, nil)).Info("cfg", "passphrase", cfg.FullPassphrase)
			return b.String()
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotContains(t, tt.out, value)
		})
	}
}
