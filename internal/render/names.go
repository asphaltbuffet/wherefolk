package render

import (
	"strings"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

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
