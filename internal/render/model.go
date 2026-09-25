package render

// Person is one person as the Directory prints them.
//
// Every field is an already-rendered string, never a rolo value. That is the
// same discipline internal/web/view.go follows and for the same reason: a tier
// rule cannot be forgotten about a value that never arrives here as a date or a
// flag. An empty field prints nothing.
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
	// Label is the household's heading, e.g. "Robert/Susan".
	Label string

	// Memorial marks a Household whose adults have all died. It renders more
	// compactly — a heading with dates rather than a full entry — because there
	// is nothing in it to act on, but it must appear in every tier so that
	// descendants group beneath it and no Path points at a missing node (§5.4).
	Memorial bool

	// AddressLines is the Household's own address. Empty when it has none, and
	// also when it shares another Household's, in which case SharedWith is set.
	AddressLines []string

	// SharedWith is the label of the Household whose address this one uses. A
	// shared Address renders as a back-reference rather than a repeated block
	// (§3), so the two fields are never both populated.
	SharedWith string

	Anniversary string

	Adults     []Person
	Dependents []Person
}

// Directory is the whole printable document.
type Directory struct {
	// GeneratedAt is the stamp every export carries. Age gating is computed at
	// export time, so the same Directory exported months apart differs as people
	// turn 18 — the date is what makes that legible (§5.7).
	GeneratedAt string

	Households []Household
}
