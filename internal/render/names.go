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

// rowName renders p as their row prints them: given name, any nickname
// double-quoted, and the surname only when the Household Name does not
// already carry it.
func rowName(p rolo.Person, carried map[string]bool) string {
	aka := ""
	if p.Aka != "" {
		aka = `"` + p.Aka + `"`
	}

	surname := p.Surname
	if carried[surname] {
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
