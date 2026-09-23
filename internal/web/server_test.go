package web_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// testLogger discards output. Injecting it keeps each test's logging isolated,
// which a package-level default logger could not guarantee.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		doc     *store.Document
		wantErr bool
	}{
		{
			name: "valid document",
			doc:  sampleDocument(),
		},
		{
			name:    "nil document",
			doc:     nil,
			wantErr: true,
		},
		{
			name: "household pointing at an unknown parent",
			doc: &store.Document{
				Schema: store.CurrentSchema,
				Households: []rolo.Household{
					{
						ID:     "h_orphan",
						Parent: "h_missing",
						Adults: []rolo.Person{{ID: "p_x", Given: "X", Surname: "Y"}},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := web.New(
				tt.doc,
				config.Config{},
				testLogger(),
				web.Meta{DocumentPath: "/tmp/test/directory.json"},
			)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, got)

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func TestRouting(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		target   string
		wantCode int
	}{
		{name: "status page", method: http.MethodGet, target: "/status", wantCode: http.StatusOK},
		{name: "root", method: http.MethodGet, target: "/", wantCode: http.StatusOK},
		{name: "unknown path", method: http.MethodGet, target: "/nope", wantCode: http.StatusNotFound},
		{name: "post to status", method: http.MethodPost, target: "/status", wantCode: http.StatusMethodNotAllowed},
		{name: "vendored htmx", method: http.MethodGet, target: "/static/htmx.min.js", wantCode: http.StatusOK},
		{
			name:     "unknown static asset",
			method:   http.MethodGet,
			target:   "/static/nope.js",
			wantCode: http.StatusNotFound,
		},
	}

	// One server shared across the rows, which is safe only while every route
	// is a read: the subtests run sequentially and nothing mutates the document.
	// Work item 5's mutating routes will need a fresh server per row, or the
	// rows become order-dependent.
	srv, err := web.New(
		sampleDocument(),
		config.Config{},
		testLogger(),
		web.Meta{DocumentPath: "/tmp/test/directory.json"},
	)
	require.NoError(t, err)
	handler := srv.Handler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}

// TestConcurrentReads checks that handlers are safe to call concurrently,
// because htmx issues overlapping requests even with a single Editor.
//
// Today it cannot fail: the package has readers and no writers, and concurrent
// reads of data nobody mutates are not a data race — removing the handler's
// RLock leaves -race clean. It is a regression trip-wire for work item 5, when
// the write path makes the locking genuinely load-bearing, not proof that the
// locking is correct now.
func TestRenderFragmentOmitsLayout(t *testing.T) {
	tests := []struct {
		name      string
		page      string
		fragment  string
		data      any
		wantErr   bool
		checkFunc func(t *testing.T, body string)
	}{
		{
			name:     "a fragment renders without page chrome",
			page:     "status",
			fragment: "fragment_probe",
			data:     "hello",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "hello")
				assert.NotContains(t, body, "<!DOCTYPE html>", "a fragment is swapped into a page, not a page itself")
				assert.NotContains(t, body, "<body>")
			},
		},
		{
			name:     "an unknown page is an error, not a blank response",
			page:     "no_such_page",
			fragment: "fragment_probe",
			data:     nil,
			wantErr:  true,
		},
		{
			name:     "an unknown fragment is an error",
			page:     "status",
			fragment: "no_such_fragment",
			data:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := web.New(sampleDocument(), config.Config{}, testLogger(), web.Meta{})
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			err = srv.RenderFragmentForTest(t.Context(), rec, tt.page, tt.fragment, tt.data)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.checkFunc(t, rec.Body.String())
		})
	}
}

func TestConcurrentReads(t *testing.T) {
	tests := []struct {
		name    string
		workers int
	}{
		{name: "sixteen concurrent readers", workers: 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := web.New(
				sampleDocument(),
				config.Config{},
				testLogger(),
				web.Meta{DocumentPath: "/tmp/test/directory.json"},
			)
			require.NoError(t, err)
			handler := srv.Handler()

			var wg sync.WaitGroup
			for range tt.workers {
				wg.Go(func() {
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
					assert.Equal(t, http.StatusOK, rec.Code)
				})
			}
			wg.Wait()
		})
	}
}
