package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
)

func TestReplySave(t *testing.T) {
	saved := func(s string) *string { return &s }

	tests := []struct {
		name       string
		existing   string
		target     string
		raw        string
		wantStatus int
		wantSaved  *string
		checkFunc  func(t *testing.T, res *http.Response, body string)
	}{
		{
			name:       "saved, announced, back on the household",
			target:     "/h/h_live/proof/reply",
			raw:        "  Mail Dawn\r\n12 Elm St  ",
			wantStatus: http.StatusSeeOther,
			wantSaved:  saved("Mail Dawn\n12 Elm St"),
			checkFunc: func(t *testing.T, res *http.Response, _ string) {
				t.Helper()

				loc, err := url.Parse(res.Header.Get("Location"))
				require.NoError(t, err)
				require.Equal(t, "/h/h_live", loc.Path)
				require.Equal(t, "The Reply Instructions were saved.", loc.Query().Get("said"))
				require.NotEmpty(t, loc.Query().Get("undo"))
			},
		},
		{
			name: "cleared", existing: "Old", target: "/h/h_live/proof/reply", raw: "",
			wantStatus: http.StatusSeeOther, wantSaved: saved(""),
			checkFunc: func(t *testing.T, res *http.Response, _ string) {
				t.Helper()

				require.Contains(t, res.Header.Get("Location"), url.QueryEscape("The Reply Instructions were cleared."))
			},
		},
		{
			name: "unchanged is not a save", existing: "Same", target: "/h/h_live/proof/reply", raw: "Same",
			wantStatus: http.StatusSeeOther,
			checkFunc: func(t *testing.T, res *http.Response, _ string) {
				t.Helper()

				require.NotContains(t, res.Header.Get("Location"), "undo=")
			},
		},
		{
			name: "too long keeps the typing", target: "/h/h_live/proof/reply", raw: strings.Repeat("x", 401),
			wantStatus: http.StatusUnprocessableEntity,
			checkFunc: func(t *testing.T, _ *http.Response, body string) {
				t.Helper()

				require.Contains(t, body, "at most 400 characters")
				require.Contains(t, body, strings.Repeat("x", 401))
				require.Contains(t, body, `id="proof" open`)
			},
		},
		{
			name: "seven lines", target: "/h/h_live/proof/reply", raw: "1\n2\n3\n4\n5\n6\n7",
			wantStatus: http.StatusUnprocessableEntity,
			checkFunc: func(t *testing.T, _ *http.Response, body string) {
				t.Helper()

				require.Contains(t, body, "at most 6 lines")
			},
		},
		{
			name: "bidi override refused", target: "/h/h_live/proof/reply", raw: "Mail \u202eDawn",
			wantStatus: http.StatusUnprocessableEntity,
			checkFunc: func(t *testing.T, _ *http.Response, body string) {
				t.Helper()

				require.Contains(t, body, "ordinary text and line breaks")
			},
		},
		{
			name: "unknown household saves nothing", target: "/h/h_nope/proof/reply", raw: "Mail Dawn",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saver := &recordingSaver{}
			srv := newProofServer(t, store.NewSettings().WithReply(tt.existing), &fakeExporter{}, saver)

			rec := postTo(t, srv, tt.target, url.Values{"reply": {tt.raw}, "open": {""}, "pane": {""}})
			require.Equal(t, tt.wantStatus, rec.Code)

			if tt.wantSaved != nil {
				require.NotNil(t, saver.savedSettings)
				require.Equal(t, *tt.wantSaved, saver.savedSettings.Reply)
			} else {
				require.Zero(t, saver.calls)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, rec.Result(), rec.Body.String())
			}
		})
	}
}

func TestReplyUndoAndDisplay(t *testing.T) {
	tests := []struct {
		name      string
		existing  string
		checkFunc func(t *testing.T, srv *web.Server, ex *fakeExporter)
	}{
		{
			name: "undo restores the previous text, on the same household", existing: "Old",
			checkFunc: func(t *testing.T, srv *web.Server, _ *fakeExporter) {
				t.Helper()

				rec := postTo(t, srv, "/h/h_live/proof/reply", url.Values{"reply": {"New"}})
				loc, err := url.Parse(rec.Header().Get("Location"))
				require.NoError(t, err)

				res := postTo(t, srv, "/undo", url.Values{"token": {loc.Query().Get("undo")}, "at": {"h_live"}})
				require.Equal(t, http.StatusSeeOther, res.Code)
				require.True(t, strings.HasPrefix(res.Header().Get("Location"), "/h/h_live"))

				require.Contains(t, fetch(t, srv, "/h/h_live").Body.String(), ">Old</textarea>")
			},
		},
		{
			name: "empty warns", existing: "",
			checkFunc: func(t *testing.T, srv *web.Server, _ *fakeExporter) {
				t.Helper()

				require.Contains(t, fetch(t, srv, "/h/h_live").Body.String(),
					"No Reply Instructions yet — recipients will not know where to send corrections.")
			},
		},
		{
			name: "saved text reaches the sheet", existing: "Mail Dawn",
			checkFunc: func(t *testing.T, srv *web.Server, ex *fakeExporter) {
				t.Helper()

				require.Equal(t, http.StatusOK, fetch(t, srv, "/h/h_live/proof").Code)
				require.Len(t, ex.proofs, 1)
				require.Equal(t, "Mail Dawn", ex.proofs[0].Reply)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &fakeExporter{pdf: []byte("%PDF-1.7")}
			tt.checkFunc(t, newProofServer(t, store.NewSettings().WithReply(tt.existing), ex, nil), ex)
		})
	}
}
