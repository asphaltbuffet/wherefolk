package render

import (
	"strings"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// householdName renders h's Household Name: the heading a reader addressing a
// card looks for (CONTEXT.md, Household Name).
//
// Adults sharing a surname print it once, last, each carrying a different
// birth name in parentheses: "Daryl & Dawn (Mitchell) Yoder". Adults whose
// surnames differ are each named in full, without birth names, because a birth
// name only reads as "née" beside a surname taken in marriage. A single adult
// is their own name. A nickname is never part of it; it belongs on the row.
//
// An adult with an empty Given is skipped when building the shared-surname
// given-name list, so a cleared Given never leaves a leading or dangling " & "
// or a bare "(Mitchell)"; a birth name only attaches to a non-empty given
// name. If every adult's Given is empty, the Household Name is just the
// surname. The differing-surname branch needs no such guard: joinWords
// already drops the empty Given and prints just the surname.
//
// BuildTree rejects a Household without adults, so none reaches here.
func householdName(h rolo.Household) string {
	surname := h.Adults[0].Surname

	shared := true
	for _, a := range h.Adults[1:] {
		if a.Surname != surname {
			shared = false
		}
	}

	if !shared {
		names := make([]string, 0, len(h.Adults))
		for _, a := range h.Adults {
			names = append(names, joinWords(a.Given, a.Surname))
		}

		return strings.Join(names, " & ")
	}

	givens := make([]string, 0, len(h.Adults))
	for _, a := range h.Adults {
		if a.Given == "" {
			continue
		}

		given := a.Given
		if len(h.Adults) > 1 && a.BirthName != "" && a.BirthName != surname {
			given = joinWords(given, "("+a.BirthName+")")
		}
		givens = append(givens, given)
	}

	return joinWords(strings.Join(givens, " & "), surname)
}

// carriedSurnames is the set of surnames a Household Name prints, which a row
// beneath it need not repeat. Birth names are not included: a parenthesised
// "(Mitchell)" is not a surname the Household carries.
func carriedSurnames(adults []rolo.Person) map[string]bool {
	carried := make(map[string]bool, len(adults))
	for _, a := range adults {
		if a.Surname != "" {
			carried[a.Surname] = true
		}
	}

	return carried
}

// nickname renders p's nickname as every printed name shows it, double-quoted,
// or "" when p has none.
func nickname(p rolo.Person) string {
	if p.Aka == "" {
		return ""
	}

	return `"` + p.Aka + `"`
}

// rowName renders p as their row prints them: given name, any nickname
// double-quoted, and the surname only when the Household Name does not
// already carry it. When Given and Aka are both empty, the surname is kept
// even if it is carried, so a person with a cleared Given never prints as a
// blank row.
func rowName(p rolo.Person, carried map[string]bool) string {
	aka := nickname(p)

	surname := p.Surname
	if carried[surname] && (p.Given != "" || aka != "") {
		surname = ""
	}

	return joinWords(p.Given, aka, surname)
}

// joinWords joins the non-empty words with single spaces.
func joinWords(words ...string) string {
	kept := make([]string, 0, len(words))
	for _, w := range words {
		if w != "" {
			kept = append(kept, w)
		}
	}

	return strings.Join(kept, " ")
}
