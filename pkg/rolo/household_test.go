package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func livingAdult(given string, birthYear int) rolo.Person {
	return rolo.Person{Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
}

func deceasedAdult(given string, birthYear, deathYear int) rolo.Person {
	return rolo.Person{
		Given:   given,
		Surname: "Whitlock",
		Birth:   rolo.Date{Year: birthYear},
		Death:   rolo.Date{Year: deathYear},
	}
}

func TestHouseholdIsMemorial(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      bool
	}{
		{
			name:      "both adults living",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Dave", 1971), livingAdult("Diane", 1973)}},
			want:      false,
		},
		{
			name: "one adult deceased, one living",
			household: rolo.Household{
				Adults: []rolo.Person{deceasedAdult("Clyde", 1938, 2019), livingAdult("Doris", 1940)},
			},
			want: false,
		},
		{
			name: "both adults deceased",
			household: rolo.Household{
				Adults: []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)},
			},
			want: true,
		},
		{
			name: "both adults deceased but a dependent survives is still memorial",
			household: rolo.Household{
				Adults:     []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)},
				Dependents: []rolo.Person{livingAdult("Junior", 1950)},
			},
			want: true,
		},
		{
			name:      "no adults at all is not memorial",
			household: rolo.Household{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.IsMemorial())
		})
	}
}

func TestHouseholdHasLivingMember(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      bool
	}{
		{
			name:      "living adult",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Doris", 1940)}},
			want:      true,
		},
		{
			name: "all adults deceased but a dependent lives",
			household: rolo.Household{
				Adults:     []rolo.Person{deceasedAdult("Aden", 1910, 1988)},
				Dependents: []rolo.Person{livingAdult("Junior", 1950)},
			},
			want: true,
		},
		{
			name: "everyone deceased",
			household: rolo.Household{
				Adults:     []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)},
				Dependents: []rolo.Person{deceasedAdult("Thomas", 1996, 2022)},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.HasLivingMember())
		})
	}
}

func TestHouseholdLabel(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      string
	}{
		{
			name:      "two adults join with a slash",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Dave", 1971), livingAdult("Diane", 1973)}},
			want:      "Dave/Diane",
		},
		{
			name:      "single adult is just the given name",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Doris", 1940)}},
			want:      "Doris",
		},
		{
			name: "deceased adults still label the household",
			household: rolo.Household{
				Adults: []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)},
			},
			want: "Aden/Nettie",
		},
		{
			name:      "no adults falls back to a placeholder",
			household: rolo.Household{ID: "h_3xqp1w"},
			want:      "(h_3xqp1w)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.Label())
		})
	}
}

func TestHouseholdName(t *testing.T) {
	adult := func(given, surname, birthName string) rolo.Person {
		return rolo.Person{Given: given, Surname: surname, BirthName: birthName}
	}

	tests := []struct {
		name   string
		adults []rolo.Person
		want   string
	}{
		{
			name:   "a shared surname prints once, with a birth name in parentheses",
			adults: []rolo.Person{adult("Daryl", "Yoder", ""), adult("Dawn", "Yoder", "Mitchell")},
			want:   "Daryl & Dawn (Mitchell) Yoder",
		},
		{
			name:   "a birth name equal to the surname is not shown",
			adults: []rolo.Person{adult("Daryl", "Yoder", ""), adult("Dawn", "Yoder", "Yoder")},
			want:   "Daryl & Dawn Yoder",
		},
		{
			name:   "different surnames name each adult in full, without birth names",
			adults: []rolo.Person{adult("Chris", "Yoder", ""), adult("Sam", "Patel", "Jones")},
			want:   "Chris Yoder & Sam Patel",
		},
		{
			name:   "a single adult prints their own name without a birth name",
			adults: []rolo.Person{adult("Dawn", "Yoder", "Mitchell")},
			want:   "Dawn Yoder",
		},
		{
			name:   "a nickname never appears",
			adults: []rolo.Person{{Given: "Patricia", Surname: "Novak", Aka: "Pat"}},
			want:   "Patricia Novak",
		},
		{
			name:   "an adult with an empty given name is skipped",
			adults: []rolo.Person{adult("Daryl", "Yoder", ""), adult("", "Yoder", "Mitchell")},
			want:   "Daryl Yoder",
		},
		{
			name:   "no adults is no name",
			adults: nil,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rolo.Household{ID: "h_test01", Adults: tt.adults}.Name())
		})
	}
}

func TestHouseholdSharesAddress(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      bool
	}{
		{
			name:      "own address",
			household: rolo.Household{Address: rolo.Address{Lines: []string{"1412 Oak St"}}},
			want:      false,
		},
		{
			name:      "shared with parent",
			household: rolo.Household{Address: rolo.Address{SharedWith: "h_9m2kfp"}},
			want:      true,
		},
		{
			name:      "no address at all",
			household: rolo.Household{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.SharesAddress())
		})
	}
}

func TestHouseholdEldestAdultBirth(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      rolo.Date
	}{
		{
			name: "earlier of two birth dates",
			household: rolo.Household{Adults: []rolo.Person{
				{Given: "Diane", Birth: rolo.Date{Year: 1973, Month: 8, Day: 19}},
				{Given: "Dave", Birth: rolo.Date{Year: 1971, Month: 3, Day: 2}},
			}},
			want: rolo.Date{Year: 1971, Month: 3, Day: 2},
		},
		{
			name: "ignores adults with no birth date",
			household: rolo.Household{Adults: []rolo.Person{
				{Given: "Unknown"},
				{Given: "Dave", Birth: rolo.Date{Year: 1971}},
			}},
			want: rolo.Date{Year: 1971},
		},
		{
			name:      "no adults yields the zero date",
			household: rolo.Household{},
			want:      rolo.Date{},
		},
		{
			name:      "no adult has a birth date yields the zero date",
			household: rolo.Household{Adults: []rolo.Person{{Given: "Unknown"}}},
			want:      rolo.Date{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.EldestAdultBirth())
		})
	}
}

func TestAddressHidden(t *testing.T) {
	tests := []struct {
		name string
		h    rolo.Household
		want bool
	}{
		{
			name: "address with no flag is visible",
			h:    rolo.Household{Address: rolo.Address{Lines: []string{"1412 Oak St"}}},
			want: false,
		},
		{
			name: "address marked hidden is hidden",
			h:    rolo.Household{Address: rolo.Address{Lines: []string{"1412 Oak St"}, Hidden: true}},
			want: true,
		},
		{
			name: "household with no address at all is not hidden",
			h:    rolo.Household{},
			want: false,
		},
		{
			name: "a shared address may itself be hidden",
			h:    rolo.Household{Address: rolo.Address{SharedWith: "h_parent", Hidden: true}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.h.AddressHidden())
		})
	}
}
