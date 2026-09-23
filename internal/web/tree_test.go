package web_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestTreeFragment(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		checkFunc  func(t *testing.T, body string)
	}{
		{
			name:       "renders every root",
			target:     "/tree",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Aden/Nettie")
				assert.Contains(t, body, "Ray")
				assert.NotContains(t, body, "Clyde/Doris", "a collapsed root hides its children")
			},
		},
		{
			name:       "an opened root reveals its children",
			target:     "/tree?open=h_aden",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Clyde/Doris")
			},
		},
		{
			name:       "a selection opens its ancestors",
			target:     "/tree?selected=h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Clyde/Doris", "the selection's ancestors are expanded to reveal it")
			},
		},
		{
			name:       "an unknown open id is ignored, not an error",
			target:     "/tree?open=h_nonexistent",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Aden/Nettie", "a stale bookmark still renders the tree")
			},
		},
		{
			name:       "the fragment carries no page chrome",
			target:     "/tree",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "<!DOCTYPE html>")
			},
		},
		{
			name:       "every node is addressable for htmx",
			target:     "/tree",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, `data-household="h_aden"`)
			},
		},
		{
			name:       "closing an open node collapses it",
			target:     "/tree?open=h_aden&close=h_aden",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "Clyde/Doris")
				assert.Contains(t, body, "Aden/Nettie", "the node itself stays; only its children go")
			},
		},
		{
			name:       "an ancestor of the selection cannot be collapsed out of view",
			target:     "/tree?selected=h_clyde&close=h_aden",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Clyde/Doris",
					"collapsing the selection's parent would hide the Household the detail pane shows")
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

func TestOpenSet(t *testing.T) {
	tests := []struct {
		name     string
		selected rolo.HouseholdID
		raw      string
		want     []rolo.HouseholdID
		absent   []rolo.HouseholdID
	}{
		{
			name:   "empty everything opens nothing",
			want:   nil,
			absent: []rolo.HouseholdID{"h_aden", "h_clyde", "h_reeve", ""},
		},
		{
			name:   "explicit ids are honoured",
			raw:    "h_aden",
			want:   []rolo.HouseholdID{"h_aden"},
			absent: []rolo.HouseholdID{"h_reeve"},
		},
		{
			name:     "a selection opens its whole ancestor chain and itself",
			selected: "h_clyde",
			want:     []rolo.HouseholdID{"h_aden", "h_clyde"},
			absent:   []rolo.HouseholdID{"h_reeve"},
		},
		{
			name:   "unknown ids are dropped",
			raw:    "h_ghost,h_aden",
			want:   []rolo.HouseholdID{"h_aden"},
			absent: []rolo.HouseholdID{"h_ghost"},
		},
		{
			name: "whitespace and empty entries are tolerated",
			raw:  " h_aden , ,",
			want: []rolo.HouseholdID{"h_aden"},
			// The empty string must not become a phantom key: a parseIDs
			// that emitted "" would pass a want-only assertion.
			absent: []rolo.HouseholdID{""},
		},
		{
			name:     "an unknown selection does not discard the explicit set",
			selected: "h_ghost",
			raw:      "h_aden",
			want:     []rolo.HouseholdID{"h_aden"},
			absent:   []rolo.HouseholdID{"h_ghost"},
		},
		{
			name:     "a selection's chain is open even when the explicit set is empty",
			selected: "h_clyde",
			raw:      "",
			want:     []rolo.HouseholdID{"h_aden", "h_clyde"},
		},
	}

	srv := newTestServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := srv.OpenSetForTest(tt.selected, tt.raw)

			for _, id := range tt.want {
				assert.True(t, got[id], "%s should be open", id)
			}
			for _, id := range tt.absent {
				assert.False(t, got[id], "%s should not be open", id)
			}
		})
	}
}

// TestToggleURLEncodesIDs guards the query construction against Household IDs
// that are not URL-safe. store.Load accepts a hand-repaired document exactly as
// written and validation in this project observes rather than rejects, so an ID
// carrying "&" or a space is a supported input, not a hypothetical one.
//
// The danger is specific to hx-get: html/template percent-encodes into href
// because it recognises it as a URL attribute, but hx-get is an attribute it
// knows nothing about and receives HTML escaping only. Building the query with
// url.Values is what closes that gap.
func TestToggleURLEncodesIDs(t *testing.T) {
	tests := []struct {
		name     string
		id       rolo.HouseholdID
		open     string
		isOpen   bool
		contains string
		absent   string
	}{
		{
			name:     "an ampersand cannot terminate the parameter",
			id:       "h_amp&c=x",
			open:     "h_a",
			contains: "h_amp%26c%3Dx",
			absent:   "h_amp&c=x",
		},
		{
			name:     "a space is encoded",
			id:       "h_ b",
			contains: "h_+b",
			absent:   "h_ b",
		},
		{
			name:     "a quote cannot escape the attribute",
			id:       `h_"q`,
			contains: "h_%22q",
			absent:   `h_"q`,
		},
		{
			name:     "collapsing sends the id as close, not appended to open",
			id:       "h_aden",
			open:     "h_aden,h_clyde",
			isOpen:   true,
			contains: "close=h_aden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := web.ToggleURLForTest("", tt.open, tt.id, tt.isOpen)

			assert.Contains(t, got, tt.contains)
			if tt.absent != "" {
				assert.NotContains(t, got, tt.absent, "the raw id must never appear unencoded")
			}
		})
	}
}

// TestSelectionAncestorsHaveNoToggle guards against advertising a control that
// cannot do anything. treeView re-opens the selection's chain after any close
// request, so a toggle on one of those nodes would render output identical to
// not clicking it — an Editor would click "Collapse" and see nothing happen.
func TestSelectionAncestorsHaveNoToggle(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		checkFunc func(t *testing.T, body string)
	}{
		{
			name:   "an ancestor of the selection offers no collapse link",
			target: "/tree?selected=h_clyde",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "Collapse Aden/Nettie",
					"h_aden is pinned open, so it must not advertise a Collapse control")
				assert.Contains(t, body, "Clyde/Doris", "the selection is still visible")
			},
		},
		{
			// h_reeve is the only other root in the fixture and is childless,
			// so the tree offers no toggle at all here. Asserting that keeps
			// the row honest: pinning removes h_aden's control, and nothing
			// else in this fixture has one to lose.
			name:   "pinning the selection's chain leaves no toggle in this fixture",
			target: "/tree?selected=h_clyde",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "aria-expanded",
					"h_aden is pinned and h_reeve is childless, so no node offers a toggle")
				assert.Contains(t, body, "Ray", "the childless root is still listed")
			},
		},
		{
			name:   "a toggleable node elsewhere is unaffected by pinning",
			target: "/tree?selected=h_reeve",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Expand Aden/Nettie",
					"h_aden is not in h_reeve's chain, so it keeps its control")
			},
		},
		{
			name:   "with nothing selected every parent is toggleable",
			target: "/tree?open=h_aden",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Collapse Aden/Nettie")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, http.StatusOK, rec.Code)
			tt.checkFunc(t, rec.Body.String())
		})
	}
}

// TestToggleLinksPushHistory guards the claim, made in treeView's comment and
// in ADR-0008, that the back button retraces expansions. An htmx swap does not
// touch the address bar on its own, so without hx-push-url the toggles would
// change the tree while the URL stood still and the back button would retrace
// nothing.
func TestToggleLinksPushHistory(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "an expand link pushes history", target: "/tree"},
		{name: "a collapse link pushes history", target: "/tree?open=h_aden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, http.StatusOK, rec.Code)

			body := rec.Body.String()
			require.Contains(t, body, "hx-get=", "this fixture state must offer a toggle")
			assert.Contains(t, body, `hx-push-url="true"`,
				"a toggle that does not push history breaks ADR-0008's back-button claim")
		})
	}
}

// TestOpenSetDropsChildlessNodes guards against unbounded URL growth.
// selectionChain always adds the selection itself, and the selection is often a
// leaf; a leaf renders no close link, so once its ID entered the open set
// nothing could ever remove it and every subsequent link carried it forward.
func TestOpenSetDropsChildlessNodes(t *testing.T) {
	tests := []struct {
		name     string
		selected rolo.HouseholdID
		raw      string
		want     string
	}{
		{
			name:     "selecting a leaf does not put it in the open list",
			selected: "h_dave",
			want:     "h_aden,h_clyde",
		},
		{
			name: "a childless root passed explicitly is dropped",
			raw:  "h_reeve",
			want: "",
		},
		{
			name: "a parent is kept",
			raw:  "h_aden",
			want: "h_aden",
		},
		{
			name:     "selecting a parent keeps its whole chain",
			selected: "h_clyde",
			want:     "h_aden,h_clyde",
		},
	}

	srv := newTestServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, srv.JoinIDsOrderedForTest(srv.OpenSetForTest(tt.selected, tt.raw)))
		})
	}
}
