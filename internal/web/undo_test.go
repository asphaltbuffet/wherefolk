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

	"github.com/asphaltbuffet/wherefolk/internal/web"
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
