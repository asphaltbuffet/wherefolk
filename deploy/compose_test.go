package deploy_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/asphaltbuffet/wherefolk/internal/config"
)

type composeService struct {
	Image       string            `yaml:"image"`
	NetworkMode string            `yaml:"network_mode"`
	Restart     string            `yaml:"restart"`
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Environment map[string]string `yaml:"environment"`
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
	Volumes  map[string]struct {
		Name string `yaml:"name"`
	} `yaml:"volumes"`
}

func loadCompose(t *testing.T) composeFile {
	t.Helper()

	var c composeFile
	require.NoError(t, yaml.Unmarshal([]byte(readDeployFile(t, "compose.yaml")), &c))
	require.Contains(t, c.Services, "tailscale")
	require.Contains(t, c.Services, "wherefolk")

	return c
}

func TestCompose(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, c composeFile)
	}{
		{
			name: "the app joins the sidecar's network namespace, so loopback is shared",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				assert.Equal(t, "service:tailscale", c.Services["wherefolk"].NetworkMode)
			},
		},
		{
			name: "the sidecar has its own network namespace, never the host's",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				assert.Empty(t, c.Services["tailscale"].NetworkMode)
			},
		},
		{
			name: "no service publishes a port",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				for name, svc := range c.Services {
					assert.Empty(t, svc.Ports,
						"%s publishes a port; reaching the app must go through tailscale serve only", name)
				}
			},
		},
		{
			name: "the sidecar image is pinned to a release tag",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				assert.Regexp(t, `^tailscale/tailscale:v\d+\.\d+\.\d+$`,
					c.Services["tailscale"].Image)
			},
		},
		{
			name: "the app runs the published image",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				assert.True(t, strings.HasPrefix(c.Services["wherefolk"].Image, "ghcr.io/asphaltbuffet/wherefolk:"))
			},
		},
		{
			name: "both services restart unless stopped",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				for name, svc := range c.Services {
					assert.Equal(t, "unless-stopped", svc.Restart, name)
				}
			},
		},
		{
			name: "Tailscale node state is on a named volume",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				svc := c.Services["tailscale"]
				assert.Contains(t, svc.Volumes, "tailscale-state:/var/lib/tailscale")
				assert.Equal(t, "/var/lib/tailscale", svc.Environment["TS_STATE_DIR"])
				assert.Equal(t, "wherefolk-tailscale-state", c.Volumes["tailscale-state"].Name)
			},
		},
		{
			name: "the Directory is on a named volume at the configured data directory",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				assert.Contains(t, c.Services["wherefolk"].Volumes, "wherefolk-data:"+config.DefaultDataDir)
				assert.Equal(t, "wherefolk-data", c.Volumes["wherefolk-data"].Name)
			},
		},
		{
			name: "the Full passphrase reaches the app and never the sidecar",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				assert.Contains(t, c.Services["wherefolk"].Environment, "WHEREFOLK_FULL_PASSPHRASE")
				assert.NotContains(t, c.Services["tailscale"].Environment, "WHEREFOLK_FULL_PASSPHRASE")
			},
		},
		{
			name: "the auth key is required rather than defaulted, and the node is tagged",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				env := c.Services["tailscale"].Environment
				assert.True(t, strings.HasPrefix(env["TS_AUTHKEY"], "${TS_AUTHKEY:?"),
					"a missing key must stop `compose up`, not boot an unauthenticated node")
				assert.Equal(t, "--advertise-tags=tag:wherefolk", env["TS_EXTRA_ARGS"])
			},
		},
		{
			name: "the Serve config is mounted where the sidecar looks for it",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				svc := c.Services["tailscale"]
				assert.Equal(t, "/config/serve.json", svc.Environment["TS_SERVE_CONFIG"])
				assert.Contains(t, svc.Volumes, "./tailscale-serve.json:/config/serve.json:ro")
			},
		},
		{
			name: "the app's port is the default the Serve config proxies to",
			check: func(t *testing.T, c composeFile) {
				t.Helper()

				assert.Equal(t, strconv.Itoa(config.DefaultPort), c.Services["wherefolk"].Environment["WHEREFOLK_PORT"])
			},
		},
	}

	c := loadCompose(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, c)
		})
	}
}

func TestEnvExample(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, composeText, exampleText string)
	}{
		{
			name: "documents every variable the compose file interpolates",
			check: func(t *testing.T, composeText, exampleText string) {
				t.Helper()

				vars := regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)`).FindAllStringSubmatch(composeText, -1)
				require.NotEmpty(t, vars)

				for _, m := range vars {
					assert.Regexp(t, `(?m)^`+m[1]+`=`, exampleText, m[1])
				}
			},
		},
		{
			name: "ships a placeholder, not a credential, for the auth key",
			check: func(t *testing.T, _, exampleText string) {
				t.Helper()

				assert.Regexp(t, `(?m)^TS_AUTHKEY=tskey-client-REPLACE_ME`, exampleText)
			},
		},
		{
			name: "keeps the auth key's node non-ephemeral",
			check: func(t *testing.T, _, exampleText string) {
				t.Helper()

				assert.Regexp(t, `(?m)^TS_AUTHKEY=\S*\?ephemeral=false`, exampleText)
			},
		},
		{
			name: "leaves the Full passphrase empty, so Full stays unavailable until one is chosen",
			check: func(t *testing.T, _, exampleText string) {
				t.Helper()

				assert.Regexp(t, `(?m)^WHEREFOLK_FULL_PASSPHRASE=$`, exampleText)
			},
		},
	}

	composeText := readDeployFile(t, "compose.yaml")
	exampleText := readDeployFile(t, ".env.example")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, composeText, exampleText)
		})
	}
}
