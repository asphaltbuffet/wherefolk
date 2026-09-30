// Package render turns a Directory into a printable document.
//
// It holds three separable things: a render model of already-rendered strings,
// a generator that emits Typst markup from that model, and a wrapper around the
// typst binary that compiles the markup to PDF or SVG (ADR-0004).
//
// Tier rules — withholding, suppression, age gating and date truncation (§5.2,
// §5.3, §5.5, §5.7) — are applied in exactly one place: [Build], while the
// values are still rolo types. Markup and the template render exactly the
// strings they are handed.
//
// Every value reaches Typst inside a string literal, so quote in markup.go is
// the only escaper the package needs: within a literal, Typst's markup
// metacharacters are ordinary text. See quote's doc comment for the evidence.
package render

// Person is one person as the Directory prints them.
//
// Every field is an already-rendered string, never a rolo value. That is the
// same discipline internal/web/view.go follows and for the same reason: a tier
// rule cannot be forgotten about a value that never arrives here as a date or a
// flag. An empty field prints nothing.
//
// Name is the row name: the surname is dropped when the Household Name already
// carries it, so a row reads "Dawn" beneath "Daryl & Dawn (Mitchell) Yoder".
type Person struct {
	Name  string
	Birth string
	Death string
	Phone string
	Email string
}

// Household is one block of the printed Directory.
//
// Blocks are a flat sequence: a Household never nests inside its parent's block
// regardless of depth (ADR-0002, §5.1).
type Household struct {
	// Name is the Household Name the block prints under, e.g.
	// "Daryl & Dawn (Mitchell) Yoder" (CONTEXT.md). It is not rolo's Label:
	// that compact "Daryl/Dawn" form is a Path segment, for navigation, and
	// stays in the editing UI.
	Name string

	// Memorial marks a Household whose adults have all died. It renders more
	// quietly — grey, with no contact details — because there is nothing in it
	// to act on, but it must appear in every tier so that descendants group
	// beneath it and no Path points at a missing node (§5.4).
	Memorial bool

	// Contents marks a Household the Table of Contents lists: a root, or a
	// child of a root (CONTEXT.md). A deeper Household's block follows its
	// ancestor's, so the reader finds it from that entry. It is a layout flag,
	// not a rule about what the audience may see: every tier lists the same
	// Households, because every tier prints every Household.
	Contents bool

	// AddressLines is the Household's own address. Empty when it has none, and
	// also when it shares another Household's, in which case SharedWith is set.
	AddressLines []string

	// SharedWith is the Household Name of the Household whose address this one
	// uses, matching the heading the reader will look for. A shared Address
	// renders as a back-reference rather than a repeated block (§3), so the two
	// fields are never both populated.
	SharedWith string

	Anniversary string

	Adults     []Person
	Dependents []Person
}

// Birthday is one row of the Birthday Calendar (CONTEXT.md): a living person,
// surname first, and the day of their birthday in its month.
//
// Month is a column number, 1–12, not a date: which month a person's row marks
// is layout, and every rule about whether they appear at all was applied in
// Build. Day is "15", or "?" when the month is known without its day.
type Birthday struct {
	Name  string
	Month int
	Day   string
}

// Directory is the whole printable document.
type Directory struct {
	// Title is the Directory Title the Title page and the PDF's metadata
	// carry. Build supplies DefaultTitle when the Editor has set none, so it is
	// never empty in a built Directory.
	Title string

	// GeneratedAt is the stamp every export carries. Age gating is computed at
	// export time, so the same Directory exported months apart differs as people
	// turn 18 — the date is what makes that legible (§5.7).
	GeneratedAt string

	// Tier names the audience in the footer (§5.8). Distribution is the real
	// control over an exported file, and a named tier is the social pressure
	// that goes with it.
	Tier string

	// Restricted puts DO NOT DISTRIBUTE on every page. Only Full sets it (§5.2).
	Restricted bool

	Households []Household

	// Birthdays is the Birthday Calendar printed after the Households, already
	// filtered and ordered. Empty when nobody qualifies, and then no calendar
	// prints at all.
	Birthdays []Birthday
}
