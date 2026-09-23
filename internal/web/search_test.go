package web_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestSearchFragment(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		checkFunc  func(t *testing.T, body string)
	}{
		{
			name:       "an empty query returns an empty list, not every person",
			target:     "/search?q=",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				// Counting list items rather than naming absent people: a bug
				// returning one unrelated match would slip past a pair of
				// NotContains, but cannot slip past a count of zero.
				assert.Equal(t, 0, strings.Count(body, "<li>"),
					"nothing typed yet means no results at all")
				assert.NotContains(t, body, "No one found",
					"an empty box is not the same as a search that found nobody")
			},
		},
		{
			name:       "a match shows the person and their Household's Path",
			target:     "/search?q=dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Dave Whitlock")
				assert.Contains(t, body, "Aden/Nettie › Clyde/Doris › Dave",
					"§4.3: results carry their Path as context")
			},
		},
		{
			name:       "a result links into the tree, not to a detached editor",
			target:     "/search?q=dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, `href="/h/h_dave"`)
			},
		},
		{
			name:       "a Dependent is findable and links to the Household listing them",
			target:     "/search?q=carl",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Carl Whitlock")
				assert.Contains(t, body, `href="/h/h_clyde"`)
			},
		},
		{
			name:       "no match says so in plain English",
			target:     "/search?q=zebedee",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "No one found")
			},
		},
		{
			name:       "the fragment carries no page chrome",
			target:     "/search?q=dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "<!DOCTYPE html>")
			},
		},
		{
			name:       "a nickname finds the person",
			target:     "/search?q=dot",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				// html/template escapes the quotes around a nickname in text
				// content (Doris "Dot" Whitlock -> Doris &#34;Dot&#34;
				// Whitlock) — that is the correct, safe rendering, and a
				// browser displays it as literal quotes. Assert on the
				// escaped form actually emitted rather than the raw string,
				// which never appears in rendered HTML.
				assert.Contains(t, body, `Doris &#34;Dot&#34; Whitlock`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, tt.wantStatus, rec.Code)
			tt.checkFunc(t, rec.Body.String())
		})
	}
}

// crowdedDocument holds more people than maxResults, all matching one query, so
// the truncation path is exercised rather than merely defined.
func crowdedDocument(t *testing.T, people int) *store.Document {
	t.Helper()

	doc := &store.Document{Schema: store.CurrentSchema}

	for i := range people {
		id := rolo.HouseholdID(fmt.Sprintf("h_%03d", i))
		doc.Households = append(doc.Households, rolo.Household{
			ID: id,
			Adults: []rolo.Person{{
				ID:      rolo.PersonID(fmt.Sprintf("p_%03d", i)),
				Given:   fmt.Sprintf("Person%03d", i),
				Surname: "Crowded",
				Birth:   rolo.Date{Year: 1900 + i},
			}},
		})
	}

	return doc
}

func TestSearchTruncatesLongResultLists(t *testing.T) {
	tests := []struct {
		name          string
		people        int
		query         string
		wantTruncated bool
	}{
		{
			name:          "a result list within the cap is not truncated",
			people:        5,
			query:         "crowded",
			wantTruncated: false,
		},
		{
			name:          "a result list over the cap is truncated and says so",
			people:        60,
			query:         "crowded",
			wantTruncated: true,
		},
		{
			name:          "a narrow query over a crowded document is not truncated",
			people:        60,
			query:         "Person007",
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, crowdedDocument(t, tt.people), "/search?q="+tt.query)

			require.Equal(t, http.StatusOK, rec.Code)

			if tt.wantTruncated {
				assert.Contains(t, rec.Body.String(), "Showing the first",
					"an Editor must know to narrow the query rather than assume they saw everything")
				return
			}

			assert.NotContains(t, rec.Body.String(), "Showing the first")
		})
	}
}

// TestSearchResultExpandsTreeToRevealHousehold guards §4.3's second clause,
// which the brief for this task never tests: a search result is a link to
// /h/{id}, and following it must expand the tree to reveal that Household —
// not just select it — so choosing a result "relocates the Editor within
// their mental model" rather than dropping them into a detached view.
//
// This exercises the *navigation target* of a search result (GET /h/{id}),
// not the search handler itself: /h/h_dave is exactly the link a search hit
// for "dave" renders (see TestSearchFragment above), and Task 4's
// selectionChain is what must open h_dave's ancestors for this to render
// correctly. A regression here would silently defeat search's whole reason
// for linking into the tree instead of opening a detached editor.
func TestSearchResultExpandsTreeToRevealHousehold(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		wantHouseholds []string
	}{
		{
			name:   "selecting a deep Household reveals its ancestors in the tree",
			target: "/h/h_dave",
			// h_dave's chain is h_aden -> h_clyde -> h_dave. All three must be
			// present as tree nodes, not just h_dave in the detail pane.
			wantHouseholds: []string{"h_aden", "h_clyde", "h_dave"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, http.StatusOK, rec.Code)

			body := rec.Body.String()
			for _, id := range tt.wantHouseholds {
				assert.Contains(t, body, `data-household="`+id+`"`,
					"the tree pane must render this ancestor, not just the selected leaf")
			}
		})
	}
}
