package render

import (
	"fmt"
	"strconv"
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// dateStamp is how GeneratedAt is formatted, matching wholeDate's spelling so
// a printed Directory reads in one style throughout.
const dateStamp = "Jan 2, 2006"

// wholeDate renders d with its year, at whatever precision it holds:
// "Mar 12, 1965", "Mar 1938" or "1938".
//
// The month is a word, not a number, because the Directory is read by
// relatives rather than parsed, and "03-12" cannot say whether it means March
// or December. It is abbreviated to keep the dates column narrow.
// rolo.Date.String stays ISO: the store and the editing form depend on it, and
// this spelling is an export concern.
func wholeDate(d rolo.Date) string {
	switch {
	case d.IsZero():
		return ""
	case d.Month == 0:
		return strconv.Itoa(d.Year)
	case d.Day == 0:
		return fmt.Sprintf("%s %d", month(d), d.Year)
	default:
		return fmt.Sprintf("%s %d, %d", month(d), d.Day, d.Year)
	}
}

// truncatedDate renders d without its year: "Mar 12" or "Mar".
//
// A year-only date truncates to nothing, because the year is all it holds.
// That empty string is the date's precision, not a suppression, so a caller
// withholding it prints nothing rather than [private].
func truncatedDate(d rolo.Date) string {
	switch {
	case d.Month == 0:
		return ""
	case d.Day == 0:
		return month(d)
	default:
		return fmt.Sprintf("%s %d", month(d), d.Day)
	}
}

// month is d's month as its three-letter abbreviation — "Jan", "Sep" — the
// same spelling as Go's "Jan" layout element, so dateStamp agrees with it.
func month(d rolo.Date) string {
	return time.Month(d.Month).String()[:3]
}
