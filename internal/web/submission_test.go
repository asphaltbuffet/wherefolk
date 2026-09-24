package web_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// editable is the Household the submissions below are posted against. Its
// people are the only ones a submission may name.
func editable() rolo.Household {
	return rolo.Household{
		ID: "h_clyde",
		Adults: []rolo.Person{
			{ID: "p_clyd01", Given: "Clyde", Surname: "Whitlock", Phone: "555-0142"},
			{ID: "p_dori01", Given: "Doris", Surname: "Whitlock", Email: "doris@example.com"},
		},
		Dependents: []rolo.Person{
			{ID: "p_carl01", Given: "Carl", Surname: "Whitlock"},
		},
	}
}

func TestParseSubmission(t *testing.T) {
	tests := []struct {
		name  string
		form  url.Values
		check func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest)
	}{
		{
			name: "a plain field edit",
			form: url.Values{
				"person.p_clyd01.given":   {"Clyde"},
				"person.p_clyd01.surname": {"Whitlock"},
				"person.p_clyd01.phone":   {"555.201.0001"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)
				require.Len(t, got.People, 1)
				assert.Equal(t, rolo.PersonID("p_clyd01"), got.People[0].ID)
				assert.Equal(t, "555.201.0001", got.People[0].Phone,
					"parsing does not normalise; that happens after the refusal check")
			},
		},
		{
			name: "an empty value clears the field",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.p_clyd01.phone": {""},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)
				require.Len(t, got.People, 1)
				assert.Empty(t, got.People[0].Phone)
			},
		},
		{
			name: "an unparseable date is a field error, not a finding",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.p_clyd01.birth": {"June-ish 1998"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Len(t, errs, 1)
				assert.Equal(t, rolo.PersonID("p_clyd01"), errs[0].Person)
				assert.Equal(t, "birth", errs[0].Field)
				assert.Equal(t, "June-ish 1998", errs[0].Value,
					"the raw value is kept so the form can re-render what was typed")
				assert.NotContains(t, errs[0].Message, "parse date",
					"§4.5: the message is for the Editor, not the parser's own text")

				require.Len(t, got.People, 1)
				assert.Equal(t, "June-ish 1998", got.People[0].BirthRaw)
			},
		},
		{
			name: "a hidden checkbox pair resolves to the checked state",
			form: url.Values{
				"person.p_clyd01.given":        {"Clyde"},
				"person.p_clyd01.hidden.phone": {"off", "on"},
				"person.p_clyd01.hidden.email": {"off"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)
				require.Len(t, got.People, 1)
				assert.True(t, got.People[0].Hidden.Phone, "the paired hidden input is followed by on")
				assert.False(t, got.People[0].Hidden.Email, "off alone means unticked")
			},
		},
		{
			name: "a person outside this household is ignored",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.p_reev01.given": {"Intruder"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs, "a stale form is ordinary, not an error")
				require.Len(t, got.People, 1)
				assert.Equal(t, rolo.PersonID("p_clyd01"), got.People[0].ID)
			},
		},
		{
			name: "a blank new-person slot is ignored",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.new1.given":     {""},
				"person.new1.surname":   {""},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)
				require.Len(t, got.People, 1)
			},
		},
		{
			name: "a filled new-person slot is a new person",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"person.new1.given":     {"Diane"},
				"person.new1.surname":   {"Whitlock"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)
				require.Len(t, got.People, 2)
				assert.True(t, got.People[1].New)
				assert.Empty(t, got.People[1].ID, "the ID is minted when the submission is applied")
				assert.Equal(t, "Diane", got.People[1].Given)
			},
		},
		{
			name: "address lines split on newlines",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"address.lines":         {"1412 Oak St\r\nSpringfield, IL 62704"},
				"address.hidden":        {"off", "on"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)
				assert.Equal(t, []string{"1412 Oak St", "Springfield, IL 62704"}, got.AddressLines)
				assert.True(t, got.AddressHidden)
			},
		},
		{
			name: "remove and promote are recorded",
			form: url.Values{
				"person.p_clyd01.given":   {"Clyde"},
				"person.p_carl01.given":   {"Carl"},
				"person.p_carl01.remove":  {"on"},
				"person.p_dori01.given":   {"Doris"},
				"person.p_dori01.promote": {"on"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)

				byID := map[rolo.PersonID]web.PersonSubmissionForTest{}
				for _, p := range got.People {
					byID[p.ID] = p
				}

				assert.True(t, byID["p_carl01"].Remove)
				assert.True(t, byID["p_dori01"].Promote)
				assert.False(t, byID["p_clyd01"].Remove)
			},
		},
		{
			name: "navigation state rides along",
			form: url.Values{
				"person.p_clyd01.given": {"Clyde"},
				"open":                  {"h_aden,h_clyde"},
				"pane":                  {"closed"},
			},
			check: func(t *testing.T, got web.SubmissionForTest, errs []web.FieldErrorForTest) {
				t.Helper()

				require.Empty(t, errs)
				assert.Equal(t, "h_aden,h_clyde", got.Open)
				assert.Equal(t, "closed", got.Pane)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errs := web.ParseSubmissionForTest(tt.form, editable())
			tt.check(t, got, errs)
		})
	}
}
