package web_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// fakeExporter stands in for render.Renderer. It records every Directory it
// is asked to render, so a test can assert both that the tier filter ran and
// that a refused request never reached the renderer at all.
type fakeExporter struct {
	pdf   []byte
	pages [][]byte
	err   error

	got []render.Directory
}

func (f *fakeExporter) PDF(_ context.Context, d render.Directory) ([]byte, error) {
	f.got = append(f.got, d)
	return f.pdf, f.err
}

func (f *fakeExporter) SVG(_ context.Context, d render.Directory) ([][]byte, error) {
	f.got = append(f.got, d)
	return f.pages, f.err
}

// fixedClock always reports t.
func fixedClock(t time.Time) web.Clock { return func() time.Time { return t } }

// testClock is the clock every non-export test uses; its value is irrelevant
// to them.
var testClock = fixedClock(time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC))

// fixturePDF is a real one-page PDF, so encryption runs for real in web tests.
func fixturePDF(t *testing.T) []byte {
	t.Helper()

	b, err := os.ReadFile("../render/testdata/minimal.pdf")
	require.NoError(t, err)

	return b
}

// newExportServer builds a Server over sampleDocument with the given config,
// exporter and time.
func newExportServer(t *testing.T, cfg config.Config, ex *fakeExporter, now time.Time) *web.Server {
	t.Helper()

	srv, err := web.New(sampleDocument(), cfg, testLogger(), web.Meta{},
		func(*store.Document) error { return nil },
		func() (rolo.HouseholdID, error) { return "h_x", nil },
		func() (rolo.PersonID, error) { return "p_x", nil },
		ex, fixedClock(now),
	)
	require.NoError(t, err)

	return srv
}

// chicago is a zone west of UTC with no tzdata dependency.
var chicago = time.FixedZone("CDT", -5*60*60)

// tokyo is a zone east of UTC.
var tokyo = time.FixedZone("JST", 9*60*60)

func TestExportDate(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "late evening west of UTC stays on the local date",
			now:  time.Date(2026, time.September, 25, 23, 30, 0, 0, chicago),
			want: time.Date(2026, time.September, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "just after midnight east of UTC is already the new local date",
			now:  time.Date(2026, time.September, 26, 0, 30, 0, 0, tokyo),
			want: time.Date(2026, time.September, 26, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "utc is its own date at midnight",
			now:  time.Date(2026, time.September, 25, 17, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.September, 25, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, web.ExportDateForTest(tt.now))
		})
	}
}

func TestExportPDF(t *testing.T) {
	const passphrase = "correcthorsebattery" // no spaces: pdfcpu's own Decrypt, our test oracle, wrongly rejects them (see render/encrypt.go)

	evening := time.Date(2026, time.September, 25, 23, 30, 0, 0, chicago)

	tests := []struct {
		name       string
		target     string
		cfg        config.Config
		exporter   func(t *testing.T) *fakeExporter
		wantStatus int
		checkFunc  func(t *testing.T, rec *httptest.ResponseRecorder, ex *fakeExporter)
	}{
		{
			name:       "downloads as an attachment named for the tier and the local date",
			target:     "/export/pdf?tier=call",
			exporter:   func(*testing.T) *fakeExporter { return &fakeExporter{pdf: []byte("%PDF-fake")} },
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, _ *fakeExporter) {
				t.Helper()
				assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
				assert.Equal(t, `attachment; filename="family_directory_call_2026-09-25.pdf"`,
					rec.Header().Get("Content-Disposition"), "no spaces; the host's date, not UTC's")
				assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
				assert.Equal(t, "%PDF-fake", rec.Body.String())
			},
		},
		{
			name:       "the tier filter runs before rendering",
			target:     "/export/pdf?tier=mail",
			exporter:   func(*testing.T) *fakeExporter { return &fakeExporter{pdf: []byte("%PDF-fake")} },
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, _ *httptest.ResponseRecorder, ex *fakeExporter) {
				t.Helper()
				require.Len(t, ex.got, 1)
				d := ex.got[0]
				assert.Equal(t, "Mail", d.Tier)
				assert.Equal(t, "September 25, 2026", d.GeneratedAt)
				require.GreaterOrEqual(t, len(d.Households), 2)
				require.Equal(t, "Clyde/Doris", d.Households[1].Label)
				assert.Empty(t, d.Households[1].Adults[0].Phone, "Mail prints no phones (§5.2)")
			},
		},
		{
			name:   "full is encrypted with the passphrase",
			target: "/export/pdf?tier=full",
			cfg:    config.Config{FullPassphrase: passphrase},
			exporter: func(t *testing.T) *fakeExporter {
				t.Helper()
				return &fakeExporter{pdf: fixturePDF(t)}
			},
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, _ *fakeExporter) {
				t.Helper()
				assert.Equal(t, `attachment; filename="family_directory_full_2026-09-25.pdf"`,
					rec.Header().Get("Content-Disposition"))

				body := rec.Body.Bytes()
				assert.Contains(t, string(body), "/Encrypt")

				var out bytes.Buffer
				require.NoError(t, api.Decrypt(bytes.NewReader(body), &out,
					model.NewAESConfiguration(passphrase, "", 256)), "the family passphrase opens it")
				assert.Error(t, api.Decrypt(bytes.NewReader(body), &out,
					model.NewAESConfiguration("wrong horse battery", "", 256)))
			},
		},
		{
			name:       "full is refused without a passphrase, and never rendered",
			target:     "/export/pdf?tier=full",
			exporter:   func(*testing.T) *fakeExporter { return &fakeExporter{pdf: []byte("%PDF-fake")} },
			wantStatus: http.StatusForbidden,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, ex *fakeExporter) {
				t.Helper()
				assert.Empty(t, ex.got, "an unencrypted Full PDF must never be produced")
				assert.Contains(t, rec.Body.String(), "passphrase hasn't been set up")
			},
		},
		{
			name:       "an unknown tier is refused",
			target:     "/export/pdf?tier=everything",
			exporter:   func(*testing.T) *fakeExporter { return &fakeExporter{} },
			wantStatus: http.StatusBadRequest,
			checkFunc: func(t *testing.T, _ *httptest.ResponseRecorder, ex *fakeExporter) {
				t.Helper()
				assert.Empty(t, ex.got)
			},
		},
		{
			name:       "a missing tier is refused",
			target:     "/export/pdf",
			exporter:   func(*testing.T) *fakeExporter { return &fakeExporter{} },
			wantStatus: http.StatusBadRequest,
			checkFunc: func(t *testing.T, _ *httptest.ResponseRecorder, ex *fakeExporter) {
				t.Helper()
				assert.Empty(t, ex.got)
			},
		},
		{
			name:       "a render failure is an Editor-facing message, not the error text",
			target:     "/export/pdf?tier=call",
			exporter:   func(*testing.T) *fakeExporter { return &fakeExporter{err: errors.New("typst: exit status 1")} },
			wantStatus: http.StatusInternalServerError,
			checkFunc: func(t *testing.T, rec *httptest.ResponseRecorder, _ *fakeExporter) {
				t.Helper()
				assert.Contains(t, rec.Body.String(), "Please try again")
				assert.NotContains(t, rec.Body.String(), "typst")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ex := tt.exporter(t)
			srv := newExportServer(t, tt.cfg, ex, evening)

			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
			tt.checkFunc(t, rec, ex)
		})
	}
}
