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

const examplePath = "../../testdata/directory.json"

func TestLoadExampleDirectory(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, doc *store.Document, tree *rolo.Tree)
	}{
		{
			name: "three households",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				assert.Len(t, doc.Households, 3)
			},
		},
		{
			name: "two roots, ordered by eldest adult's birth date",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				roots := tree.Roots()
				require.Len(t, roots, 2)
				assert.Equal(t, "Patricia", roots[0].Label(), "Patricia b. 1952 precedes Robert b. 1965")
				assert.Equal(t, "Robert/Susan", roots[1].Label())
			},
		},
		{
			name: "a married child with a dependent is its own household",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				kids := tree.Children("h_lang01")
				require.Len(t, kids, 1)
				assert.Equal(t, "Daniel/Claire", kids[0].Label())
			},
		},
		{
			name: "path disambiguates a nested household",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				path, err := tree.PathString("h_lang02")
				require.NoError(t, err)
				assert.Equal(t, "Robert/Susan › Daniel/Claire", path)
			},
		},
		{
			name: "an unmarried adult child remains a dependent",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang01")
				require.True(t, ok)

				var emma rolo.Person
				for _, d := range h.Dependents {
					if d.Given == "Emma" {
						emma = d
					}
				}
				require.Equal(t, rolo.PersonID("p_emm001"), emma.ID)
				assert.Equal(t, "555-201-0020", emma.Phone,
					"a dependent carries contact details; this is an address book")
			},
		},
		{
			name: "a deceased dependent keeps both dates",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang01")
				require.True(t, ok)

				var thomas rolo.Person
				for _, d := range h.Dependents {
					if d.Given == "Thomas" {
						thomas = d
					}
				}
				require.True(t, thomas.IsDeceased())
				assert.Equal(t, rolo.Date{Year: 1996, Month: 7, Day: 19}, thomas.Birth)
				assert.Equal(t, rolo.Date{Year: 2022, Month: 1, Day: 8}, thomas.Death)
			},
		},
		{
			name: "a nickname renders in double quotes",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_nova01")
				require.True(t, ok)
				assert.Equal(t, `Patricia "Pat" Novak`, h.Adults[0].DisplayName())
			},
		},
		{
			name: "multiple address lines survive",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_nova01")
				require.True(t, ok)
				assert.Len(t, h.Address.Lines, 3)
			},
		},
		{
			name: "a minor dependent is gated by age",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang02")
				require.True(t, ok)

				asOf := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
				require.Len(t, h.Dependents, 1)
				assert.True(t, h.Dependents[0].IsMinor(asOf), "Mia b. 2021 is a minor in 2026")
			},
		},
		{
			name: "no household in the example is memorial",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				require.NoError(t, tree.Walk(func(h rolo.Household, _ int) error {
					assert.False(t, h.IsMemorial(), "%s should not be memorial", h.Label())
					return nil
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := store.Load(examplePath)
			require.NoError(t, err)

			tree, err := doc.Tree()
			require.NoError(t, err)

			tt.checkFunc(t, doc, tree)
		})
	}
}

func TestExampleDirectorySurvivesSaveLoad(t *testing.T) {
	original, err := store.Load(examplePath)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, original))

	reloaded, err := store.Load(path)
	require.NoError(t, err)

	assert.Equal(t, original, reloaded, "a save/load cycle must not alter the document")
}

func TestExampleDirectoryIsCanonicallyFormatted(t *testing.T) {
	original, err := store.Load(examplePath)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, original))

	want, err := os.ReadFile(path)
	require.NoError(t, err)

	got, err := os.ReadFile(examplePath)
	require.NoError(t, err)

	assert.Equal(t, string(want), string(got),
		"testdata/directory.json should match what Save produces; run the test, then copy the saved file over it")
}
