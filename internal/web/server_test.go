package web_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

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
			got, err := web.New(tt.doc)

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
	}

	srv, err := web.New(sampleDocument())
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

// TestConcurrentReads exists to be run under -race: htmx issues overlapping
// requests, so handlers must not race even with a single Editor.
func TestConcurrentReads(t *testing.T) {
	tests := []struct {
		name    string
		workers int
	}{
		{name: "sixteen concurrent readers", workers: 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := web.New(sampleDocument())
			require.NoError(t, err)
			handler := srv.Handler()

			var wg sync.WaitGroup
			for range tt.workers {
				wg.Add(1)

				go func() {
					defer wg.Done()

					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
					assert.Equal(t, http.StatusOK, rec.Code)
				}()
			}
			wg.Wait()
		})
	}
}
