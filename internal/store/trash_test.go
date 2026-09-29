package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// deletedAt is a fixed deletion time, so entries compare exactly.
var deletedAt = time.Date(2026, time.September, 20, 9, 30, 0, 0, time.UTC)

func sampleTrash() *store.Trash {
	return &store.Trash{
		Schema: store.CurrentTrashSchema,
		Entries: []store.TrashEntry{
			{
				DeletedAt: deletedAt,
				Path:      "Aden/Nettie › Clyde/Doris › Carla",
				Household: rolo.Household{
					ID:     "h_carla",
					Parent: "h_clyde",
					Adults: []rolo.Person{{ID: "p_carla01", Given: "Carla", Surname: "Whitlock"}},
				},
			},
		},
	}
}

func TestLoadTrash(t *testing.T) {
	tests := []struct {
		name      string
		contents  *string // nil: no file at all
		wantErr   error
		checkFunc func(t *testing.T, got *store.Trash)
	}{
		{
			name: "a missing file is an empty Trash, not an error",
			checkFunc: func(t *testing.T, got *store.Trash) {
				t.Helper()
				assert.Equal(t, store.CurrentTrashSchema, got.Schema)
				assert.Empty(t, got.Entries)
			},
		},
		{
			name:     "a Trash newer than the binary is refused",
			contents: ptr(`{"schema": 99, "entries": []}`),
			wantErr:  store.ErrSchemaTooNew,
		},
		{
			name:     "a Trash with no schema is refused",
			contents: ptr(`{"entries": []}`),
			wantErr:  store.ErrSchemaMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trash.json")
			if tt.contents != nil {
				require.NoError(t, os.WriteFile(path, []byte(*tt.contents), 0o600))
			}

			got, err := store.LoadTrash(path)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			tt.checkFunc(t, got)
		})
	}
}

func TestSaveTrashRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, path string, got *store.Trash)
	}{
		{
			name: "an entry survives a round trip exactly",
			checkFunc: func(t *testing.T, _ string, got *store.Trash) {
				t.Helper()
				require.Len(t, got.Entries, 1)
				e := got.Entries[0]
				assert.True(t, deletedAt.Equal(e.DeletedAt))
				assert.Equal(t, "Aden/Nettie › Clyde/Doris › Carla", e.Path)
				assert.Equal(t, rolo.HouseholdID("h_carla"), e.Household.ID)
				assert.Equal(t, rolo.HouseholdID("h_clyde"), e.Household.Parent)
			},
		},
		{
			name: "the file is readable only by its owner",
			checkFunc: func(t *testing.T, path string, _ *store.Trash) {
				t.Helper()
				info, err := os.Stat(path)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trash.json")
			require.NoError(t, store.SaveTrash(path, sampleTrash()))

			got, err := store.LoadTrash(path)
			require.NoError(t, err)

			tt.checkFunc(t, path, got)
		})
	}
}

func ptr(s string) *string { return &s }
