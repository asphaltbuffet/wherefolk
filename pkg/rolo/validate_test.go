package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// findingFor returns the first Finding about the given field, and whether one
// was present at all.
func findingFor(findings []rolo.Finding, field string) (rolo.Finding, bool) {
	for _, f := range findings {
		if f.Field == field {
			return f, true
		}
	}
	return rolo.Finding{}, false
}

func TestValidateHouseholdsReportsProblems(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		checkFunc func(t *testing.T, findings []rolo.Finding)
	}{
		{
			name: "an unrecognised phone is a notice",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Nigel", Phone: "+44 20 7946 0958"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "phone")
				require.True(t, ok, "expected a finding about phone")
				assert.Equal(t, rolo.SeverityNotice, f.Severity)
				assert.Equal(t, rolo.HouseholdID("h_a"), f.Household)
				assert.Equal(t, rolo.PersonID("p_a"), f.Person)
				assert.Contains(t, f.Message, "+44 20 7946 0958",
					"the message should quote the value so the Editor can see what it means")
			},
		},
		{
			name: "an unparseable email is a warning",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "not an email"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "email")
				require.True(t, ok, "expected a finding about email")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
				assert.Contains(t, f.Message, "not an email")
			},
		},
		{
			name: "an email with a display name is a warning",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "Pat Novak <pat@example.com>"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "email")
				require.True(t, ok, "a display-name address should be flagged, not silently accepted")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
			},
		},
		{
			name: "a death date before a birth date is a warning",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Thomas",
					Birth: rolo.Date{Year: 1996, Month: 7, Day: 19},
					Death: rolo.Date{Year: 1990, Month: 1, Day: 8},
				}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "death")
				require.True(t, ok, "expected a finding about death")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
			},
		},
		{
			name: "an anniversary before an adult's birth is a warning",
			household: rolo.Household{
				ID:          "h_a",
				Anniversary: rolo.Date{Year: 1960},
				Adults:      []rolo.Person{{ID: "p_a", Given: "Pat", Birth: rolo.Date{Year: 1975}}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "anniversary")
				require.True(t, ok, "expected a finding about anniversary")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
				assert.Equal(t, rolo.PersonID(""), f.Person,
					"an anniversary belongs to the Household, not to one Person")
			},
		},
		{
			name: "problems are reported for dependents too",
			household: rolo.Household{
				ID:         "h_a",
				Adults:     []rolo.Person{{ID: "p_a", Given: "Pat"}},
				Dependents: []rolo.Person{{ID: "p_kid", Given: "Mia", Email: "not an email"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "email")
				require.True(t, ok)
				assert.Equal(t, rolo.PersonID("p_kid"), f.Person)
			},
		},
		{
			name: "several problems produce several findings",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{
					{ID: "p_a", Given: "Pat", Phone: "call the house", Email: "not an email"},
				},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				assert.Len(t, findings, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, rolo.ValidateHouseholds([]rolo.Household{tt.household}))
		})
	}
}

func TestValidateHouseholdsStaysQuiet(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
	}{
		{
			name: "a complete, well-formed household",
			household: rolo.Household{
				ID:          "h_a",
				Anniversary: rolo.Date{Year: 1991, Month: 6, Day: 15},
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Robert",
					Birth: rolo.Date{Year: 1965, Month: 3, Day: 12},
					Phone: "555-201-0001",
					Email: "robert@example.com",
				}},
			},
		},
		{
			name: "no contact details at all",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Mia"}},
			},
		},
		{
			name: "no birth date is not a problem during editing",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Harold", Phone: "555-201-0001"}},
			},
		},
		{
			name: "a deceased person with sensible dates",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Thomas",
					Birth: rolo.Date{Year: 1996, Month: 7, Day: 19},
					Death: rolo.Date{Year: 2022, Month: 1, Day: 8},
				}},
			},
		},
		{
			name: "a death date with no birth date cannot be compared",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Nettie", Death: rolo.Date{Year: 1995}}},
			},
		},
		{
			name: "an anniversary with no birth dates cannot be compared",
			household: rolo.Household{
				ID:          "h_a",
				Anniversary: rolo.Date{Year: 1961},
				Adults:      []rolo.Person{{ID: "p_a", Given: "Clyde"}},
			},
		},
		{
			name: "a year-only birth equal to the death year is not backwards",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Infant",
					Birth: rolo.Date{Year: 2020},
					Death: rolo.Date{Year: 2020},
				}},
			},
		},
		{
			name: "an email that is unusual but valid",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "pat@localhost"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, rolo.ValidateHouseholds([]rolo.Household{tt.household}),
				"validation must stay quiet about data that is merely incomplete")
		})
	}
}

func TestValidateHouseholdsAcrossTheDirectory(t *testing.T) {
	households := []rolo.Household{
		{ID: "h_a", Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "not an email"}}},
		{ID: "h_b", Adults: []rolo.Person{{ID: "p_b", Given: "Sam", Phone: "555-201-0002"}}},
		{ID: "h_c", Adults: []rolo.Person{{ID: "p_c", Given: "Nigel", Phone: "call the house"}}},
	}

	findings := rolo.ValidateHouseholds(households)
	require.Len(t, findings, 2)

	assert.Equal(t, rolo.HouseholdID("h_a"), findings[0].Household,
		"findings should arrive in document order")
	assert.Equal(t, rolo.HouseholdID("h_c"), findings[1].Household)
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		name     string
		severity rolo.Severity
		want     string
	}{
		{name: "notice", severity: rolo.SeverityNotice, want: "notice"},
		{name: "warning", severity: rolo.SeverityWarning, want: "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.severity.String())
		})
	}
}
