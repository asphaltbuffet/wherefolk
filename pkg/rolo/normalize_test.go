package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		want           string
		wantRecognized bool
	}{
		{
			name:           "already in house style",
			input:          "555-201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "parenthesised area code",
			input:          "(555) 201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "dot separated",
			input:          "555.201.0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "space separated",
			input:          "555 201 0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "bare digits",
			input:          "5552010001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "leading country code without plus",
			input:          "15552010001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "explicit +1 country code",
			input:          "+1 555 201 0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "explicit +1 with punctuation",
			input:          "+1 (555) 201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "surrounding whitespace is trimmed",
			input:          "  555-201-0001  ",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "empty is not a failure",
			input:          "",
			want:           "",
			wantRecognized: true,
		},
		{
			name:           "whitespace only becomes empty",
			input:          "   ",
			want:           "",
			wantRecognized: true,
		},
		{
			name:           "international number is kept verbatim",
			input:          "+44 20 7946 0958",
			want:           "+44 20 7946 0958",
			wantRecognized: false,
		},
		{
			name:           "another international number is kept verbatim",
			input:          "+61 2 9374 4000",
			want:           "+61 2 9374 4000",
			wantRecognized: false,
		},
		{
			name:           "extension is kept verbatim",
			input:          "555-201-0001 x12",
			want:           "555-201-0001 x12",
			wantRecognized: false,
		},
		{
			name:           "two numbers in one field are kept verbatim",
			input:          "555-201-0001, 555-201-0002",
			want:           "555-201-0001, 555-201-0002",
			wantRecognized: false,
		},
		{
			name:           "too few digits is kept verbatim",
			input:          "555-0001",
			want:           "555-0001",
			wantRecognized: false,
		},
		{
			name:           "vanity number is kept verbatim",
			input:          "1-800-FLOWERS",
			want:           "1-800-FLOWERS",
			wantRecognized: false,
		},
		{
			name:           "a note rather than a number is kept verbatim",
			input:          "call the house",
			want:           "call the house",
			wantRecognized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, recognized := rolo.NormalizePhone(tt.input)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantRecognized, recognized)
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already normal",
			input: "pat.novak@example.com",
			want:  "pat.novak@example.com",
		},
		{
			name:  "domain is lowercased",
			input: "pat@Example.COM",
			want:  "pat@example.com",
		},
		{
			name:  "local part keeps its case",
			input: "Pat.Novak@example.com",
			want:  "Pat.Novak@example.com",
		},
		{
			name:  "surrounding whitespace is trimmed",
			input: "  pat@example.com  ",
			want:  "pat@example.com",
		},
		{
			name:  "empty stays empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only becomes empty",
			input: "   ",
			want:  "",
		},
		{
			name:  "a value with no at sign is left alone but trimmed",
			input: "  not an email  ",
			want:  "not an email",
		},
		{
			name:  "only the last at sign separates the domain",
			input: "odd@local@Example.COM",
			want:  "odd@local@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rolo.NormalizeEmail(tt.input))
		})
	}
}

func TestPersonNormalize(t *testing.T) {
	tests := []struct {
		name      string
		person    rolo.Person
		checkFunc func(t *testing.T, p rolo.Person)
	}{
		{
			name:   "phone and email are both normalised",
			person: rolo.Person{Given: "Pat", Phone: "(555) 201-0001", Email: "pat@Example.COM"},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, "555-201-0001", p.Phone)
				assert.Equal(t, "pat@example.com", p.Email)
			},
		},
		{
			name:   "an unrecognised phone survives untouched but trimmed",
			person: rolo.Person{Given: "Nigel", Phone: "  +44 20 7946 0958  "},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, "+44 20 7946 0958", p.Phone)
			},
		},
		{
			name:   "names are trimmed",
			person: rolo.Person{Given: "  Pat  ", Surname: "  Novak  ", Aka: "  Patty  ", BirthName: "  Marsh  "},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, "Pat", p.Given)
				assert.Equal(t, "Novak", p.Surname)
				assert.Equal(t, "Patty", p.Aka)
				assert.Equal(t, "Marsh", p.BirthName)
			},
		},
		{
			name:   "empty fields stay empty",
			person: rolo.Person{Given: "Mia"},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Empty(t, p.Phone)
				assert.Empty(t, p.Email)
			},
		},
		{
			name: "identity and hidden flags are not touched",
			person: rolo.Person{
				ID:     "p_pat001",
				Given:  "Pat",
				Phone:  "(555) 201-0001",
				Hidden: rolo.HiddenFields{Phone: true},
			},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, rolo.PersonID("p_pat001"), p.ID)
				assert.True(t, p.Hidden.Phone, "normalising a withheld field must not un-withhold it")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.person
			p.Normalize()
			tt.checkFunc(t, p)
		})
	}
}

func TestHouseholdNormalize(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		checkFunc func(t *testing.T, h rolo.Household)
	}{
		{
			name: "adults are normalised",
			household: rolo.Household{
				Adults: []rolo.Person{
					{Given: "Pat", Phone: "555.201.0001"},
					{Given: "Sam", Email: "sam@Example.COM"},
				},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, "555-201-0001", h.Adults[0].Phone)
				assert.Equal(t, "sam@example.com", h.Adults[1].Email)
			},
		},
		{
			name: "dependents are normalised too",
			household: rolo.Household{
				Adults:     []rolo.Person{{Given: "Pat"}},
				Dependents: []rolo.Person{{Given: "Mia", Phone: "(555) 201-0020"}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, "555-201-0020", h.Dependents[0].Phone)
			},
		},
		{
			name: "address lines are trimmed and blanks removed",
			household: rolo.Household{
				Adults:  []rolo.Person{{Given: "Pat"}},
				Address: rolo.Address{Lines: []string{"  88 Oakwood Drive  ", "", "   ", "Shelbyville, IL 62565"}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, []string{"88 Oakwood Drive", "Shelbyville, IL 62565"}, h.Address.Lines)
			},
		},
		{
			name: "a shared address reference is not disturbed",
			household: rolo.Household{
				Adults:  []rolo.Person{{Given: "Pat"}},
				Address: rolo.Address{SharedWith: "h_lang01"},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, rolo.HouseholdID("h_lang01"), h.Address.SharedWith)
				assert.Empty(t, h.Address.Lines)
			},
		},
		{
			name: "an address of only blank lines becomes empty",
			household: rolo.Household{
				Adults:  []rolo.Person{{Given: "Pat"}},
				Address: rolo.Address{Lines: []string{"", "  "}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Empty(t, h.Address.Lines)
			},
		},
		{
			name: "structure is not touched",
			household: rolo.Household{
				ID:     "h_nova01",
				Parent: "h_lang01",
				Adults: []rolo.Person{{Given: "Pat", Phone: "555.201.0001"}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, rolo.HouseholdID("h_nova01"), h.ID)
				assert.Equal(t, rolo.HouseholdID("h_lang01"), h.Parent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.household
			h.Normalize()
			tt.checkFunc(t, h)
		})
	}
}
