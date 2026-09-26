// Package config reads the service's settings from the environment.
//
// Only data locations are configurable. The bind interface is deliberately
// absent: it is a package constant in internal/web, because the no-authentication
// design (ADR-0001) holds only while the binary cannot listen beyond loopback.
// See ADR-0007.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// DefaultDataDir is the container's named volume mount point (§2.1).
	DefaultDataDir = "/var/lib/wherefolk"
	// DefaultPort is the port tailscale serve forwards to (§2.2).
	DefaultPort = 8080
	// DefaultLogLevel is the threshold when WHEREFOLK_LOG_LEVEL is unset.
	DefaultLogLevel = slog.LevelInfo
	// DocumentName is the store's filename within the data directory.
	// Item 6's snapshots/ directory lives beside it, which is why WHEREFOLK_DATA
	// names a directory rather than a file (§2.1).
	DocumentName = "directory.json"
	// DefaultTemplateDir is where the Typst layout lives inside the container.
	// It is a directory under the same volume as the document, so the Operator
	// edits the template through the same mount they already have (§2.1,
	// ADR-0004).
	DefaultTemplateDir = "/var/lib/wherefolk/template"

	// MinPassphraseLength is the shortest Full-tier passphrase accepted. The
	// passphrase is read out over the phone to relatives (§2.3), so it is a
	// phrase rather than a password; a short one is a configuration mistake.
	MinPassphraseLength = 8
)

// Config is the service's runtime configuration.
type Config struct {
	DataDir string
	Port    int

	// LogLevel is the threshold main installs on the logger it builds. The
	// logger itself is not configuration: only the level is read from the
	// environment, because only main knows where the log output goes.
	LogLevel slog.Level

	// TemplateDir holds the on-disk Typst layout. The template is not embedded
	// in the binary, so that "the addresses look cramped" is a template edit and
	// a restart rather than a rebuild (ADR-0004).
	TemplateDir string

	// FullPassphrase encrypts every Full-tier export (§5.2, ADR-0011). Empty
	// means the Full tier is unavailable: the service still starts, because a
	// missing passphrase should not take the whole address book offline to
	// protect one of its four exports.
	FullPassphrase Secret
}

// DocumentPath is the JSON store's location on disk.
func (c Config) DocumentPath() string { return filepath.Join(c.DataDir, DocumentName) }

// Load reads configuration from getenv, which is [os.Getenv] in production and a
// map lookup in tests. An unset variable takes its default; an invalid one is an
// error, never a silent fallback.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DataDir:     DefaultDataDir,
		Port:        DefaultPort,
		LogLevel:    DefaultLogLevel,
		TemplateDir: DefaultTemplateDir,
	}

	if dir := getenv("WHEREFOLK_DATA"); dir != "" {
		cfg.DataDir = dir
	}

	if dir := getenv("WHEREFOLK_TEMPLATE"); dir != "" {
		cfg.TemplateDir = dir
	}

	if raw := getenv("WHEREFOLK_PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("WHEREFOLK_PORT: %q is not a number", raw)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("WHEREFOLK_PORT: %d is outside 1-65535", port)
		}
		// Atoi accepts a leading sign and leading zeros, so "+8080" and "08080"
		// would both parse to 8080. A malformed port is fatal rather than
		// silently coerced, so require the canonical spelling: anything that
		// does not render back to itself was not a plain port number.
		if strconv.Itoa(port) != raw {
			return Config{}, fmt.Errorf("WHEREFOLK_PORT: %q is not a plain port number, did you mean %d?", raw, port)
		}
		cfg.Port = port
	}

	if raw := getenv("WHEREFOLK_LOG_LEVEL"); raw != "" {
		// slog.Level parses its own spellings — debug, info, warn, error, and
		// offsets like debug+2 — case-insensitively, so the accepted values stay
		// identical to the ones the logger prints.
		var level slog.Level

		err := level.UnmarshalText([]byte(raw))
		if err != nil {
			return Config{}, fmt.Errorf("WHEREFOLK_LOG_LEVEL: %q is not a log level", raw)
		}

		cfg.LogLevel = level
	}

	if raw := getenv("WHEREFOLK_FULL_PASSPHRASE"); raw != "" {
		// Neither error echoes the value: this is the one variable whose
		// content must never reach the logs.
		if raw != strings.TrimSpace(raw) {
			return Config{}, errors.New("WHEREFOLK_FULL_PASSPHRASE: has leading or trailing whitespace")
		}

		if len([]rune(raw)) < MinPassphraseLength {
			return Config{}, fmt.Errorf("WHEREFOLK_FULL_PASSPHRASE: shorter than %d characters", MinPassphraseLength)
		}

		cfg.FullPassphrase = Secret(raw)
	}

	return cfg, nil
}
