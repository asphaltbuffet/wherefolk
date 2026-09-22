// Package rolo holds the Directory's domain types: the Person and Household
// records, the Tree derived from their Parent links, and the normalisation and
// validation rules applied to them.
//
// Nothing here persists anything. Loading and saving live in internal/store, so
// these types can be reasoned about without a filesystem. See CONTEXT.md for the
// vocabulary and docs/adr/ for the decisions behind the shape.
package rolo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a calendar date of partial precision. A Date may be fully specified
// (1938-03-12), know only a year and month (1938-03), know only a year (1938),
// or be unknown entirely (the zero Date).
//
// Partial precision is a domain requirement, not a convenience: the export
// tiers truncate a living person's date to month and day, and many ancestors
// are recorded with only a year.
// encoding/json forces the receiver split: UnmarshalJSON must take a pointer to
// write through, while MarshalJSON stays on the value so it is also used for
// non-addressable Dates. Every other method is a query on a 3-int value type.
//
//nolint:recvcheck // mixed receivers are required by encoding/json; see above
type Date struct {
	Year  int
	Month int
	Day   int
}

// ParseDate reads a date in YYYY, YYYY-MM, or YYYY-MM-DD form. The empty
// string yields the zero Date, which represents an unknown date rather than an
// error.
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, nil
	}

	// A date is at most year-month-day; anything longer is not a date.
	const maxDateParts = 3

	parts := strings.Split(s, "-")
	if len(parts) > maxDateParts {
		return Date{}, fmt.Errorf("parse date %q: too many components", s)
	}

	var d Date
	fields := []*int{&d.Year, &d.Month, &d.Day}
	widths := []int{4, 2, 2}

	for i, p := range parts {
		if len(p) != widths[i] {
			return Date{}, fmt.Errorf("parse date %q: component %q is not %d digits", s, p, widths[i])
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Date{}, fmt.Errorf("parse date %q: %w", s, err)
		}
		*fields[i] = n
	}

	if d.Month != 0 && (d.Month < 1 || d.Month > 12) {
		return Date{}, fmt.Errorf("parse date %q: month out of range", s)
	}

	// Verify the day exists in that month. time.Date normalises overflow
	// (Feb 30 becomes Mar 2), so a round-trip mismatch means the day was invalid.
	if d.Day != 0 {
		t := time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
		if t.Year() != d.Year || int(t.Month()) != d.Month || t.Day() != d.Day {
			return Date{}, fmt.Errorf("parse date %q: day out of range for month", s)
		}
	}

	return d, nil
}

// IsZero reports whether the date is unknown.
func (d Date) IsZero() bool { return d.Year == 0 && d.Month == 0 && d.Day == 0 }

// HasYear reports whether the year is known.
func (d Date) HasYear() bool { return d.Year != 0 }

// String renders the date at whatever precision it holds. The zero Date
// renders as the empty string.
func (d Date) String() string {
	switch {
	case d.IsZero():
		return ""
	case d.Month == 0:
		return fmt.Sprintf("%04d", d.Year)
	case d.Day == 0:
		return fmt.Sprintf("%04d-%02d", d.Year, d.Month)
	default:
		return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	}
}

// MarshalJSON renders the date as a string, so the stored document stays
// readable and hand-editable.
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON parses a date string.
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
