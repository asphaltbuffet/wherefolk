package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// searchTree builds a forest with two Daves in different Branches and one
// Dependent, which is the shape §4.3's example is drawn from.
func searchTree(t *testing.T) *rolo.Tree {
	t.Helper()

	tree, err := rolo.BuildTree([]rolo.Household{
		{
			ID: "h_aden",
			Adults: []rolo.Person{
				{ID: "p_aden", Given: "Aden", Surname: "Whitlock", Birth: rolo.Date{Year: 1910}},
				{ID: "p_nett", Given: "Nettie", Surname: "Whitlock", Birth: rolo.Date{Year: 1912}},
			},
		},
		{
			ID:     "h_clyde",
			Parent: "h_aden",
			Adults: []rolo.Person{
				{ID: "p_clyd", Given: "Clyde", Surname: "Whitlock", Birth: rolo.Date{Year: 1938}},
				{ID: "p_dori", Given: "Doris", Surname: "Whitlock", Birth: rolo.Date{Year: 1940}},
			},
		},
		{
			ID:     "h_dave",
			Parent: "h_clyde",
			Adults: []rolo.Person{
				{ID: "p_dave", Given: "Dave", Surname: "Whitlock", Birth: rolo.Date{Year: 1971}},
			},
			Dependents: []rolo.Person{
				{ID: "p_elli", Given: "Ellie", Surname: "Whitlock", Birth: rolo.Date{Year: 2009}},
			},
		},
		{
			ID: "h_harold",
			Adults: []rolo.Person{
				{ID: "p_haro", Given: "Harold", Surname: "Reeves", Birth: rolo.Date{Year: 1915}},
			},
		},
		{
			ID:     "h_dave2",
			Parent: "h_harold",
			Adults: []rolo.Person{
				{ID: "p_dave2", Given: "Dave", Aka: "Sonny", Surname: "Reeves", Birth: rolo.Date{Year: 1948}},
			},
		},
	})
	require.NoError(t, err)

	return tree
}

func TestSearchPeople(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		checkFunc func(t *testing.T, got []rolo.Match)
	}{
		{
			name:  "empty query returns nothing",
			query: "",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				assert.Empty(t, got)
			},
		},
		{
			name:  "whitespace-only query returns nothing",
			query: "   ",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				assert.Empty(t, got)
			},
		},
		{
			name:  "given name finds both Daves, each with its Path",
			query: "dave",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				require.Len(t, got, 2)
				assert.Equal(t, rolo.HouseholdID("h_dave"), got[0].Household)
				assert.Equal(t, "Aden/Nettie › Clyde/Doris › Dave", got[0].Path)
				assert.Equal(t, rolo.HouseholdID("h_dave2"), got[1].Household)
				assert.Equal(t, "Harold › Dave", got[1].Path)
			},
		},
		{
			name:  "match is case-insensitive",
			query: "DAVE",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				assert.Len(t, got, 2)
			},
		},
		{
			name:  "surname matches",
			query: "reeves",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				assert.Len(t, got, 2, "Harold and Dave Reeves")
			},
		},
		{
			name:  "nickname matches",
			query: "sonny",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				require.Len(t, got, 1)
				assert.Equal(t, rolo.PersonID("p_dave2"), got[0].Person.ID)
			},
		},
		{
			name:  "Dependents are searchable, not just adults",
			query: "ellie",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				require.Len(t, got, 1)
				assert.Equal(t, rolo.HouseholdID("h_dave"), got[0].Household,
					"a Dependent's match names the Household they are listed in")
			},
		},
		{
			name:  "partial substring matches",
			query: "whit",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				assert.Len(t, got, 6, "every Whitlock, adults and Dependents alike")
			},
		},
		{
			name:  "no match returns an empty slice, never nil-panics",
			query: "zebedee",
			checkFunc: func(t *testing.T, got []rolo.Match) {
				t.Helper()
				assert.Empty(t, got)
			},
		},
	}

	tree := searchTree(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, tree.SearchPeople(tt.query))
		})
	}
}

func TestSearchPeopleBirthName(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "birth name matches", query: "kowalski", want: 1},
		{name: "married surname still matches", query: "novak", want: 1},
	}

	tree, err := rolo.BuildTree([]rolo.Household{
		{
			ID: "h_nova",
			Adults: []rolo.Person{
				{ID: "p_pat", Given: "Patricia", Surname: "Novak", BirthName: "Kowalski"},
			},
		},
	})
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Len(t, tree.SearchPeople(tt.query), tt.want)
		})
	}
}
