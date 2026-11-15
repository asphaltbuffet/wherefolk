package web_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestFormViewFromHousehold(t *testing.T) {
	tests := []struct {
		name  string
		h     rolo.Household
		check func(t *testing.T, got web.HouseholdFormViewForTest)
	}{
		{
			name: "every stored value reaches the form",
			h:    editable(),
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				require.Len(t, got.Adults, 2)
				assert.Equal(t, "p_clyd01", got.Adults[0].Key,
					"the key is the PersonID, which names the field")
				assert.Equal(t, "Clyde", got.Adults[0].Given)
				assert.Equal(t, "555-0142", got.Adults[0].Phone)
			},
		},
		{
			name: "a withheld field carries its value and a ticked box",
			h: rolo.Household{
				ID: "h_x",
				Adults: []rolo.Person{{
					ID: "p_x", Given: "Ray", Phone: "555-0199",
					Hidden: rolo.HiddenFields{Phone: true},
				}},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				require.Len(t, got.Adults, 1)
				assert.Equal(t, "555-0199", got.Adults[0].Phone,
					"ADR-0010: the editing UI never masks")
				assert.True(t, got.Adults[0].HiddenPhone)
			},
		},
		{
			name: "a deceased person's contact details are editable",
			h: rolo.Household{
				ID: "h_x",
				Adults: []rolo.Person{{
					ID: "p_x", Given: "Aden",
					Death: rolo.Date{Year: 1989},
					Phone: "555-0100",
				}},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				require.Len(t, got.Adults, 1)
				assert.Equal(t, "555-0100", got.Adults[0].Phone,
					"ADR-0010: suppression is export-only")
				assert.True(t, got.Adults[0].Deceased)
			},
		},
		{
			name: "address lines become one textarea value",
			h: rolo.Household{
				ID:      "h_x",
				Adults:  []rolo.Person{{ID: "p_x", Given: "Ray"}},
				Address: rolo.Address{Lines: []string{"9 Elm St", "Springfield, IL"}},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				assert.Equal(t, "9 Elm St\nSpringfield, IL", got.AddressText)
			},
		},
		{
			name: "a household with room offers a blank adult slot",
			h: rolo.Household{
				ID:     "h_x",
				Adults: []rolo.Person{{ID: "p_x", Given: "Dave"}},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				require.NotNil(t, got.NewSlot)
				assert.Equal(t, "new1", got.NewSlot.Key)
				assert.Empty(t, got.NewSlot.Given)
			},
		},
		{
			name: "a full household offers no adult slot",
			h:    editable(),
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				assert.Nil(t, got.NewSlot, "§3 caps a Household at two adults")
			},
		},
		{
			name: "the dependent slot is always offered",
			h:    editable(),
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				assert.Equal(t, "new2", got.NewDependent.Key,
					"a full household must still be able to gain a Dependent")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := web.FormViewFromHouseholdForTest(tt.h.ID, tt.h, nil)
			tt.check(t, got)
		})
	}
}

// TestDeceasedContactSuppressionIsPerPerson pins ADR-0010 against the form
// layer: the editing UI never masks, and that is a per-person fact rather
// than a per-household one. A Memorial Household (every adult deceased)
// still shows a living Dependent's contact details untouched, while a
// deceased person's own details remain visible and editable rather than
// blanked.
func TestDeceasedContactSuppressionIsPerPerson(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, got web.HouseholdFormViewForTest)
	}{
		{
			name: "a deceased adult's details are shown, not suppressed",
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()
				require.Len(t, got.Adults, 1)
				assert.Equal(t, "555-DEAD", got.Adults[0].Phone)
				assert.Equal(t, "dead@example.com", got.Adults[0].Email)
				assert.True(t, got.Adults[0].Deceased)
			},
		},
		{
			name: "a living Dependent keeps theirs, even in a Memorial Household",
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()
				require.Len(t, got.Dependents, 2)
				assert.Equal(t, "555-LIVING", got.Dependents[0].Phone,
					"a living relative's number is exactly what the Editor needs")
				assert.Equal(t, "living@example.com", got.Dependents[0].Email)
			},
		},
		{
			name: "a deceased Dependent's details are shown, not suppressed",
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()
				require.Len(t, got.Dependents, 2)
				assert.Equal(t, "555-GONEDEP", got.Dependents[1].Phone)
				assert.Equal(t, "gonedep@example.com", got.Dependents[1].Email)
			},
		},
	}

	h := rolo.Household{
		ID: "h_mem",
		Adults: []rolo.Person{{
			ID: "p_dead", Given: "Gone", Surname: "X",
			Birth: rolo.Date{Year: 1910}, Death: rolo.Date{Year: 1990},
			Phone: "555-DEAD", Email: "dead@example.com",
		}},
		Dependents: []rolo.Person{
			{
				ID: "p_living", Given: "Living", Surname: "X",
				Birth: rolo.Date{Year: 2010},
				Phone: "555-LIVING", Email: "living@example.com",
			},
			{
				ID: "p_gonedep", Given: "GoneDep", Surname: "X",
				Birth: rolo.Date{Year: 1950}, Death: rolo.Date{Year: 1975},
				Phone: "555-GONEDEP", Email: "gonedep@example.com",
			},
		},
	}

	got := web.FormViewFromHouseholdForTest(h.ID, h, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, got)
		})
	}
}

func TestFormViewFromSubmission(t *testing.T) {
	tests := []struct {
		name  string
		form  url.Values
		check func(t *testing.T, got web.HouseholdFormViewForTest)
	}{
		{
			name: "a refused date keeps the raw text and the message",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.p_clyd01.birth": {"June-ish 1998"},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				require.Len(t, got.Adults, 1)
				assert.Equal(t, "June-ish 1998", got.Adults[0].Birth,
					"the Editor's own typing comes back")
				assert.Contains(t, got.Adults[0].Errors["birth"], "is not a date this can read")
			},
		},
		{
			name: "navigation state is carried through the refusal",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"open":                  {"h_aden,h_clyde"},
				"pane":                  {"closed"},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				assert.Equal(t, "h_aden,h_clyde", got.Open)
				assert.Equal(t, "closed", got.Pane)
			},
		},
		{
			name: "a ticked remove survives a refusal",
			form: url.Values{
				"person.p_clyd01.given":  {"Clyde"},
				"person.p_clyd01.birth":  {"June-ish 1998"},
				"person.p_clyd01.remove": {"off", "on"},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				require.Len(t, got.Adults, 1)
				assert.True(t, got.Adults[0].Remove, "a pending removal is not silently dropped")
				assert.False(t, got.Adults[0].Promote)
			},
		},
		{
			name: "a ticked promote survives a refusal",
			form: url.Values{
				"person.p_carl01.given":    {"Carl"},
				"person.p_carl01.is_adult": {"false"},
				"person.p_carl01.birth":    {"soon"},
				"person.p_carl01.promote":  {"off", "on"},
			},
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()

				require.Len(t, got.Adults, 1, "the refusal path renders everyone in one list")
				assert.True(t, got.Adults[0].Promote)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, errs := web.ParseSubmissionForTest(tt.form, editable())

			got := web.FormViewFromSubmissionForTest("h_clyde", sub, errs)
			tt.check(t, got)
		})
	}
}

func TestHouseholdSummary(t *testing.T) {
	adult := rolo.Person{ID: "p_a", Given: "Ann"}
	child := rolo.Person{ID: "p_c", Given: "Cal"}

	twelve := make([]rolo.Person, 12)
	for i := range twelve {
		twelve[i] = rolo.Person{ID: rolo.PersonID(fmt.Sprintf("p_k%02d", i))}
	}

	tests := []struct {
		name string
		h    rolo.Household
		want string
	}{
		{
			name: "counts and a whole anniversary",
			h: rolo.Household{
				Adults:      []rolo.Person{adult, adult},
				Dependents:  []rolo.Person{child},
				Anniversary: rolo.Date{Year: 1962, Month: 6, Day: 14},
			},
			want: "Two adults, one dependent · Married June 14, 1962",
		},
		{
			name: "one adult and nothing else",
			h:    rolo.Household{Adults: []rolo.Person{adult}},
			want: "One adult",
		},
		{
			name: "an anniversary known only to the year",
			h:    rolo.Household{Adults: []rolo.Person{adult}, Anniversary: rolo.Date{Year: 1962}},
			want: "One adult · Married 1962",
		},
		{
			name: "an anniversary known to the month",
			h:    rolo.Household{Adults: []rolo.Person{adult}, Anniversary: rolo.Date{Year: 1962, Month: 6}},
			want: "One adult · Married June 1962",
		},
		{
			name: "large counts use digits",
			h:    rolo.Household{Adults: []rolo.Person{adult}, Dependents: twelve},
			want: "One adult, 12 dependents",
		},
		{
			name: "no people but an anniversary",
			h:    rolo.Household{Anniversary: rolo.Date{Year: 1962}},
			want: "Married 1962",
		},
		{
			name: "nothing to say says nothing",
			h:    rolo.Household{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := web.FormViewFromHouseholdForTest("h_x", tt.h, nil)
			assert.Equal(t, tt.want, got.Summary)
		})
	}
}

func TestLifespan(t *testing.T) {
	tests := []struct {
		name string
		p    rolo.Person
		want string
	}{
		{
			name: "both years known",
			p:    rolo.Person{ID: "p_x", Birth: rolo.Date{Year: 1963}, Death: rolo.Date{Year: 1981, Month: 2}},
			want: "1963–1981",
		},
		{
			name: "birth unknown",
			p:    rolo.Person{ID: "p_x", Death: rolo.Date{Year: 1981}},
			want: "died 1981",
		},
		{
			name: "living",
			p:    rolo.Person{ID: "p_x", Birth: rolo.Date{Year: 1963}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := web.FormViewFromHouseholdForTest("h_x", rolo.Household{Adults: []rolo.Person{tt.p}}, nil)
			require.Len(t, got.Adults, 1)
			assert.Equal(t, tt.want, got.Adults[0].Lifespan)
		})
	}
}

func TestFindingsFileWithTheirPerson(t *testing.T) {
	h := rolo.Household{
		ID:         "h_x",
		Adults:     []rolo.Person{{ID: "p_a", Given: "Ann"}},
		Dependents: []rolo.Person{{ID: "p_c", Given: "Cal"}},
	}
	findings := []rolo.Finding{
		{Person: "p_a", Message: "about Ann"},
		{Message: "about the household"},
		{Person: "p_c", Message: "about Cal"},
		{Person: "p_gone", Message: "about someone not here"},
	}

	messages := func(fs []web.FindingViewForTest) []string {
		out := make([]string, 0, len(fs))
		for _, f := range fs {
			out = append(out, f.Message)
		}
		return out
	}

	tests := []struct {
		name  string
		check func(t *testing.T, got web.HouseholdFormViewForTest)
	}{
		{
			name: "an adult's finding is on the adult",
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()
				assert.Equal(t, []string{"about Ann"}, messages(got.Adults[0].Findings))
			},
		},
		{
			name: "a dependent's finding is on the dependent",
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()
				assert.Equal(t, []string{"about Cal"}, messages(got.Dependents[0].Findings))
			},
		},
		{
			name: "an orphaned finding falls back to the household card",
			check: func(t *testing.T, got web.HouseholdFormViewForTest) {
				t.Helper()
				assert.Equal(t, []string{"about the household", "about someone not here"}, messages(got.Findings),
					"a finding is never dropped because its person has no card")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := web.FormViewFromHouseholdForTest("h_x", h, findings)
			require.Len(t, got.Adults, 1)
			require.Len(t, got.Dependents, 1)
			tt.check(t, got)
		})
	}
}
