// Package config reads the service's settings from the environment.
//
// Only data locations are configurable. The bind interface is deliberately
// absent: it is a package constant in internal/web, because the no-authentication
// design (ADR-0001) holds only while the binary cannot listen beyond loopback.
// See ADR-0007.
package config

import (
	"fmt"
	"path/filepath"
	"strconv"
)

const (
	// DefaultDataDir is the container's named volume mount point (§2.1).
	DefaultDataDir = "/var/lib/wherefolk"
	// DefaultPort is the port tailscale serve forwards to (§2.2).
	DefaultPort = 8080
	// DocumentName is the store's filename within the data directory.
	// Item 6's snapshots/ directory lives beside it, which is why WHEREFOLK_DATA
	// names a directory rather than a file (§2.1).
	DocumentName = "directory.json"
)

// Config is the service's runtime configuration.
type Config struct {
	DataDir string
	Port    int
}

// DocumentPath is the JSON store's location on disk.
func (c Config) DocumentPath() string { return filepath.Join(c.DataDir, DocumentName) }

// Load reads configuration from getenv, which is os.Getenv in production and a
// map lookup in tests. An unset variable takes its default; an invalid one is an
// error, never a silent fallback.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{DataDir: DefaultDataDir, Port: DefaultPort}

	if dir := getenv("WHEREFOLK_DATA"); dir != "" {
		cfg.DataDir = dir
	}

	if raw := getenv("WHEREFOLK_PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("WHEREFOLK_PORT: %q is not a number", raw)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("WHEREFOLK_PORT: %d is outside 1-65535", port)
		}
		cfg.Port = port
	}

	return cfg, nil
}
