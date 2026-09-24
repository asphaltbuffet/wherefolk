package web_test

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, errs := web.ParseSubmissionForTest(tt.form, editable())

			got := web.FormViewFromSubmissionForTest("h_clyde", sub, errs)
			tt.check(t, got)
		})
	}
}
