package render

import (
	"fmt"
	"strconv"
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// dateStamp is how GeneratedAt is formatted, matching wholeDate's spelling so
// a printed Directory reads in one style throughout.
const dateStamp = "January 2, 2006"

// wholeDate renders d with its year, at whatever precision it holds:
// "March 12, 1965", "March 1938" or "1938".
//
// The month is spelled out because the Directory is read by relatives rather
// than parsed, and "03-12" cannot say whether it means March or December.
// rolo.Date.String stays ISO: the store and the editing form depend on it, and
// this spelling is an export concern.
func wholeDate(d rolo.Date) string {
	switch {
	case d.IsZero():
		return ""
	case d.Month == 0:
		return strconv.Itoa(d.Year)
	case d.Day == 0:
		return fmt.Sprintf("%s %d", time.Month(d.Month), d.Year)
	default:
		return fmt.Sprintf("%s %d, %d", time.Month(d.Month), d.Day, d.Year)
	}
}

// truncatedDate renders d without its year: "March 12" or "March".
//
// A year-only date truncates to nothing, because the year is all it holds.
// That empty string is the date's precision, not a suppression, so a caller
// withholding it prints nothing rather than [private].
func truncatedDate(d rolo.Date) string {
	switch {
	case d.Month == 0:
		return ""
	case d.Day == 0:
		return time.Month(d.Month).String()
	default:
		return fmt.Sprintf("%s %d", time.Month(d.Month), d.Day)
	}
}
