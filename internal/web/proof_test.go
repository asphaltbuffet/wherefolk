package web_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func proofDocument() *store.Document {
	return &store.Document{Schema: store.CurrentSchema, Households: []rolo.Household{
		{ID: "h_live", Address: rolo.Address{Lines: []string{"12 Elm St"}}, Adults: []rolo.Person{
			{ID: "p_dar", Given: "Daryl", Surname: "Yoder"},
			{ID: "p_daw", Given: "Dawn", Surname: "Yoder", BirthName: "Mitchell"},
		}},
		{ID: "h_gone", Adults: []rolo.Person{
			{ID: "p_old", Given: "Aden", Surname: "Yoder", Death: rolo.Date{Year: 1990}},
		}},
	}}
}

func newProofServer(t *testing.T, settings *store.Settings, ex *fakeExporter, saver *recordingSaver) *web.Server {
	t.Helper()

	if saver == nil {
		saver = &recordingSaver{}
	}

	srv, err := web.New(store.State{Document: proofDocument(), Settings: settings},
		config.Config{}, testLogger(), web.Meta{}, saver.save,
		func() (rolo.HouseholdID, error) { return "h_x", nil },
		func() (rolo.PersonID, error) { return "p_x", nil },
		ex, fixedClock(time.Date(2026, time.November, 15, 21, 0, 0, 0, time.UTC)),
	)
	require.NoError(t, err)

	return srv
}

func TestProofDownload(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		exErr      error
		wantStatus int
		checkFunc  func(t *testing.T, res *http.Response, body string, ex *fakeExporter)
	}{
		{
			name: "living household downloads", target: "/h/h_live/proof", wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, res *http.Response, _ string, ex *fakeExporter) {
				require.Equal(t, "application/pdf", res.Header.Get("Content-Type"))
				require.Equal(t,
					`attachment; filename="proof_sheet_daryl_dawn_mitchell_yoder_2026-11-15.pdf"`,
					res.Header.Get("Content-Disposition"))
				require.Len(t, ex.proofs, 1)
				require.Equal(t, "Daryl & Dawn (Mitchell) Yoder", ex.proofs[0].Name)
				require.Equal(t, "Nov 15, 2026", ex.proofs[0].GeneratedAt)
			},
		},
		{
			name: "memorial has no sheet", target: "/h/h_gone/proof", wantStatus: http.StatusNotFound,
			checkFunc: func(t *testing.T, _ *http.Response, body string, ex *fakeExporter) {
				require.Contains(t, body, "nobody in this household is living")
				require.Empty(t, ex.proofs)
			},
		},
		{
			name: "unknown household", target: "/h/h_nope/proof", wantStatus: http.StatusNotFound,
			checkFunc: func(t *testing.T, _ *http.Response, body string, _ *fakeExporter) {
				require.Contains(t, body, "not in the Directory")
			},
		},
		{
			name: "render failure is a sentence", target: "/h/h_live/proof", exErr: errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			checkFunc: func(t *testing.T, _ *http.Response, body string, _ *fakeExporter) {
				require.Contains(t, body, "Something went wrong making the Proof Sheet")
				require.NotContains(t, body, "boom")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := &fakeExporter{pdf: []byte("%PDF-1.7"), err: tt.exErr}
			srv := newProofServer(t, &store.Settings{Schema: store.CurrentSettingsSchema}, ex, nil)

			rec := fetch(t, srv, tt.target)
			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			tt.checkFunc(t, rec.Result(), rec.Body.String(), ex)
		})
	}
}

func TestProofSection(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		contains   []string
		notContain []string
	}{
		{
			name: "living household offers the download", target: "/h/h_live",
			contains: []string{`href="/h/h_live/proof"`, "Download Proof Sheet"},
		},
		{
			name: "memorial explains", target: "/h/h_gone",
			contains: []string{"nobody in this household is living"}, notContain: []string{`/proof"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := fetch(t, newProofServer(t, nil, &fakeExporter{}, nil), tt.target)
			require.Equal(t, http.StatusOK, rec.Code)

			for _, s := range tt.contains {
				require.Contains(t, rec.Body.String(), s)
			}

			for _, s := range tt.notContain {
				require.NotContains(t, rec.Body.String(), s)
			}
		})
	}
}

func TestProofFilename(t *testing.T) {
	date := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name, household string
		want            string
	}{
		{
			name: "household name", household: "Daryl & Dawn (Mitchell) Yoder",
			want: "proof_sheet_daryl_dawn_mitchell_yoder_2026-11-15.pdf",
		},
		{name: "accents fold", household: "Zoë Dvořák", want: "proof_sheet_zoe_dvorak_2026-11-15.pdf"},
		{name: "nothing ASCII falls back to the id", household: "李明", want: "proof_sheet_h_abc_2026-11-15.pdf"},
		{name: "quote cannot break the header", household: `A "B" C`, want: "proof_sheet_a_b_c_2026-11-15.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, web.ProofFilenameForTest(tt.household, "h_abc", date))
		})
	}
}
