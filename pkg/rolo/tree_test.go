package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// sampleHouseholds returns a three-generation tree:
//
//	Aden/Nettie
//	├── Clyde/Doris   (b. 1938)
//	│   └── Dave/Diane
//	├── Harold/June   (b. 1941)
//	└── Susan/Ray     (b. 1944)
//
// Deliberately supplied out of order, to prove ordering is derived.
func sampleHouseholds() []rolo.Household {
	adult := func(given string, birthYear int) rolo.Person {
		return rolo.Person{Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
	}

	return []rolo.Household{
		{ID: "h_susan", Parent: "h_aden", Adults: []rolo.Person{adult("Susan", 1944), adult("Ray", 1943)}},
		{ID: "h_dave", Parent: "h_clyde", Adults: []rolo.Person{adult("Dave", 1971), adult("Diane", 1973)}},
		{ID: "h_aden", Adults: []rolo.Person{adult("Aden", 1910), adult("Nettie", 1912)}},
		{ID: "h_harold", Parent: "h_aden", Adults: []rolo.Person{adult("Harold", 1941), adult("June", 1942)}},
		{ID: "h_clyde", Parent: "h_aden", Adults: []rolo.Person{adult("Clyde", 1938), adult("Doris", 1940)}},
	}
}

func TestBuildTreeStructure(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, tree *rolo.Tree)
	}{
		{
			name: "one root",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				roots := tree.Roots()
				require.Len(t, roots, 1)
				assert.Equal(t, rolo.HouseholdID("h_aden"), roots[0].ID)
			},
		},
		{
			name: "children are ordered by eldest adult's birth date",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				kids := tree.Children("h_aden")
				require.Len(t, kids, 3)
				assert.Equal(t, "Clyde/Doris", kids[0].Label())
				assert.Equal(t, "Harold/June", kids[1].Label())
				assert.Equal(t, "Susan/Ray", kids[2].Label())
			},
		},
		{
			name: "a leaf has no children",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				assert.Empty(t, tree.Children("h_dave"))
			},
		},
		{
			name: "get returns a known household",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_dave")
				require.True(t, ok)
				assert.Equal(t, "Dave/Diane", h.Label())
			},
		},
		{
			name: "get reports an unknown household",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				_, ok := tree.Get("h_nobody")
				assert.False(t, ok)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := rolo.BuildTree(sampleHouseholds())
			require.NoError(t, err)
			tt.checkFunc(t, tree)
		})
	}
}

func TestTreePath(t *testing.T) {
	tests := []struct {
		name    string
		id      rolo.HouseholdID
		want    string
		wantErr bool
	}{
		{
			name: "three generations deep",
			id:   "h_dave",
			want: "Aden/Nettie › Clyde/Doris › Dave/Diane",
		},
		{
			name: "one generation deep",
			id:   "h_clyde",
			want: "Aden/Nettie › Clyde/Doris",
		},
		{
			name: "the root is its own path",
			id:   "h_aden",
			want: "Aden/Nettie",
		},
		{
			name:    "unknown household",
			id:      "h_nobody",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := rolo.BuildTree(sampleHouseholds())
			require.NoError(t, err)

			got, err := tree.PathString(tt.id)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTreeWalkOrder(t *testing.T) {
	tree, err := rolo.BuildTree(sampleHouseholds())
	require.NoError(t, err)

	type visit struct {
		label string
		depth int
	}

	var got []visit
	require.NoError(t, tree.Walk(func(h rolo.Household, depth int) error {
		got = append(got, visit{label: h.Label(), depth: depth})
		return nil
	}))

	want := []visit{
		{label: "Aden/Nettie", depth: 0},
		{label: "Clyde/Doris", depth: 1},
		{label: "Dave/Diane", depth: 2},
		{label: "Harold/June", depth: 1},
		{label: "Susan/Ray", depth: 1},
	}
	assert.Equal(t, want, got)
}

func TestBuildTreeRejectsMalformedData(t *testing.T) {
	adult := func(given string) rolo.Person {
		return rolo.Person{Given: given, Birth: rolo.Date{Year: 1950}}
	}

	tests := []struct {
		name       string
		households []rolo.Household
		wantErr    error
	}{
		{
			name: "parent that does not exist",
			households: []rolo.Household{
				{ID: "h_dave", Parent: "h_ghost", Adults: []rolo.Person{adult("Dave")}},
			},
			wantErr: rolo.ErrUnknownParent,
		},
		{
			name: "duplicate household id",
			households: []rolo.Household{
				{ID: "h_dave", Adults: []rolo.Person{adult("Dave")}},
				{ID: "h_dave", Adults: []rolo.Person{adult("David")}},
			},
			wantErr: rolo.ErrDuplicateID,
		},
		{
			name: "household is its own parent",
			households: []rolo.Household{
				{ID: "h_dave", Parent: "h_dave", Adults: []rolo.Person{adult("Dave")}},
			},
			wantErr: rolo.ErrCycle,
		},
		{
			name: "two households parent each other",
			households: []rolo.Household{
				{ID: "h_a", Parent: "h_b", Adults: []rolo.Person{adult("A")}},
				{ID: "h_b", Parent: "h_a", Adults: []rolo.Person{adult("B")}},
			},
			wantErr: rolo.ErrCycle,
		},
		{
			name: "household with no adults",
			households: []rolo.Household{
				{ID: "h_empty"},
			},
			wantErr: rolo.ErrNoAdults,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rolo.BuildTree(tt.households)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestBuildTreeAcceptsMultipleRoots(t *testing.T) {
	adult := func(given string, birthYear int) rolo.Person {
		return rolo.Person{Given: given, Birth: rolo.Date{Year: birthYear}}
	}

	households := []rolo.Household{
		{ID: "h_novak", Adults: []rolo.Person{adult("Patricia", 1952)}},
		{ID: "h_aden", Adults: []rolo.Person{adult("Aden", 1910)}},
	}

	tree, err := rolo.BuildTree(households)
	require.NoError(t, err)

	roots := tree.Roots()
	require.Len(t, roots, 2)
	assert.Equal(t, rolo.HouseholdID("h_aden"), roots[0].ID, "roots sort by eldest adult's birth date")
	assert.Equal(t, rolo.HouseholdID("h_novak"), roots[1].ID)
}
