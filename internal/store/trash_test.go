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

// day is a day after the fixture's epoch, for readable retention arithmetic.
func day(n int) time.Time {
	return time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n)
}

func entry(id, parent, sharedWith rolo.HouseholdID, deleted time.Time) store.TrashEntry {
	return store.TrashEntry{
		DeletedAt: deleted,
		Household: rolo.Household{
			ID:      id,
			Parent:  parent,
			Adults:  []rolo.Person{{ID: rolo.PersonID("p_" + id), Given: string(id)}},
			Address: rolo.Address{SharedWith: sharedWith},
		},
	}
}

func trashOf(entries ...store.TrashEntry) *store.Trash {
	return &store.Trash{Schema: store.CurrentTrashSchema, Entries: entries}
}

func ids(t *store.Trash) []rolo.HouseholdID {
	out := make([]rolo.HouseholdID, 0, len(t.Entries))
	for _, e := range t.Entries {
		out = append(out, e.Household.ID)
	}
	return out
}

func TestPurge(t *testing.T) {
	tests := []struct {
		name        string
		trash       *store.Trash
		now         time.Time
		want        []rolo.HouseholdID
		wantChanged bool
	}{
		{
			name:  "an entry inside its 30 days is kept",
			trash: trashOf(entry("h_a", "", "", day(0))),
			now:   day(29),
			want:  []rolo.HouseholdID{"h_a"},
		},
		{
			name:        "an entry past its 30 days is purged",
			trash:       trashOf(entry("h_a", "", "", day(0))),
			now:         day(31),
			want:        []rolo.HouseholdID{},
			wantChanged: true,
		},
		{
			name: "an expired parent is kept while a child that needs it is not expired",
			trash: trashOf(
				entry("h_clyde", "h_aden", "", day(0)),
				entry("h_dave", "h_clyde", "", day(20)),
			),
			now:  day(35),
			want: []rolo.HouseholdID{"h_clyde", "h_dave"},
		},
		{
			name: "anchoring is transitive, through a grandparent",
			trash: trashOf(
				entry("h_aden", "", "", day(0)),
				entry("h_clyde", "h_aden", "", day(1)),
				entry("h_dave", "h_clyde", "", day(20)),
			),
			now:  day(35),
			want: []rolo.HouseholdID{"h_aden", "h_clyde", "h_dave"},
		},
		{
			name: "a Shared Address target is anchored like a parent",
			trash: trashOf(
				entry("h_susan", "", "", day(0)),
				entry("h_harold", "", "h_susan", day(20)),
			),
			now:  day(35),
			want: []rolo.HouseholdID{"h_susan", "h_harold"},
		},
		{
			name: "the whole chain goes once its youngest member expires",
			trash: trashOf(
				entry("h_clyde", "h_aden", "", day(0)),
				entry("h_dave", "h_clyde", "", day(20)),
			),
			now:         day(51),
			want:        []rolo.HouseholdID{},
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := tt.trash.Purge(tt.now)

			assert.Equal(t, tt.want, ids(got))
			assert.Equal(t, tt.wantChanged, changed)

			if !tt.wantChanged {
				assert.Same(t, tt.trash, got, "an unchanged Trash must be the same value, so Persist skips its write")
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	doc := &store.Document{Schema: store.CurrentSchema, Households: []rolo.Household{{ID: "h_dave"}}}

	tests := []struct {
		name        string
		trash       *store.Trash
		want        []rolo.HouseholdID
		wantChanged bool
	}{
		{
			name:        "an entry whose Household is back in the document is dropped (ADR-0012)",
			trash:       trashOf(entry("h_dave", "", "", day(0)), entry("h_reeve", "", "", day(0))),
			want:        []rolo.HouseholdID{"h_reeve"},
			wantChanged: true,
		},
		{
			name:  "a Trash with no duplicates is returned as it is",
			trash: trashOf(entry("h_reeve", "", "", day(0))),
			want:  []rolo.HouseholdID{"h_reeve"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := tt.trash.Reconcile(doc)

			assert.Equal(t, tt.want, ids(got))
			assert.Equal(t, tt.wantChanged, changed)

			if !tt.wantChanged {
				assert.Same(t, tt.trash, got, "an unchanged Trash must be the same value, so Persist skips its write")
			}
		})
	}
}

func TestChain(t *testing.T) {
	doc := &store.Document{Schema: store.CurrentSchema, Households: []rolo.Household{{ID: "h_aden"}}}

	tests := []struct {
		name    string
		trash   *store.Trash
		id      rolo.HouseholdID
		want    []rolo.HouseholdID
		wantErr error
	}{
		{
			name:  "a Household whose parent is in the Directory comes back alone",
			trash: trashOf(entry("h_clyde", "h_aden", "", day(0))),
			id:    "h_clyde",
			want:  []rolo.HouseholdID{"h_clyde"},
		},
		{
			name: "a trashed parent comes back with it",
			trash: trashOf(
				entry("h_clyde", "h_aden", "", day(0)),
				entry("h_dave", "h_clyde", "", day(1)),
			),
			id:   "h_dave",
			want: []rolo.HouseholdID{"h_dave", "h_clyde"},
		},
		{
			name: "a trashed Shared Address target comes back with it",
			trash: trashOf(
				entry("h_susan", "h_aden", "", day(0)),
				entry("h_harold", "h_aden", "h_susan", day(1)),
			),
			id:   "h_harold",
			want: []rolo.HouseholdID{"h_harold", "h_susan"},
		},
		{
			name:    "a Household not in the Trash is refused",
			trash:   trashOf(),
			id:      "h_nope",
			wantErr: store.ErrNotInTrash,
		},
		{
			name:    "a Household needing one that is nowhere is refused",
			trash:   trashOf(entry("h_dave", "h_gone", "", day(0))),
			id:      "h_dave",
			wantErr: store.ErrUnrestorable,
		},
		{
			name:    "a Household already back in the Directory is not restorable",
			trash:   trashOf(entry("h_aden", "", "", day(0))),
			id:      "h_aden",
			wantErr: store.ErrNotInTrash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.trash.Chain(tt.id, doc)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeleteAndRestore(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "delete moves the Household from the document to the Trash",
			checkFunc: func(t *testing.T) {
				t.Helper()
				doc := sampleDocument()
				trash := store.NewTrash()
				carla := doc.Households[2]

				gotDoc, gotTrash, err := store.Delete(doc, trash, store.TrashEntry{DeletedAt: day(0), Household: carla})
				require.NoError(t, err)

				assert.Len(t, gotDoc.Households, 2)
				assert.Equal(t, []rolo.HouseholdID{"h_carla"}, ids(gotTrash))
				assert.Len(t, doc.Households, 3, "the input document must not change")
				assert.Empty(t, trash.Entries, "the input Trash must not change")
			},
		},
		{
			name: "deleting a Household not in the document is refused",
			checkFunc: func(t *testing.T) {
				t.Helper()
				_, _, err := store.Delete(sampleDocument(), store.NewTrash(),
					store.TrashEntry{Household: rolo.Household{ID: "h_nope"}})
				require.ErrorIs(t, err, store.ErrNotInDocument)
			},
		},
		{
			name: "restore brings back the chain and reports it, requested first",
			checkFunc: func(t *testing.T) {
				t.Helper()
				doc := &store.Document{Schema: store.CurrentSchema, Households: []rolo.Household{{ID: "h_aden"}}}
				trash := trashOf(entry("h_clyde", "h_aden", "", day(0)), entry("h_dave", "h_clyde", "", day(1)))

				gotDoc, gotTrash, restored, err := store.Restore(doc, trash, "h_dave")
				require.NoError(t, err)

				assert.Len(t, gotDoc.Households, 3)
				assert.Empty(t, gotTrash.Entries)
				require.Len(t, restored, 2)
				assert.Equal(t, rolo.HouseholdID("h_dave"), restored[0].ID)
				assert.Equal(t, rolo.HouseholdID("h_clyde"), restored[1].ID)
				assert.Len(t, doc.Households, 1, "the input document must not change")
			},
		},
		{
			name: "restore leaves unrelated entries in the Trash",
			checkFunc: func(t *testing.T) {
				t.Helper()
				doc := &store.Document{Schema: store.CurrentSchema, Households: []rolo.Household{{ID: "h_aden"}}}
				trash := trashOf(entry("h_clyde", "h_aden", "", day(0)), entry("h_dave", "h_clyde", "", day(1)))

				_, gotTrash, _, err := store.Restore(doc, trash, "h_clyde")
				require.NoError(t, err)

				assert.Equal(t, []rolo.HouseholdID{"h_dave"}, ids(gotTrash))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.checkFunc)
	}
}
