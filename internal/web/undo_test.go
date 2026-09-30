package web_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// fetch GETs target from srv itself, so a test can follow a redirect on the
// same server whose memory holds the undo point.
func fetch(t *testing.T, srv *web.Server, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

	return rec
}

// postTo submits a form to any route.
func postTo(t *testing.T, srv *web.Server, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	return rec
}

// location parses a redirect's target.
func location(t *testing.T, rec *httptest.ResponseRecorder) *url.URL {
	t.Helper()

	require.Equal(t, http.StatusSeeOther, rec.Code)

	loc, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)

	return loc
}

// saveClydePhone edits Clyde's phone and returns where the save redirected.
func saveClydePhone(t *testing.T, srv *web.Server, phone string) *url.URL {
	t.Helper()

	return location(t, post(t, srv, "h_clyde", url.Values{
		"person.p_clyd01.given":   {"Clyde"},
		"person.p_clyd01.surname": {"Whitlock"},
		"person.p_clyd01.phone":   {phone},
	}))
}

// undo posts the Undo form a page at loc would carry.
func undo(t *testing.T, srv *web.Server, loc *url.URL, at string) *httptest.ResponseRecorder {
	t.Helper()

	return postTo(t, srv, "/undo", url.Values{"token": {loc.Query().Get("undo")}, "at": {at}})
}

func TestUndo(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, srv *web.Server, saver *recordingSaver)
	}{
		{
			name: "a plain edit is announced and offers Undo beside the sentence",
			check: func(t *testing.T, srv *web.Server, _ *recordingSaver) {
				t.Helper()
				loc := saveClydePhone(t, srv, "555-201-0001")

				assert.Contains(t, loc.Query().Get("said"), "were saved")
				require.NotEmpty(t, loc.Query().Get("undo"))

				body := fetch(t, srv, loc.String()).Body.String()
				assert.Contains(t, body, `action="/undo"`)
				assert.Contains(t, body, loc.Query().Get("undo"))
			},
		},
		{
			name: "Undo restores the Household exactly and says so",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				loc := saveClydePhone(t, srv, "555-201-0001")

				back := location(t, undo(t, srv, loc, "h_clyde"))
				assert.Equal(t, "/h/h_clyde", back.Path)
				assert.Equal(t, "Your last change was undone.", back.Query().Get("said"))
				assert.Empty(t, back.Query().Get("undo"), "one step, no redo")

				require.Equal(t, 2, saver.calls, "the undo is itself saved")
				clyde := findHousehold(t, saver.saved, "h_clyde")
				assert.Equal(t, "555-0142", clyde.Adults[0].Phone)
				assert.Equal(t, "clyde@example.com", clyde.Adults[0].Email,
					"a field the edit cleared comes back too")
			},
		},
		{
			name: "the same Undo twice does nothing more",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				loc := saveClydePhone(t, srv, "555-201-0001")
				_ = undo(t, srv, loc, "h_clyde")

				again := location(t, undo(t, srv, loc, "h_clyde"))
				assert.Contains(t, again.Query().Get("said"), "can no longer be undone")
				assert.Equal(t, 2, saver.calls)
			},
		},
		{
			name: "a newer save retires the older Undo, on the page and on the server",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				first := saveClydePhone(t, srv, "555-201-0001")
				_ = saveClydePhone(t, srv, "555-201-0002")

				body := fetch(t, srv, first.String()).Body.String()
				assert.NotContains(t, body, `action="/undo"`,
					"an Undo that would be refused must not be offered")

				stale := location(t, undo(t, srv, first, "h_clyde"))
				assert.Contains(t, stale.Query().Get("said"), "can no longer be undone")
				assert.Equal(t, 2, saver.calls)
			},
		},
		{
			name: "a token the server never issued is refused",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				rec := postTo(t, srv, "/undo", url.Values{"token": {"from-before-a-restart"}, "at": {"h_clyde"}})

				assert.Contains(t, location(t, rec).Query().Get("said"), "can no longer be undone")
				assert.Zero(t, saver.calls)
			},
		},
		{
			name: "an Undo whose save fails leaves the Directory as the disk holds it",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				loc := saveClydePhone(t, srv, "555-201-0001")
				saver.err = errors.New("disk full")

				rec := undo(t, srv, loc, "h_clyde")
				assert.Equal(t, http.StatusInternalServerError, rec.Code)

				body := fetch(t, srv, "/h/h_clyde").Body.String()
				assert.Contains(t, body, "555-201-0001")
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

// TestSavesKeepTheSettings proves every write path hands persist the settings
// it is serving. A State literal that forgot them would swap a nil in, and the
// next title change would start from nothing.
func TestSavesKeepTheSettings(t *testing.T) {
	tests := []struct {
		name string
		act  func(t *testing.T, srv *web.Server) *url.URL
	}{
		{
			name: "a household edit",
			act: func(t *testing.T, srv *web.Server) *url.URL {
				t.Helper()
				return saveClydePhone(t, srv, "555-000-1111")
			},
		},
		{
			name: "an undo of a household edit",
			act: func(t *testing.T, srv *web.Server) *url.URL {
				t.Helper()
				loc := saveClydePhone(t, srv, "555-000-1111")
				return location(t, undo(t, srv, loc, "h_clyde"))
			},
		},
		{
			name: "a household deletion",
			act: func(t *testing.T, srv *web.Server) *url.URL {
				t.Helper()
				return location(t, postTo(t, srv, "/h/h_dave/delete", nil))
			},
		},
		{
			name: "a restore from Recently deleted",
			act: func(t *testing.T, srv *web.Server) *url.URL {
				t.Helper()
				_ = location(t, postTo(t, srv, "/h/h_dave/delete", nil))
				return location(t, postTo(t, srv, "/trash/h_dave/restore", nil))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := store.NewSettings().WithTitle("The Whitlock Directory")
			saver := &recordingSaver{}

			srv, err := web.New(store.State{Document: sampleDocument(), Settings: settings},
				config.Config{}, testLogger(), web.Meta{}, saver.save,
				func() (rolo.HouseholdID, error) { return "h_x", nil },
				func() (rolo.PersonID, error) { return "p_x", nil },
				&fakeExporter{}, testClock,
			)
			require.NoError(t, err)

			tt.act(t, srv)

			assert.Same(t, settings, saver.savedSettings, "the served settings ride along unchanged")
		})
	}
}
