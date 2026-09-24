package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// findHousehold returns a Household from a document by ID.
func findHousehold(t *testing.T, doc *store.Document, id rolo.HouseholdID) rolo.Household {
	t.Helper()

	for _, h := range doc.Households {
		if h.ID == id {
			return h
		}
	}

	t.Fatalf("household %q not in document", id)

	return rolo.Household{}
}

func TestCloneDocumentIsDeep(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(clone *store.Document)
		check  func(t *testing.T, original *store.Document)
	}{
		{
			name: "mutating an adult does not touch the original",
			mutate: func(clone *store.Document) {
				clone.Households[1].Adults[0].Phone = "CHANGED"
			},
			check: func(t *testing.T, original *store.Document) {
				t.Helper()
				assert.Equal(t, "555-0142", original.Households[1].Adults[0].Phone)
			},
		},
		{
			name: "mutating address lines does not touch the original",
			mutate: func(clone *store.Document) {
				clone.Households[1].Address.Lines[0] = "CHANGED"
			},
			check: func(t *testing.T, original *store.Document) {
				t.Helper()
				assert.Equal(t, "1412 Oak St", original.Households[1].Address.Lines[0])
			},
		},
		{
			name: "appending a household does not touch the original",
			mutate: func(clone *store.Document) {
				clone.Households = append(clone.Households, rolo.Household{ID: "h_extra"})
			},
			check: func(t *testing.T, original *store.Document) {
				t.Helper()
				assert.Len(t, original.Households, 4)
			},
		},
		{
			name: "appending a dependent does not touch the original",
			mutate: func(clone *store.Document) {
				clone.Households[1].Dependents = append(clone.Households[1].Dependents,
					rolo.Person{ID: "p_extra"})
			},
			check: func(t *testing.T, original *store.Document) {
				t.Helper()
				assert.Len(t, original.Households[1].Dependents, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := sampleDocument()

			clone := web.CloneDocumentForTest(original)
			tt.mutate(clone)

			tt.check(t, original)
		})
	}
}

func TestApplySubmission(t *testing.T) {
	tests := []struct {
		name  string
		id    rolo.HouseholdID
		form  map[string][]string
		check func(t *testing.T, doc *store.Document, changes []web.ChangeForTest, err error)
	}{
		{
			name: "a field edit lands on the right person",
			id:   "h_clyde",
			form: map[string][]string{
				"person.p_clyd01.given":   {"Clyde"},
				"person.p_clyd01.surname": {"Whitlock"},
				"person.p_clyd01.phone":   {"555-9999"},
				"person.p_dori01.given":   {"Doris"},
				"person.p_dori01.surname": {"Whitlock"},
				"person.p_carl01.given":   {"Carl"},
				"person.p_carl01.surname": {"Whitlock"},
			},
			check: func(t *testing.T, doc *store.Document, changes []web.ChangeForTest, err error) {
				t.Helper()

				require.NoError(t, err)
				assert.Empty(t, changes, "a field edit is not a structural change")

				h := findHousehold(t, doc, "h_clyde")
				assert.Equal(t, "555-9999", h.Adults[0].Phone)
				assert.Equal(t, "Doris", h.Adults[1].Given)
			},
		},
		{
			name: "a withheld flag is recorded without touching the value",
			id:   "h_clyde",
			form: map[string][]string{
				"person.p_clyd01.given":        {"Clyde"},
				"person.p_clyd01.phone":        {"555-0142"},
				"person.p_clyd01.hidden.phone": {"off", "on"},
			},
			check: func(t *testing.T, doc *store.Document, _ []web.ChangeForTest, err error) {
				t.Helper()

				require.NoError(t, err)

				h := findHousehold(t, doc, "h_clyde")
				assert.True(t, h.Adults[0].Hidden.Phone)
				assert.Equal(t, "555-0142", h.Adults[0].Phone,
					"withholding marks a value for export; it never clears it")
			},
		},
		{
			name: "a person absent from the form keeps their stored values",
			id:   "h_clyde",
			form: map[string][]string{
				"person.p_clyd01.given": {"Clyde"},
			},
			check: func(t *testing.T, doc *store.Document, _ []web.ChangeForTest, err error) {
				t.Helper()

				require.NoError(t, err)

				h := findHousehold(t, doc, "h_clyde")
				require.Len(t, h.Adults, 2)
				assert.Equal(t, "Doris", h.Adults[1].Given, "a stale form cannot blank someone")
				assert.Equal(t, "doris@example.com", h.Adults[1].Email)
			},
		},
		{
			name: "a new adult gets a minted id and is announced",
			id:   "h_dave",
			form: map[string][]string{
				"person.p_dave01.given": {"Dave"},
				"person.new1.given":     {"Diane"},
				"person.new1.surname":   {"Whitlock"},
			},
			check: func(t *testing.T, doc *store.Document, changes []web.ChangeForTest, err error) {
				t.Helper()

				require.NoError(t, err)

				h := findHousehold(t, doc, "h_dave")
				require.Len(t, h.Adults, 2)
				assert.Equal(t, rolo.PersonID("p_new001"), h.Adults[1].ID)
				assert.Equal(t, "Diane", h.Adults[1].Given)

				require.Len(t, changes, 1)
				assert.Contains(t, changes[0].Message, "Diane")
			},
		},
		{
			name: "a third adult is refused",
			id:   "h_clyde",
			form: map[string][]string{
				"person.p_clyd01.given": {"Clyde"},
				"person.p_dori01.given": {"Doris"},
				"person.new1.given":     {"Interloper"},
			},
			check: func(t *testing.T, _ *store.Document, _ []web.ChangeForTest, err error) {
				t.Helper()

				require.Error(t, err, "a Household holds one or two adults")
			},
		},
		{
			name: "promoting a dependent creates a household and moves the person",
			id:   "h_clyde",
			form: map[string][]string{
				"person.p_clyd01.given":   {"Clyde"},
				"person.p_dori01.given":   {"Doris"},
				"person.p_carl01.given":   {"Carl"},
				"person.p_carl01.surname": {"Whitlock"},
				"person.p_carl01.promote": {"on"},
			},
			check: func(t *testing.T, doc *store.Document, changes []web.ChangeForTest, err error) {
				t.Helper()

				require.NoError(t, err)

				parent := findHousehold(t, doc, "h_clyde")
				assert.Empty(t, parent.Dependents, "Carl left his parents' block")

				promoted := findHousehold(t, doc, "h_new001")
				assert.Equal(t, rolo.HouseholdID("h_clyde"), promoted.Parent)
				require.Len(t, promoted.Adults, 1)
				assert.Equal(t, rolo.PersonID("p_carl01"), promoted.Adults[0].ID,
					"a PersonID is stable and survives the move")
				assert.Equal(t, "Carl", promoted.Adults[0].Given)

				require.Len(t, changes, 1)
				assert.Equal(t, rolo.HouseholdID("h_new001"), changes[0].Household)
				assert.Contains(t, changes[0].Message, "Carl")
			},
		},
		{
			name: "removing a dependent drops them and announces it",
			id:   "h_clyde",
			form: map[string][]string{
				"person.p_clyd01.given":  {"Clyde"},
				"person.p_dori01.given":  {"Doris"},
				"person.p_carl01.given":  {"Carl"},
				"person.p_carl01.remove": {"on"},
			},
			check: func(t *testing.T, doc *store.Document, changes []web.ChangeForTest, err error) {
				t.Helper()

				require.NoError(t, err)

				h := findHousehold(t, doc, "h_clyde")
				assert.Empty(t, h.Dependents)

				require.Len(t, changes, 1)
				assert.Contains(t, changes[0].Message, "Carl")
			},
		},
		{
			name: "removing the last adult is refused",
			id:   "h_reeve",
			form: map[string][]string{
				"person.p_reev01.given":  {"Ray"},
				"person.p_reev01.remove": {"on"},
			},
			check: func(t *testing.T, _ *store.Document, _ []web.ChangeForTest, err error) {
				t.Helper()

				require.Error(t, err,
					"a Household with no adults is malformed and the store rejects it at load")
			},
		},
		{
			name: "clearing the address leaves the household standing",
			id:   "h_reeve",
			form: map[string][]string{
				"person.p_reev01.given": {"Ray"},
				"address.lines":         {""},
			},
			check: func(t *testing.T, doc *store.Document, changes []web.ChangeForTest, err error) {
				t.Helper()

				require.NoError(t, err)

				h := findHousehold(t, doc, "h_reeve")
				assert.Empty(t, h.Address.Lines)
				assert.Empty(t, changes, "ADR-0009: promotion is one-way; nothing demotes")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, sampleDocument(), nil)
			doc := web.CloneDocumentForTest(sampleDocument())

			target := findHousehold(t, doc, tt.id)
			sub, errs := web.ParseSubmissionForTest(tt.form, target)
			require.Empty(t, errs)

			changes, err := srv.ApplySubmissionForTest(doc, tt.id, sub)
			tt.check(t, doc, changes, err)
		})
	}
}
