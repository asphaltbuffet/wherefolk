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
			name: "four households",
			checkFunc: func(t *testing.T, doc *store.Document, _ *rolo.Tree) {
				t.Helper()
				assert.Len(t, doc.Households, 4)
			},
		},
		{
			name: "two roots, ordered by eldest adult's birth date",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				roots := tree.Roots()
				require.Len(t, roots, 2)
				assert.Equal(t, "Harold/June", roots[0].Label(), "Harold b. 1928 precedes Patricia b. 1952")
				assert.Equal(t, "Patricia", roots[1].Label())
			},
		},
		{
			name: "a married child with a dependent is its own household",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				kids := tree.Children("h_lang01")
				require.Len(t, kids, 1)
				assert.Equal(t, "Daniel/Claire", kids[0].Label())
			},
		},
		{
			name: "path disambiguates a nested household",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				path, err := tree.PathString("h_lang02")
				require.NoError(t, err)
				assert.Equal(t, "Harold/June › Robert/Susan › Daniel/Claire", path)
			},
		},
		{
			name: "an unmarried adult child remains a dependent",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
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
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
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
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_nova01")
				require.True(t, ok)
				assert.Equal(t, `Patricia "Pat" Novak`, h.Adults[0].DisplayName())
			},
		},
		{
			name: "multiple address lines survive",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_nova01")
				require.True(t, ok)
				assert.Len(t, h.Address.Lines, 3)
			},
		},
		{
			name: "a minor dependent is gated by age",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang02")
				require.True(t, ok)

				asOf := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
				require.Len(t, h.Dependents, 1)
				assert.True(t, h.Dependents[0].IsMinor(asOf), "Mia b. 2021 is a minor in 2026")
			},
		},
		{
			name: "the memorial household anchors the langford branch",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_meml01")
				require.True(t, ok)
				assert.True(t, h.IsMemorial(), "both adults are deceased")

				kids := tree.Children("h_meml01")
				require.Len(t, kids, 1)
				assert.Equal(t, "Robert/Susan", kids[0].Label())
			},
		},
		{
			name: "a household may share its parent's address",
			checkFunc: func(t *testing.T, _ *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang02")
				require.True(t, ok)
				assert.True(t, h.SharesAddress())
				assert.Equal(t, rolo.HouseholdID("h_lang01"), h.Address.SharedWith)
				assert.Empty(t, h.Address.Lines, "a shared address holds no lines of its own")
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

func TestExampleDirectoryHasNoFindings(t *testing.T) {
	doc, err := store.Load(examplePath)
	require.NoError(t, err)

	findings := rolo.ValidateHouseholds(doc.Households)
	assert.Empty(t, findings,
		"the example Directory should be clean; a finding here means either the data or the rules are wrong")
}

func TestExampleDirectoryIsAlreadyNormalised(t *testing.T) {
	doc, err := store.Load(examplePath)
	require.NoError(t, err)

	before, err := os.ReadFile(examplePath)
	require.NoError(t, err)

	for i := range doc.Households {
		doc.Households[i].Normalize()
	}

	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, doc))

	after, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, string(before), string(after),
		"normalising the example Directory must change nothing, or the fixture is not in house style")
}
