package web_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func hasHousehold(doc *store.Document, id rolo.HouseholdID) bool {
	for _, h := range doc.Households {
		if h.ID == id {
			return true
		}
	}
	return false
}

func TestDeletePages(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		want       []string
		notWant    []string
	}{
		{
			name:       "a leaf Household is offered for deletion with what it means",
			target:     "/h/h_dave/delete",
			wantStatus: http.StatusOK,
			want: []string{
				"Delete Dave Whitlock?",
				`action="/h/h_dave/delete"`,
				"Recently deleted until October 25",
			},
		},
		{
			name:       "a Household with Households beneath it explains instead of offering",
			target:     "/h/h_clyde/delete",
			wantStatus: http.StatusOK,
			want:       []string{"first delete the households beneath it: Dave Whitlock"},
			notWant:    []string{`action="/h/h_clyde/delete"`},
		},
		{
			name:       "a Memorial Household is never offered",
			target:     "/h/h_aden/delete",
			wantStatus: http.StatusOK,
			want:       []string{"memorial household is kept permanently"},
			notWant:    []string{`action="/h/h_aden/delete"`},
		},
		{
			name:       "an unknown Household is a sentence, not a code",
			target:     "/h/h_nope/delete",
			wantStatus: http.StatusNotFound,
			want:       []string{"That household is not in the directory"},
		},
		{
			name:       "the detail pane links a deletable Household to its confirmation",
			target:     "/h/h_dave",
			wantStatus: http.StatusOK,
			want:       []string{`href="/h/h_dave/delete`, "Delete this household…"},
		},
		{
			name:       "the detail pane explains why a Household cannot be deleted",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			want:       []string{"first delete the households beneath it"},
			notWant:    []string{"/h/h_clyde/delete"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := fetch(t, newTestServer(t, sampleDocument(), nil), tt.target)

			assert.Equal(t, tt.wantStatus, rec.Code)

			body := rec.Body.String()
			for _, w := range tt.want {
				assert.Contains(t, body, w)
			}
			for _, nw := range tt.notWant {
				assert.NotContains(t, body, nw)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, srv *web.Server, saver *recordingSaver)
	}{
		{
			name: "deleting moves the Household to the Trash and lands on its parent",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				loc := location(t, postTo(t, srv, "/h/h_dave/delete", url.Values{"open": {"h_aden,h_clyde"}}))

				assert.Equal(t, "/h/h_clyde", loc.Path)
				assert.Equal(t, "h_aden,h_clyde", loc.Query().Get("open"))
				assert.Contains(t, loc.Query().Get("said"), "Dave Whitlock was deleted")
				assert.Contains(t, loc.Query().Get("said"), "October 25")
				assert.NotEmpty(t, loc.Query().Get("undo"))

				assert.False(t, hasHousehold(saver.saved, "h_dave"))
				require.Len(t, saver.savedTrash.Entries, 1)
				e := saver.savedTrash.Entries[0]
				assert.Equal(t, rolo.HouseholdID("h_dave"), e.Household.ID)
				assert.Equal(t, "Aden/Nettie › Clyde/Doris › Dave", e.Path)
				assert.True(t, e.DeletedAt.Equal(time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)))
			},
		},
		{
			name: "deleting a root lands on the directory",
			check: func(t *testing.T, srv *web.Server, _ *recordingSaver) {
				t.Helper()
				loc := location(t, postTo(t, srv, "/h/h_reeve/delete", nil))
				assert.Equal(t, "/", loc.Path)
			},
		},
		{
			name: "a blocked deletion is refused and saves nothing",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				rec := postTo(t, srv, "/h/h_clyde/delete", nil)

				assert.Equal(t, http.StatusConflict, rec.Code)
				assert.Contains(t, rec.Body.String(), "first delete the households beneath it")
				assert.Zero(t, saver.calls)
			},
		},
		{
			name: "deleting an unknown Household is a 404",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				rec := postTo(t, srv, "/h/h_nope/delete", nil)
				assert.Equal(t, http.StatusNotFound, rec.Code)
				assert.Zero(t, saver.calls)
			},
		},
		{
			name: "a deleted Household is gone from the tree",
			check: func(t *testing.T, srv *web.Server, _ *recordingSaver) {
				t.Helper()
				_ = location(t, postTo(t, srv, "/h/h_dave/delete", nil))

				assert.Equal(t, http.StatusNotFound, fetch(t, srv, "/h/h_dave").Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saver := &recordingSaver{}
			srv := newTestServer(t, sampleDocument(), saver)

			tt.check(t, srv, saver)
		})
	}
}
