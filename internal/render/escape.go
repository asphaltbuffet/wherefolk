// Package render turns a Directory into a printable document.
//
// It holds three separable things: a render model of already-rendered strings,
// a generator that emits Typst markup from that model, and a wrapper around the
// typst binary that compiles the markup to PDF or SVG (ADR-0004).
//
// The package applies no tier rules. It renders exactly the strings it is
// handed; withholding, suppression and date truncation (§5.2, §5.3, §5.5) act
// upstream by constructing a different [Directory].
package render

import "strings"

// typstSpecials are the characters Typst reads as markup syntax. A value
// interpolated into generated markup must have each of them escaped, or an
// address line like "#1 Elm St" is parsed as a code expression rather than
// printed.
//
// The backslash is listed first and handled first: escaping it after the others
// would double-escape the backslashes this function itself introduces.
var typstSpecials = []string{`\`, `#`, `$`, `*`, `_`, `@`, `<`, `>`, `[`, `]`}

// escape renders s as literal Typst text.
func escape(s string) string {
	if s == "" {
		return ""
	}

	for _, c := range typstSpecials {
		s = strings.ReplaceAll(s, c, `\`+c)
	}

	return s
}
