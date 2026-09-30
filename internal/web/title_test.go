package web_test

import (
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

// newTitleServer builds a Server serving settings, recording what it saves.
func newTitleServer(t *testing.T, settings *store.Settings, saver *recordingSaver) *web.Server {
	t.Helper()

	srv, err := web.New(store.State{Document: sampleDocument(), Settings: settings},
		config.Config{}, testLogger(), web.Meta{}, saver.save,
		func() (rolo.HouseholdID, error) { return "h_x", nil },
		func() (rolo.PersonID, error) { return "p_x", nil },
		&fakeExporter{pages: [][]byte{[]byte("<svg/>")}}, testClock,
	)
	require.NoError(t, err)

	return srv
}

func TestSetTitle(t *testing.T) {
	tests := []struct {
		name       string
		stored     string
		form       url.Values
		wantStatus int
		checkFunc  func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver)
	}{
		{
			name:       "saves the trimmed title and announces it on the export page",
			form:       url.Values{"title": {"  The Langford Family Directory  "}, "tier": {"call"}},
			wantStatus: http.StatusSeeOther,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()
				require.NotNil(t, saver.savedSettings)
				assert.Equal(t, "The Langford Family Directory", saver.savedSettings.Title)

				loc := location(t, rec)
				assert.Equal(t, "/export", loc.Path)
				assert.Equal(t, "call", loc.Query().Get("tier"), "the preview the Editor was looking at survives")
				assert.Equal(t, "The Directory is now titled “The Langford Family Directory”.", loc.Query().Get("said"))
				assert.NotEmpty(t, loc.Query().Get("undo"))
			},
		},
		{
			name:       "clearing the title announces the default",
			stored:     "The Langford Family Directory",
			form:       url.Values{"title": {""}},
			wantStatus: http.StatusSeeOther,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()
				require.NotNil(t, saver.savedSettings)
				assert.Empty(t, saver.savedSettings.Title)

				loc := location(t, rec)
				assert.Equal(t, "The title was cleared, so the Directory is titled “Family Directory”.",
					loc.Query().Get("said"))
				assert.Empty(t, loc.Query().Get("tier"), "no tier was chosen, so none is invented")
			},
		},
		{
			name:       "an unchanged title saves nothing and announces nothing",
			stored:     "The Langford Family Directory",
			form:       url.Values{"title": {"The Langford Family Directory"}, "tier": {"mail"}},
			wantStatus: http.StatusSeeOther,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()
				assert.Zero(t, saver.calls)
				assert.Equal(t, "/export?tier=mail", location(t, rec).String())
			},
		},
		{
			name:       "a title over 80 characters is refused and the typing survives",
			form:       url.Values{"title": {strings.Repeat("é", 81)}},
			wantStatus: http.StatusUnprocessableEntity,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()
				assert.Zero(t, saver.calls)
				assert.Contains(t, rec.Body.String(), strings.Repeat("é", 81), "the Editor's typing is re-rendered")
				assert.Contains(t, rec.Body.String(), "A title can be at most 80 characters.")
			},
		},
		{
			name:       "exactly 80 characters is accepted",
			form:       url.Values{"title": {strings.Repeat("é", 80)}},
			wantStatus: http.StatusSeeOther,
			checkFunc: func(t *testing.T, _ *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()
				require.NotNil(t, saver.savedSettings)
				assert.Equal(t, strings.Repeat("é", 80), saver.savedSettings.Title)
			},
		},
		{
			name:       "a title with a line break is refused",
			form:       url.Values{"title": {"The Langford\nFamily Directory"}},
			wantStatus: http.StatusUnprocessableEntity,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, saver *recordingSaver) {
				t.Helper()
				assert.Zero(t, saver.calls)
				assert.Contains(t, rec.Body.String(), "A title must be a single line.")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saver := &recordingSaver{}
			srv := newTitleServer(t, store.NewSettings().WithTitle(tt.stored), saver)

			rec := postTo(t, srv, "/export/title", tt.form)

			require.Equal(t, tt.wantStatus, rec.Code)
			tt.checkFunc(t, rec, saver)
		})
	}
}

func TestExportPageShowsTheTitle(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		want   []string
	}{
		{
			name: "an untitled Directory shows the default as a placeholder",
			want: []string{`name="title" value=""`, `placeholder="Family Directory"`},
		},
		{
			name:   "a titled Directory shows the title",
			stored: "The Langford Family Directory",
			want:   []string{`name="title" value="The Langford Family Directory"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTitleServer(t, store.NewSettings().WithTitle(tt.stored), &recordingSaver{})

			body := fetch(t, srv, "/export").Body.String()

			for _, w := range tt.want {
				assert.Contains(t, body, w)
			}
		})
	}
}

func TestUndoTitle(t *testing.T) {
	tests := []struct {
		name     string
		tier     string
		wantUndo string // where Undo lands
	}{
		{
			name:     "returns to the chosen preview",
			tier:     "call",
			wantUndo: "/export?said=Your+last+change+was+undone.&tier=call",
		},
		{
			name:     "returns to the export page with no preview",
			tier:     "",
			wantUndo: "/export?said=Your+last+change+was+undone.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTitleServer(t, store.NewSettings(), &recordingSaver{})

			form := url.Values{"title": {"The Langford Family Directory"}}
			if tt.tier != "" {
				form.Set("tier", tt.tier)
			}
			loc := location(t, postTo(t, srv, "/export/title", form))

			page := fetch(t, srv, loc.String()).Body.String()
			require.Contains(t, page, `name="back" value="export"`, "the Undo beside the announcement returns here")

			undoForm := url.Values{"token": {loc.Query().Get("undo")}, "back": {"export"}}
			if tt.tier != "" {
				undoForm.Set("tier", tt.tier)
			}
			back := location(t, postTo(t, srv, "/undo", undoForm))
			assert.Equal(t, tt.wantUndo, back.String())

			after := fetch(t, srv, "/export").Body.String()
			assert.Contains(t, after, `name="title" value=""`, "the title is back to unset")
		})
	}
}
