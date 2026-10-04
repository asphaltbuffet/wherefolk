package deploy_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
)

// serveHost is the key tailscaled expands from ${TS_CERT_DOMAIN} to the node's
// own certificate domain, so the file works for any tailnet without edits.
const serveHost = "${TS_CERT_DOMAIN}:443"

// serveConfig is the subset of Tailscale's ServeConfig this file may contain.
// loadServeConfig rejects any other key, so a misspelt "AllowFunnel" cannot
// pass for an assertion that Funnel is off.
type serveConfig struct {
	TCP map[string]struct {
		HTTPS bool `json:"HTTPS"`
	} `json:"TCP"`
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy"`
		} `json:"Handlers"`
	} `json:"Web"`
	AllowFunnel map[string]bool `json:"AllowFunnel"`
}

func loadServeConfig(t *testing.T) serveConfig {
	t.Helper()

	dec := json.NewDecoder(strings.NewReader(readDeployFile(t, "tailscale-serve.json")))
	dec.DisallowUnknownFields()

	var cfg serveConfig
	require.NoError(t, dec.Decode(&cfg))

	return cfg
}

func TestServeConfig(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, cfg serveConfig)
	}{
		{
			name: "HTTPS terminates on 443",
			check: func(t *testing.T, cfg serveConfig) {
				t.Helper()

				assert.True(t, cfg.TCP["443"].HTTPS)
			},
		},
		{
			name: "the only published host is the node's own certificate domain",
			check: func(t *testing.T, cfg serveConfig) {
				t.Helper()

				require.Len(t, cfg.Web, 1)
				assert.Contains(t, cfg.Web, serveHost)
			},
		},
		{
			name: "Funnel is explicitly off for every published host",
			check: func(t *testing.T, cfg serveConfig) {
				t.Helper()

				require.NotEmpty(t, cfg.Web)

				for host := range cfg.Web {
					on, present := cfg.AllowFunnel[host]
					assert.True(t, present, "%s has no AllowFunnel entry: absent is not asserted-off", host)
					assert.False(t, on, "%s has Funnel on, which publishes the Directory to the internet", host)
				}
			},
		},
		{
			name: "no host anywhere has Funnel on",
			check: func(t *testing.T, cfg serveConfig) {
				t.Helper()

				for host, on := range cfg.AllowFunnel {
					assert.False(t, on, host)
				}
			},
		},
		{
			name: "the root path is the only handler and proxies to the app on loopback",
			check: func(t *testing.T, cfg serveConfig) {
				t.Helper()

				handlers := cfg.Web[serveHost].Handlers
				require.Len(t, handlers, 1)
				assert.Equal(t, fmt.Sprintf("http://127.0.0.1:%d", config.DefaultPort), handlers["/"].Proxy)
			},
		},
	}

	cfg := loadServeConfig(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, cfg)
		})
	}
}
