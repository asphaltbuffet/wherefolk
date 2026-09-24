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
				assert.Contains(t, location.Query().Get("said"), "household of their own",
					"a promotion carries its message alongside the destination id")
			},
		},
		{
			name: "removing a dependent announces itself in the redirect",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given":  {"Clyde"},
				"person.p_carl01.given":  {"Carl"},
				"person.p_carl01.remove": {"on"},
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, _ *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusSeeOther, rec.Code)

				location, err := url.Parse(rec.Header().Get("Location"))
				require.NoError(t, err)

				assert.Contains(t, location.Query().Get("said"), "removed from the directory",
					"a removal must announce itself even though it has no household to link to")
				assert.Empty(t, location.Query().Get("moved"),
					"a removal has no destination household")
			},
		},
		{
			name: "adding a person announces itself in the redirect",
			id:   "h_reeve",
			form: url.Values{
				"person.new1.given":   {"Nadia"},
				"person.new1.surname": {"Reeve"},
			},
			check: func(t *testing.T, rec *httptest.ResponseRecorder, _ *recordingSaver) {
				t.Helper()

				assert.Equal(t, http.StatusSeeOther, rec.Code)

				location, err := url.Parse(rec.Header().Get("Location"))
				require.NoError(t, err)

				assert.Contains(t, location.Query().Get("said"), "was added to",
					"an addition must announce itself")
				assert.Empty(t, location.Query().Get("moved"),
					"an addition has no destination household")
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

func TestAnnouncementRendersFromTheQueryString(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "a removal's message renders without a link",
			target: "/h/h_clyde?said=" + url.QueryEscape("Carl was removed from the directory."),
			want:   "Carl was removed from the directory.",
		},
		{
			name: "a promotion's message renders alongside its link",
			target: "/h/h_clyde?said=" +
				url.QueryEscape("Carl now has a household of their own, beneath Clyde/Doris.") +
				"&moved=h_clyde",
			want: "Carl now has a household of their own, beneath Clyde/Doris.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)
			require.Equal(t, http.StatusOK, rec.Code)

			assert.Contains(t, rec.Body.String(), tt.want)
		})
	}
}

func TestAnnouncementLinksToTheMovedHousehold(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantAnchor string
	}{
		{
			// moved=h_clyde stands in for a promotion destination here; the
			// assertion is on the anchor itself, which is what the Link/Label
			// branch of announcementFor renders.
			name: "a promotion's link points at the destination household",
			target: "/h/h_clyde?said=" +
				url.QueryEscape("Carl now has a household of their own, beneath Clyde/Doris.") +
				"&moved=h_clyde",
			wantAnchor: `<a href="/h/h_clyde">Go to Clyde/Doris</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)
			require.Equal(t, http.StatusOK, rec.Code)

			assert.Contains(t, rec.Body.String(), tt.wantAnchor)
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
				"person.p_clyd01.is_adult",
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

func TestRefusedFormRendersEveryEditableField(t *testing.T) {
	// The 422 body is also a form the Editor submits next, so the same
	// invariant TestFormRendersEveryEditableField checks on the GET path
	// binds it with equal force. This is the divergence Important findings 1
	// and 2 introduced: formViewFromSubmission dropped IsAdult and NewSlot,
	// and this test exists to catch a repeat.
	tests := []struct {
		name  string
		id    string
		form  url.Values
		names []string
	}{
		{
			name: "an existing adult's every field is a named input on refusal",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given":    {"Clyde"},
				"person.p_clyd01.birth":    {"June-ish 1998"},
				"person.p_clyd01.is_adult": {"true"},
			},
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
				"person.p_clyd01.is_adult",
				"person.p_clyd01.remove",
			},
		},
		{
			// A refused submit must still offer the blank adult slot, or an
			// Editor recovering from a typo loses the ability to add an
			// adult without another round trip (Important finding 2).
			name: "the blank adult slot survives a refusal",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given":    {"Clyde"},
				"person.p_clyd01.birth":    {"June-ish 1998"},
				"person.p_clyd01.is_adult": {"true"},
			},
			names: []string{
				"person.new1.given",
				"person.new1.surname",
				"person.new2.given",
				"person.new2.surname",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, sampleDocument(), nil)

			rec := post(t, srv, tt.id, tt.form)
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

			body := rec.Body.String()

			for _, name := range tt.names {
				assert.Contains(t, body, `name="`+name+`"`,
					"%s must be rendered as an input in the refused form", name)
			}
		})
	}
}

func TestRefusedFormDoesNotDuplicateANewPerson(t *testing.T) {
	// The refusal path re-renders everyone in sub.People through view.Adults,
	// then unconditionally offered the blank new-person slot alongside them.
	// For new1 that slot was guarded (haveNewAdult); new2 was not, so a
	// refused Dependent addition rendered twice — once from the submission
	// loop, once as the blank NewDependent slot — with both fieldsets
	// carrying the same name="person.new2.*" attributes. assert.Contains
	// would not catch a duplicate; strings.Count does.
	tests := []struct {
		name  string
		id    string
		form  url.Values
		field string
	}{
		{
			name: "a refused new adult is rendered exactly once",
			id:   "h_clyde",
			form: url.Values{
				"person.new1.given": {"Nadia"},
				"person.new1.birth": {"not a date"},
			},
			field: "person.new1.given",
		},
		{
			name: "a refused new dependent is rendered exactly once",
			id:   "h_clyde",
			form: url.Values{
				"person.new2.given": {"Nadia"},
				"person.new2.birth": {"not a date"},
			},
			field: "person.new2.given",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, sampleDocument(), nil)

			rec := post(t, srv, tt.id, tt.form)
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

			body := rec.Body.String()

			assert.Equal(t, 1, strings.Count(body, `name="`+tt.field+`"`),
				"%s must appear exactly once in the refused form", tt.field)
		})
	}
}

func TestRefusedFormDoesNotOfferAnAdultPromoteControl(t *testing.T) {
	// formViewFromSubmission never used to set IsAdult, so every person it
	// built rendered with IsAdult == false, and the template's
	// {{if not .IsAdult}} gate offered "Give them a household of their own"
	// on adults — an operation applyToGroup silently ignores for an adult.
	// Important finding 1.
	tests := []struct {
		name string
		id   string
		form url.Values
	}{
		{
			name: "an adult refused for a bad birth date is not offered a promote control",
			id:   "h_clyde",
			form: url.Values{
				"person.p_clyd01.given":    {"Clyde"},
				"person.p_clyd01.birth":    {"June-ish 1998"},
				"person.p_clyd01.is_adult": {"true"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, sampleDocument(), nil)

			rec := post(t, srv, tt.id, tt.form)
			require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

			assert.NotContains(t, rec.Body.String(), `name="person.p_clyd01.promote"`,
				"an adult must never be offered a promote control, refused or not")
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
