package rolo

import (
	"fmt"
	"net/mail"
	"strings"
)

// Severity says how much attention a Finding deserves. Neither level blocks
// anything: validation is an observation, never a rejection.
type Severity int

const (
	// SeverityNotice marks something the Editor probably meant, which will
	// simply appear in the Directory as typed.
	SeverityNotice Severity = iota

	// SeverityWarning marks something likely to be a mistake — a date that
	// runs backwards, an address that cannot be parsed.
	SeverityWarning
)

// String renders the severity for display.
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	default:
		return "notice"
	}
}

// Finding is one observation about the Directory's contents, carrying enough
// location for the UI to show it beside the field it concerns.
//
// Person is empty when the finding is about the Household itself, such as an
// Anniversary.
type Finding struct {
	Household HouseholdID
	Person    PersonID
	Field     string
	Message   string
	Severity  Severity
}

// ValidateHouseholds inspects every Household and reports what looks wrong. It
// changes nothing and rejects nothing.
//
// It stays silent about data that is merely absent. Most people in the
// Directory have no email, and a missing birth date is a fact with an export
// consequence rather than an error — export surfaces that, so nagging about it
// during editing would train the Editor to ignore findings.
//
// Findings arrive in document order so the UI can show them against the tree
// without sorting.
func ValidateHouseholds(households []Household) []Finding {
	var findings []Finding

	for _, h := range households {
		for _, p := range h.Adults {
			findings = append(findings, validatePerson(h.ID, p)...)
		}
		for _, p := range h.Dependents {
			findings = append(findings, validatePerson(h.ID, p)...)
		}
		findings = append(findings, validateAnniversary(h)...)
	}

	return findings
}

// validatePerson reports on one Person's contact details and dates.
func validatePerson(household HouseholdID, p Person) []Finding {
	var findings []Finding

	add := func(field, message string, severity Severity) {
		findings = append(findings, Finding{
			Household: household,
			Person:    p.ID,
			Field:     field,
			Message:   message,
			Severity:  severity,
		})
	}

	if p.Phone != "" {
		if _, recognized := NormalizePhone(p.Phone); !recognized {
			add("phone", fmt.Sprintf("%q is not a standard phone number, so it will print exactly as written", p.Phone), SeverityNotice)
		}
	}

	if p.Email != "" {
		if err := checkEmail(p.Email); err != nil {
			add("email", fmt.Sprintf("%q does not look like an email address: %s", p.Email, err), SeverityWarning)
		}
	}

	// Both dates must be known before they can be compared. A year-only date
	// is compared at its earliest instant, so a birth and a death in the same
	// year is not backwards.
	if !p.Birth.IsZero() && !p.Death.IsZero() && dateLess(p.Death, p.Birth) {
		add("death", fmt.Sprintf("died %s but was born %s", p.Death, p.Birth), SeverityWarning)
	}

	return findings
}

// validateAnniversary reports an Anniversary that predates an adult's birth.
func validateAnniversary(h Household) []Finding {
	if h.Anniversary.IsZero() {
		return nil
	}

	for _, p := range h.Adults {
		if p.Birth.IsZero() {
			continue
		}
		if dateLess(h.Anniversary, p.Birth) {
			return []Finding{{
				Household: h.ID,
				Field:     "anniversary",
				Message: fmt.Sprintf("the anniversary %s is before %s was born (%s)",
					h.Anniversary, strings.TrimSpace(p.Given+" "+p.Surname), p.Birth),
				Severity: SeverityWarning,
			}}
		}
	}

	return nil
}

// checkEmail reports whether a value is a plain email address.
//
// net/mail.ParseAddress also accepts the display-name form, Pat Novak
// <pat@example.com>, which is not what belongs in a directory's email field —
// so an address that parses but carries a name is rejected here.
func checkEmail(s string) error {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return err
	}
	if addr.Name != "" {
		return fmt.Errorf("it includes a name; enter only the address, %s", addr.Address)
	}
	return nil
}
