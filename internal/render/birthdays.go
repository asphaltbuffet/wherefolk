package render

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// birthdays builds the Birthday Calendar from every person in t (CONTEXT.md,
// Birthday Calendar).
//
// A person appears when they are living, their birth month is known and their
// birth date is not withheld. A withheld date gets no row rather than a
// [private] one: on a page listing only birthdays, a marker would single that
// person out and add nothing. The deceased never appear, because a row reads as
// a reminder to send a card.
//
// The calendar is the same in every tier. A row shows a month and day, which
// is exactly what a Truncated Date already shows everywhere, and minors'
// birth dates are not suppressed (§5.2). It belongs to a Directory only: a
// Proof Sheet shows one Household's own entry, and must never carry the whole
// family's birthdays.
//
// Rows are ordered by surname and then given name, ignoring case, so
// "de Groot" sorts among the Ds. A person with no surname sorts by their given
// name, where the reader would look for the name that prints; equal keys keep
// Directory order.
func birthdays(t *rolo.Tree) []Birthday {
	type row struct {
		surname, given string
		out            Birthday
	}

	var rows []row

	// The callback never errors, so the returned error is always nil.
	_ = t.Walk(func(h rolo.Household, _ int) error {
		for _, people := range [][]rolo.Person{h.Adults, h.Dependents} {
			for _, p := range people {
				if p.IsDeceased() || p.Hidden.Birth || p.Birth.Month == 0 {
					continue
				}

				day := "?"
				if p.Birth.Day != 0 {
					day = strconv.Itoa(p.Birth.Day)
				}

				surname := p.Surname
				if surname == "" {
					surname = p.Given
				}

				rows = append(rows, row{
					surname: strings.ToLower(surname),
					given:   strings.ToLower(p.Given),
					out: Birthday{
						Name:        calendarName(p),
						HouseholdID: string(h.ID),
						Month:       p.Birth.Month,
						Day:         day,
					},
				})
			}
		}

		return nil
	})

	if len(rows) == 0 {
		return nil
	}

	slices.SortStableFunc(rows, func(a, b row) int {
		return cmp.Or(cmp.Compare(a.surname, b.surname), cmp.Compare(a.given, b.given))
	})

	out := make([]Birthday, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.out)
	}

	return out
}

// calendarName renders p surname first, as the Birthday Calendar lists them:
// `Weldy, Katelynn "Katie" (Birch)`. The given name and nickname are exactly
// as p's own row prints them. A birth name follows in parentheses, as in the
// Household Name, when it differs from the surname: it lets a relative find
// someone by the name they grew up with, and tells apart two people who share
// a name.
func calendarName(p rolo.Person) string {
	birthName := ""
	if p.BirthName != "" && p.BirthName != p.Surname {
		birthName = "(" + p.BirthName + ")"
	}

	given := joinWords(p.Given, nickname(p), birthName)

	switch {
	case p.Surname == "":
		return given
	case given == "":
		return p.Surname
	default:
		return p.Surname + ", " + given
	}
}
