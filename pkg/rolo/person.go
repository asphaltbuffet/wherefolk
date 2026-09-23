package rolo

import (
	"strings"
	"time"
)

// PersonID is the stable identity of a Person. It never changes and is never
// shown to the Editor; the human-readable identity of someone in the Directory
// is their Household's Path.
type PersonID string

// HiddenFields records which of a Person's fields the Editor has withheld from
// every export. A withheld field renders as "[private]" so that withheld data
// is distinguishable from data that was never collected.
type HiddenFields struct {
	Phone bool `json:"phone,omitempty"`
	Email bool `json:"email,omitempty"`
	Birth bool `json:"birth,omitempty"`
}

// Person is a single human being in the Directory. Every Person belongs to
// exactly one Household, either as one of its adults or as one of its
// Dependents.
// The queries below are value-semantic and only Normalize mutates, so only
// Normalize takes a pointer. Household.Normalize therefore iterates by index
// rather than by range copy, which would discard the result.
//
//nolint:recvcheck // mixed receivers are deliberate; see above
type Person struct {
	ID PersonID `json:"id"`

	Given     string `json:"given"`
	Surname   string `json:"surname"`
	BirthName string `json:"birth_name,omitempty"`
	Aka       string `json:"aka,omitempty"`

	Birth Date `json:"birth"`
	Death Date `json:"death"`

	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`

	// omitzero, not omitempty: encoding/json's omitempty has no effect on a
	// struct field, so omitempty would write "hidden": {} for every Person and
	// clutter a document the Operator repairs by hand. omitzero (Go 1.24+)
	// omits the field when the struct is its zero value.
	Hidden HiddenFields `json:"hidden,omitzero"`
}

// IsDeceased reports whether a death date has been recorded. Death is a
// property of a Person, never of a Household.
func (p Person) IsDeceased() bool { return !p.Death.IsZero() }

// IsMinor reports whether the Person is under 18 at asOf.
//
// The reference time is a parameter rather than [time.Now] so that the caller
// sees the time dependency: age gating means the same Directory exported
// months apart differs as people turn 18.
//
// A Person with no recorded birth date is treated as a minor. This fails
// closed — it suppresses contact details for elderly relatives whose birth year
// nobody knows, which export surfaces as a warning rather than hiding.
func (p Person) IsMinor(asOf time.Time) bool {
	if p.Birth.IsZero() {
		return true
	}

	// A partial date is read at its earliest possible instant: year-only 2008
	// becomes 2008-01-01, which is the oldest the person could be. That is the
	// generous reading, but it only applies where a year is known, and the
	// fail-closed default already covers the unknown case.
	month := p.Birth.Month
	if month == 0 {
		month = 1
	}
	day := p.Birth.Day
	if day == 0 {
		day = 1
	}

	eighteenth := time.Date(p.Birth.Year+18, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return asOf.Before(eighteenth)
}

// DisplayName renders the Person's name, with any nickname double-quoted
// between the given name and the surname.
func (p Person) DisplayName() string {
	// Given, "Aka", Surname — the widest a display name gets.
	const maxNameParts = 3

	parts := make([]string, 0, maxNameParts)
	if p.Given != "" {
		parts = append(parts, p.Given)
	}
	if p.Aka != "" {
		parts = append(parts, `"`+p.Aka+`"`)
	}
	if p.Surname != "" {
		parts = append(parts, p.Surname)
	}
	return strings.Join(parts, " ")
}
