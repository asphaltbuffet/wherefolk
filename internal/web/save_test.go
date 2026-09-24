package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/web"
)

// post submits a form to the save route and returns the recorder.
func post(t *testing.T, srv *web.Server, id string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/h/"+id, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	return rec
}

func TestHandleSave(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		form  url.Values
		saver *recordingSaver
		check func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver)
	}{
		{
			name: "a good submit saves and redirects",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given":   {"Clyde"},
				"person.p_clyd01.surname": {"Whitlock"},
				"person.p_clyd01.phone":   {"555.201.0001"},
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusSeeOther, rec.Code)
				assert.Equal(t, "/h/h_clyde", rec.Header().Get("Location"))

				require.Equal(t, 1, saver.calls)
				require.NotNil(t, saver.saved)

				h := findHousehold(t, saver.saved, "h_clyde")
				assert.Equal(t, "555-201-0001", h.Adults[0].Phone,
					"§4.4: normalise on save, then display the normalised value")
			},
		},
		{
			name: "the tree's open set survives the redirect",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"open":                  {"h_aden,h_clyde"},
				"pane":                  {"closed"},
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, _ *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusSeeOther, rec.Code)

				location, err := url.Parse(rec.Header().Get("Location"))
				require.NoError(t, err)

				assert.Equal(t, "h_aden,h_clyde", location.Query().Get("open"))
				assert.Equal(t, "closed", location.Query().Get("pane"))
			},
		},
		{
			name: "an unreadable date refuses the submit and saves nothing",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.p_clyd01.birth": {"June-ish 1998"},
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
				assert.Zero(t, saver.calls, "nothing is written when the submit is refused")

				body := rec.Body.String()
				assert.Contains(t, body, "June-ish 1998",
					"the Editor's typing is preserved in the re-rendered form")
				assert.Contains(t, body, "is not a date this can read")
			},
		},
		{
			name: "a failed save leaves the served document untouched",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.p_clyd01.phone": {"555-9999"},
			},
			saver: &recordingSaver{err: assert.AnError},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusInternalServerError, rec.Code)
				assert.Equal(t, 1, saver.calls)
			},
		},
		{
			name: "a promotion announces itself in the redirect",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given":   {"Clyde"},
				"person.p_dori01.given":   {"Doris"},
				"person.p_carl01.given":   {"Carl"},
				"person.p_carl01.promote": {"on"},
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, _ *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusSeeOther, rec.Code)

				location, err := url.Parse(rec.Header().Get("Location"))
				require.NoError(t, err)

				assert.Equal(t, "h_new001", location.Query().Get("moved"),
					"ADR-0008: the announcement rides in the URL")
			},
		},
		{
			name: "a save to an unknown household is a 404",
			id:   "h_missing",
			form: url.Values{"person.p_x.given": {"X"}},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusNotFound, rec.Code)
				assert.Zero(t, saver.calls)
			},
		},
		{
			name: "a submission that would break the document is refused",
			id:   "h_reeve",
			form: url.Values{
				"person.p_reev01.given":  {"Ray"},
				"person.p_reev01.remove": {"on"},
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
				assert.Zero(t, saver.calls)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saver := tt.saver
			if saver == nil {
				saver = &recordingSaver{}
			}

			srv := newTestServer(t, sampleDocument(), saver)

			rec := post(t, srv, tt.id, tt.form)
			tt.check(t, rec, saver)
		})
	}
}

func TestSaveRebuildsTheTree(t *testing.T) {
	tests := []struct {
		name  string
		form  url.Values
		check func(t *testing.T, srv *web.Server)
	}{
		{
			name: "a renamed adult changes the tree label",
			form: url.Values{
				"person.p_clyd01.given":   {"Clive"},
				"person.p_clyd01.surname": {"Whitlock"},
				"person.p_dori01.given":   {"Doris"},
				"person.p_dori01.surname": {"Whitlock"},
			},
			check: func(t *testing.T, srv *web.Server) {
				t.Helper()

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/h/h_clyde", nil)
				srv.Handler().ServeHTTP(rec, req)

				assert.Contains(t, rec.Body.String(), "Clive/Doris",
					"the tree renders from the rebuilt tree, not the old one")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, sampleDocument(), nil)

			rec := post(t, srv, "h_clyde", tt.form)
			require.Equal(t, http.StatusSeeOther, rec.Code)

			tt.check(t, srv)
		})
	}
}

func TestFormRendersEveryEditableField(t *testing.T) {
	// Every field parseSubmission reads must appear in the rendered form. A
	// field the template stops rendering would not come back in the next
	// submission, and url.Values.Get cannot tell that from a field the Editor
	// cleared — so it would erase itself on the following save.
	tests := []struct {
		name  string
		id    string
		names []string
	}{
		{
			name: "an adult's every field is a named input",
			id:   "h_clyde",
			names: []string{
				"person.p_clyd01.given",
				"person.p_clyd01.surname",
				"person.p_clyd01.birth_name",
				"person.p_clyd01.aka",
				"person.p_clyd01.birth",
				"person.p_clyd01.death",
				"person.p_clyd01.phone",
				"person.p_clyd01.email",
				"person.p_clyd01.hidden.birth",
				"person.p_clyd01.hidden.phone",
				"person.p_clyd01.hidden.email",
			},
		},
		{
			name: "the household's own fields are named inputs",
			id:   "h_clyde",
			names: []string{
				"address.lines",
				"address.hidden",
				"anniversary",
				"open",
				"pane",
			},
		},
		{
			name: "a dependent carries the structural controls too",
			id:   "h_clyde",
			names: []string{
				"person.p_carl01.given",
				"person.p_carl01.promote",
				"person.p_carl01.remove",
			},
		},
		{
			// An existing adult offers no promotion: only a Dependent can be
			// promoted, and applyToGroup ignores the flag on an adult.
			name: "an adult can be removed but not promoted",
			id:   "h_clyde",
			names: []string{
				"person.p_clyd01.remove",
			},
		},
		{
			// A Household with room offers both blank slots, each under the
			// name that tells applySubmission which group the person joins.
			name: "a household with room offers both blank slots",
			id:   "h_reeve",
			names: []string{
				"person.new1.given",
				"person.new1.surname",
				"person.new1.birth",
				"person.new2.given",
				"person.new2.surname",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), "/h/"+tt.id)
			require.Equal(t, http.StatusOK, rec.Code)

			body := rec.Body.String()

			for _, name := range tt.names {
				assert.Contains(t, body, `name="`+name+`"`,
					"%s must be rendered as an input, or it will clear itself on the next save", name)
			}
		})
	}
}

func TestFullHouseholdStillOffersADependentSlot(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		present []string
		absent  []string
	}{
		{
			// §3 caps a Household at two adults, so the adult slot goes away —
			// but the Dependent slot must not, or a full Household could never
			// record a child.
			name:    "the adult slot closes and the dependent slot stays",
			id:      "h_clyde",
			present: []string{"person.new2.given"},
			absent:  []string{"person.new1.given"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), "/h/"+tt.id)
			require.Equal(t, http.StatusOK, rec.Code)

			body := rec.Body.String()

			for _, name := range tt.present {
				assert.Contains(t, body, `name="`+name+`"`)
			}

			for _, name := range tt.absent {
				assert.NotContains(t, body, `name="`+name+`"`,
					"a full household must not offer a third adult")
			}
		})
	}
}
