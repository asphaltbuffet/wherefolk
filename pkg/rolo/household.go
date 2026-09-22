package rolo

import "strings"

// HouseholdID is the stable identity of a Household. It survives restructuring
// of the tree, unlike the Path, which is a display identity.
type HouseholdID string

// Address is a Household's mailing address, either as its own lines or as a
// reference to another Household's address.
//
// A Shared Address is a reference rather than a copy so that it stays correct
// when the referenced Household moves, and so the Directory can render a
// back-reference instead of repeating the same lines.
type Address struct {
	Lines      []string    `json:"lines,omitempty"`
	SharedWith HouseholdID `json:"shared_with,omitempty"`
}

// Household is the unit that renders as one block in the Directory. A node
// becomes a Household when it has a spouse, has Dependents, or has its own
// Address.
//
// Parent is empty for a root Household. Children are not stored: they are
// derived by grouping on Parent when the Tree is built, so moving a Household
// is a one-field edit that cannot leave two records disagreeing.
type Household struct {
	ID     HouseholdID `json:"id"`
	Parent HouseholdID `json:"parent,omitempty"`

	Adults     []Person `json:"adults"`
	Dependents []Person `json:"dependents,omitempty"`

	Anniversary Date `json:"anniversary"`

	// omitzero for the same reason as Person.Hidden: Address is a struct, and
	// omitempty does not suppress an empty one.
	Address Address `json:"address,omitzero"`
}

// IsMemorial reports whether every adult in the Household is deceased. A
// Memorial Household persists permanently — it anchors the Path of every Branch
// beneath it — and carries no contact details, because it has no living adult
// to own them.
//
// A Household with no adults is not memorial; it is malformed, and the store
// rejects it at load.
func (h Household) IsMemorial() bool {
	if len(h.Adults) == 0 {
		return false
	}
	for _, a := range h.Adults {
		if !a.IsDeceased() {
			return false
		}
	}
	return true
}

// HasLivingMember reports whether anyone in the Household is living, including
// Dependents. Proof Sheets are produced only for Households where this holds.
func (h Household) HasLivingMember() bool {
	for _, p := range h.Adults {
		if !p.IsDeceased() {
			return true
		}
	}
	for _, p := range h.Dependents {
		if !p.IsDeceased() {
			return true
		}
	}
	return false
}

// SharesAddress reports whether this Household's address is a reference to
// another Household's.
func (h Household) SharesAddress() bool { return h.Address.SharedWith != "" }

// Label is the Household's name as it appears in the tree and in a Path:
// the adults' given names joined by a slash, as in "Dave/Diane".
func (h Household) Label() string {
	if len(h.Adults) == 0 {
		return "(" + string(h.ID) + ")"
	}

	names := make([]string, 0, len(h.Adults))
	for _, a := range h.Adults {
		names = append(names, a.Given)
	}
	return strings.Join(names, "/")
}

// EldestAdultBirth returns the earliest known birth date among the adults, or
// the zero Date when none is known. Sibling Households are ordered by this,
// which matches how families list their children.
func (h Household) EldestAdultBirth() Date {
	var eldest Date
	for _, a := range h.Adults {
		if a.Birth.IsZero() {
			continue
		}
		if eldest.IsZero() || dateLess(a.Birth, eldest) {
			eldest = a.Birth
		}
	}
	return eldest
}

// dateLess reports whether a falls before b, comparing at whatever precision
// each date holds. A missing month or day sorts as if it were the first of the
// period.
func dateLess(a, b Date) bool {
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	if a.Month != b.Month {
		return a.Month < b.Month
	}
	return a.Day < b.Day
}
