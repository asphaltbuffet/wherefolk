package render

// This file exposes unexported identifiers to the external test package,
// matching the convention in internal/web. Tests live in package render_test so
// that they exercise the package as a caller sees it.

// Escape is the test alias for escape.
var Escape = escape
