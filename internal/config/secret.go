package config

import "log/slog"

// redacted is what a set Secret prints as.
const redacted = "[redacted]"

// Secret is a string that never prints itself. fmt's %v, %+v and %#v, and
// slog, all see "[redacted]" (or nothing, when unset); only Reveal returns the
// value, so every place the secret is actually used is a grep away.
//
// It exists because Config is a plain struct that is easy to log whole while
// debugging, and one such line would put the family passphrase in the
// container's logs.
type Secret string

// String implements [fmt.Stringer], which %v and %s use, including for a
// Secret nested inside a struct.
func (s Secret) String() string {
	if s == "" {
		return ""
	}

	return redacted
}

// GoString implements [fmt.GoStringer], which %#v uses.
func (s Secret) GoString() string { return `config.Secret("` + s.String() + `")` }

// LogValue implements [slog.LogValuer].
func (s Secret) LogValue() slog.Value { return slog.StringValue(s.String()) }

// MarshalText implements [encoding.TextMarshaler], so encoding/json and slog's
// JSONHandler print the redacted form too.
func (s Secret) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Reveal returns the secret itself. Call it only where the value is consumed.
func (s Secret) Reveal() string { return string(s) }
