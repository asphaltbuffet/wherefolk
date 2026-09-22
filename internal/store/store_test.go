package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func sampleDocument() *store.Document {
	adult := func(id rolo.PersonID, given string, birthYear int) rolo.Person {
		return rolo.Person{ID: id, Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
	}

	return &store.Document{
		Schema: store.CurrentSchema,
		Households: []rolo.Household{
			{
				ID:     "h_aden",
				Adults: []rolo.Person{adult("p_aden01", "Aden", 1910), adult("p_nett01", "Nettie", 1912)},
			},
			{
				ID:     "h_clyde",
				Parent: "h_aden",
				Adults: []rolo.Person{adult("p_clyd01", "Clyde", 1938), adult("p_dori01", "Doris", 1940)},
				Dependents: []rolo.Person{
					{
						ID:      "p_carl01",
						Given:   "Carl",
						Surname: "Whitlock",
						Birth:   rolo.Date{Year: 1963},
						Hidden:  rolo.HiddenFields{Phone: true},
					},
				},
				Anniversary: rolo.Date{Year: 1961, Month: 6, Day: 14},
				Address:     rolo.Address{Lines: []string{"88 Oakwood Drive", "Shelbyville, IL 62565"}},
			},
			{
				ID:      "h_carla",
				Parent:  "h_clyde",
				Adults:  []rolo.Person{adult("p_carla01", "Carla", 1965)},
				Address: rolo.Address{SharedWith: "h_clyde"},
			},
		},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, got *store.Document)
	}{
		{
			name: "schema is preserved",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				assert.Equal(t, store.CurrentSchema, got.Schema)
			},
		},
		{
			name: "households survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				require.Len(t, got.Households, 3)
			},
		},
		{
			name: "dates survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				h, ok := tree.Get("h_clyde")
				require.True(t, ok)
				assert.Equal(t, rolo.Date{Year: 1961, Month: 6, Day: 14}, h.Anniversary)
				assert.Equal(t, rolo.Date{Year: 1938}, h.Adults[0].Birth)
			},
		},
		{
			name: "addresses survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				h, ok := tree.Get("h_clyde")
				require.True(t, ok)
				assert.Equal(t, []string{"88 Oakwood Drive", "Shelbyville, IL 62565"}, h.Address.Lines)
			},
		},
		{
			name: "parent links survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				path, err := tree.PathString("h_clyde")
				require.NoError(t, err)
				assert.Equal(t, "Aden/Nettie › Clyde/Doris", path)
			},
		},
		{
			name: "a dependent survives the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				h, ok := tree.Get("h_clyde")
				require.True(t, ok)
				require.Len(t, h.Dependents, 1)
				assert.Equal(t, rolo.PersonID("p_carl01"), h.Dependents[0].ID)
				assert.Equal(t, "Carl", h.Dependents[0].Given)
			},
		},
		{
			name: "a withheld field survives the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				h, ok := tree.Get("h_clyde")
				require.True(t, ok)
				require.Len(t, h.Dependents, 1)
				assert.True(t, h.Dependents[0].Hidden.Phone)
				assert.False(t, h.Dependents[0].Hidden.Email)
				assert.False(t, h.Dependents[0].Hidden.Birth)
			},
		},
		{
			name: "a shared address survives as a reference",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				h, ok := tree.Get("h_carla")
				require.True(t, ok)
				assert.True(t, h.SharesAddress())
				assert.Equal(t, rolo.HouseholdID("h_clyde"), h.Address.SharedWith)
				assert.Empty(t, h.Address.Lines)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "directory.json")
			require.NoError(t, store.Save(path, sampleDocument()))

			got, err := store.Load(path)
			require.NoError(t, err)
			tt.checkFunc(t, got)
		})
	}
}

func TestLoadRejectsBadDocuments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{
			name:    "schema newer than this binary understands",
			content: `{"schema": 99, "households": []}`,
			wantErr: store.ErrSchemaTooNew,
		},
		{
			name:    "no schema field at all",
			content: `{"households": []}`,
			wantErr: store.ErrSchemaMissing,
		},
		{
			name:    "household names a parent that does not exist",
			content: `{"schema": 1, "households": [{"id": "h_a", "parent": "h_ghost", "adults": [{"id": "p_a", "given": "A", "birth": "1950"}]}]}`,
			wantErr: rolo.ErrUnknownParent,
		},
		{
			name:    "duplicate household ids",
			content: `{"schema": 1, "households": [{"id": "h_a", "adults": [{"id": "p_a", "given": "A", "birth": "1950"}]}, {"id": "h_a", "adults": [{"id": "p_b", "given": "B", "birth": "1950"}]}]}`,
			wantErr: rolo.ErrDuplicateID,
		},
		{
			name:    "household with no adults",
			content: `{"schema": 1, "households": [{"id": "h_a", "adults": []}]}`,
			wantErr: rolo.ErrNoAdults,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "directory.json")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			_, err := store.Load(path)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := store.Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"schema": 1, "households": [`), 0o600))

	_, err := store.Load(path)
	require.Error(t, err)
}

func TestSaveIsReadableAndIndented(t *testing.T) {
	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, sampleDocument()))

	b, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Contains(t, string(b), "\n  ", "the document is hand-edited during repair and must be indented")
	assert.Contains(t, string(b), `"schema": 1`)
}
