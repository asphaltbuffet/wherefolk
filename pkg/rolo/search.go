package rolo

import "strings"

// Match is one Person found by a search, carrying the Household they belong to
// and that Household's Path.
//
// §4.3 makes search a shortcut *through* the tree rather than an alternative to
// it, so a Match always names a destination in the tree. The Path is what makes
// five people called Dave distinguishable in a result list, so it is part of the
// Match rather than something the caller is trusted to look up.
type Match struct {
	Person    Person
	Household HouseholdID
	Path      string
}

// SearchPeople returns every Person whose name contains query, case-insensitively.
//
// At a few hundred people a linear scan is instant, so there is no index to keep
// in sync with the document — which matters because item 5 makes the document
// mutable and a stale index would silently return people who have been renamed.
//
// Results are ordered by the depth-first tree walk, so people group by Branch.
// That is what makes the Path column readable: two Daves under different
// grandparents appear apart, with their differing Paths adjacent to their
// identical names.
func (t *Tree) SearchPeople(query string) []Match {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil
	}

	var matches []Match

	// Walk's fn returns an error to stop early; this search never stops early,
	// so the error is always nil and the outer error is always nil with it.
	_ = t.Walk(func(h Household, _ int) error {
		path, err := t.PathString(h.ID)
		if err != nil {
			// Unreachable: Walk only visits Households that are in the tree,
			// which is exactly the condition PathString fails on. Fall back to
			// the bare label rather than dropping the person from the results.
			path = h.Label()
		}

		for _, group := range [][]Person{h.Adults, h.Dependents} {
			for _, p := range group {
				if !personMatches(p, needle) {
					continue
				}

				matches = append(matches, Match{Person: p, Household: h.ID, Path: path})
			}
		}

		return nil
	})

	return matches
}

// personMatches reports whether needle appears in any name this Person is known
// by. needle is already lowercased and trimmed.
//
// Birth name and nickname are included because the Editor searches for the name
// they remember, which for a relative who married in 1968 is frequently not the
// one on the envelope.
func personMatches(p Person, needle string) bool {
	for _, field := range []string{p.Given, p.Surname, p.BirthName, p.Aka} {
		if field == "" {
			continue
		}

		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}

	return false
}
