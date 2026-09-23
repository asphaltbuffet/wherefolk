package web_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
			absent: []rolo.HouseholdID{"h_aden", "h_clyde"},
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
			name:   "whitespace and empty entries are tolerated",
			raw:    " h_aden , ,",
			want:   []rolo.HouseholdID{"h_aden"},
			absent: nil,
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

