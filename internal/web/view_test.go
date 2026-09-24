package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func newTestServer(t *testing.T) *web.Server {
	t.Helper()

	srv, err := web.New(
		sampleDocument(),
		config.Config{},
		testLogger(),
		web.Meta{DocumentPath: "/tmp/test/directory.json"},
		func(*store.Document) error { return nil },
		func() (rolo.HouseholdID, error) { return "h_test01", nil },
		func() (rolo.PersonID, error) { return "p_test01", nil },
	)
	require.NoError(t, err)

	return srv
}

func TestTreeNodes(t *testing.T) {
	tests := []struct {
		name      string
		selected  rolo.HouseholdID
		open      []rolo.HouseholdID
		checkFunc func(t *testing.T, nodes []web.TreeNodeForTest)
	}{
		{
			name:     "nothing selected renders roots collapsed",
			selected: "",
			open:     nil,
			checkFunc: func(t *testing.T, nodes []web.TreeNodeForTest) {
				t.Helper()
				require.Len(t, nodes, 2, "h_aden and h_reeve are the roots")
				for _, n := range nodes {
					assert.False(t, n.Open, "%s should be collapsed", n.ID)
					assert.Empty(t, n.Children, "a collapsed node renders no children")
				}
			},
		},
		{
			name:     "a root with children advertises them even when collapsed",
			selected: "",
			open:     nil,
			checkFunc: func(t *testing.T, nodes []web.TreeNodeForTest) {
				t.Helper()
				byID := indexNodes(nodes)
				assert.True(t, byID["h_aden"].HasChildren, "h_clyde is beneath h_aden")
				assert.False(t, byID["h_reeve"].HasChildren, "h_reeve is a standalone root")
			},
		},
		{
			name:     "selecting a node opens its ancestors and marks it selected",
			selected: "h_clyde",
			open:     nil,
			checkFunc: func(t *testing.T, nodes []web.TreeNodeForTest) {
				t.Helper()
				byID := indexNodes(nodes)
				assert.True(t, byID["h_aden"].Open, "an ancestor of the selection is open")
				require.Len(t, byID["h_aden"].Children, 1)
				assert.True(t, byID["h_clyde"].Selected)
				assert.False(t, byID["h_reeve"].Open, "an unrelated root stays collapsed")
			},
		},
		{
			name:     "an explicitly opened node renders its children",
			selected: "",
			open:     []rolo.HouseholdID{"h_aden"},
			checkFunc: func(t *testing.T, nodes []web.TreeNodeForTest) {
				t.Helper()
				byID := indexNodes(nodes)
				assert.True(t, byID["h_aden"].Open)
				require.Len(t, byID["h_aden"].Children, 1)
				assert.Equal(t, rolo.HouseholdID("h_clyde"), byID["h_aden"].Children[0].ID)
			},
		},
		{
			name:     "labels come from the Household, in sibling order",
			selected: "",
			open:     nil,
			checkFunc: func(t *testing.T, nodes []web.TreeNodeForTest) {
				t.Helper()
				require.Len(t, nodes, 2)
				assert.Equal(t, "Aden/Nettie", nodes[0].Label, "Aden b. 1910 precedes Ray b. 1942")
				assert.Equal(t, "Ray", nodes[1].Label)
			},
		},
	}

	srv := newTestServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			open := make(map[rolo.HouseholdID]bool, len(tt.open))
			for _, id := range tt.open {
				open[id] = true
			}

			tt.checkFunc(t, srv.TreeNodesForTest(tt.selected, open))
		})
	}
}

// indexNodes flattens a node tree into a lookup by ID.
func indexNodes(nodes []web.TreeNodeForTest) map[rolo.HouseholdID]web.TreeNodeForTest {
	out := make(map[rolo.HouseholdID]web.TreeNodeForTest)

	var visit func(ns []web.TreeNodeForTest)
	visit = func(ns []web.TreeNodeForTest) {
		for _, n := range ns {
			out[n.ID] = n
			visit(n.Children)
		}
	}
	visit(nodes)

	return out
}

func TestHouseholdView(t *testing.T) {
	tests := []struct {
		name      string
		id        rolo.HouseholdID
		wantOK    bool
		checkFunc func(t *testing.T, v web.HouseholdViewForTest)
	}{
		{
			name:   "an unknown id is not found",
			id:     "h_ghost",
			wantOK: false,
		},
		{
			name:   "the breadcrumb is the whole Path, root first",
			id:     "h_dave",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				require.Len(t, v.Crumbs, 3)
				assert.Equal(t, "Aden/Nettie", v.Crumbs[0].Label)
				assert.Equal(t, "Clyde/Doris", v.Crumbs[1].Label)
				assert.Equal(t, "Dave", v.Crumbs[2].Label)
				assert.Equal(t, rolo.HouseholdID("h_aden"), v.Crumbs[0].ID,
					"every crumb is navigable, so the Editor can walk back up")
			},
		},
		{
			name:   "the title names the adults in full",
			id:     "h_clyde",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				assert.Equal(t, `Clyde Whitlock & Doris "Dot" Whitlock`, v.Title)
			},
		},
		{
			name:   "a withheld email still shows its value; hiding is an export concern",
			id:     "h_clyde",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()

				// ADR-0010: the editing UI never masks. Doris's email is
				// marked hidden in the fixture, which affects export only.
				assert.Equal(t, "doris@example.com", v.Adults[1].Email)
				assert.Equal(t, "555-0143", v.Adults[1].Phone)
			},
		},
		{
			name:   "a withheld address still shows its lines",
			id:     "h_reeve",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()

				// ADR-0010: the marker belongs to export, not to this pane.
				// AddressPrivate still reflects the fixture's Hidden flag —
				// it just no longer suppresses the lines below it.
				assert.True(t, v.AddressPrivate)
				assert.Equal(t, []string{"9 Elm St"}, v.AddressLines)
			},
		},
		{
			name:   "a shared address renders as a back-reference, not a copy",
			id:     "h_dave",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				assert.Equal(t, "Clyde/Doris", v.SharedWith)
				assert.Empty(t, v.AddressLines, "§3: a Shared Address is a reference, not a copy")
			},
		},
		{
			name:   "a memorial household is flagged and has no contact details",
			id:     "h_aden",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				assert.True(t, v.Memorial)
				require.Len(t, v.Adults, 2)
				assert.Equal(t, "1910-04-02", v.Adults[0].Birth)
				assert.Equal(t, "1989-11-17", v.Adults[0].Death)
				assert.True(t, v.Adults[0].Deceased)

				// ADR-0010: suppression is export-only. The Editor must be
				// able to see and clear a deceased person's recorded details.
				assert.Equal(t, "555-0100", v.Adults[0].Phone)
				assert.Equal(t, "aden@example.com", v.Adults[0].Email)
			},
		},
		{
			name:   "a deceased Dependent keeps both dates",
			id:     "h_clyde",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				require.Len(t, v.Dependents, 1)
				assert.Equal(t, "Carl Whitlock", v.Dependents[0].Name)
				assert.Equal(t, "1981", v.Dependents[0].Death)
			},
		},
		{
			name:   "the anniversary renders when present",
			id:     "h_clyde",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				assert.Equal(t, "1962-06-14", v.Anniversary)
			},
		},
		{
			name:   "a household with no anniversary renders none",
			id:     "h_reeve",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				assert.Empty(t, v.Anniversary)
			},
		},
	}

	srv := newTestServer(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := srv.HouseholdViewForTest(tt.id)

			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}

			tt.checkFunc(t, got)
		})
	}
}

// TestTreeMarksMemorialHouseholds covers the one treeNode field nothing else
// asserts. Memorial is the tree's only signal that a Branch's heads have died,
// and it drives a style rather than text, so without this row the marker could
// disappear silently.
//
// This asserts on a class name, which the project otherwise avoids. The
// exception is deliberate: the fact under test — "the tree distinguishes a
// Memorial Household" — has no textual form to assert on, since the label is
// the same either way. The class IS the observable behaviour here.
func TestTreeMarksMemorialHouseholds(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		checkFunc func(t *testing.T, body string)
	}{
		{
			name:   "a Memorial Household is marked in the tree",
			target: "/tree",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "tree-memorial",
					"h_aden's adults are both deceased, so the tree must distinguish it")
			},
		},
		{
			name:   "a Household with a living adult is not marked",
			target: "/tree?open=h_aden",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				// h_clyde is revealed by the open set and has living adults.
				require.Contains(t, body, "Clyde/Doris", "precondition: the child is visible")
				assert.Equal(t, 1, strings.Count(body, "tree-memorial"),
					"only h_aden is a Memorial Household, so only one node carries the marker")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, http.StatusOK, rec.Code)
			tt.checkFunc(t, rec.Body.String())
		})
	}
}

// TestAddressPrecedence pins how householdView populates the address fields.
// ADR-0010 removed masking from this pane, so AddressLines and SharedWith are
// independent: a Household can carry both a Hidden flag and a Shared Address,
// and the view renders lines and back-reference together rather than picking
// one over the other.
func TestAddressPrecedence(t *testing.T) {
	tests := []struct {
		name           string
		address        rolo.Address
		wantSharedWith string
		wantLines      int
	}{
		{
			name:           "hidden no longer suppresses lines or the back-reference",
			address:        rolo.Address{SharedWith: "h_root", Lines: []string{"1 Leak Ln"}, Hidden: true},
			wantSharedWith: "Root",
			wantLines:      1,
		},
		{
			name:           "shared without hidden renders the back-reference",
			address:        rolo.Address{SharedWith: "h_root"},
			wantSharedWith: "Root",
		},
		{
			name:      "plain address renders its lines",
			address:   rolo.Address{Lines: []string{"1 Plain St"}},
			wantLines: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &store.Document{Schema: store.CurrentSchema, Households: []rolo.Household{
				{ID: "h_root", Adults: []rolo.Person{{ID: "p_root", Given: "Root", Birth: rolo.Date{Year: 1930}}}},
				{
					ID: "h_kid", Parent: "h_root", Address: tt.address,
					Adults: []rolo.Person{{ID: "p_kid", Given: "Kid", Birth: rolo.Date{Year: 1960}}},
				},
			}}

			srv, err := web.New(doc, config.Config{}, testLogger(), web.Meta{},
				func(*store.Document) error { return nil },
				func() (rolo.HouseholdID, error) { return "h_test01", nil },
				func() (rolo.PersonID, error) { return "p_test01", nil },
			)
			require.NoError(t, err)

			v, ok := srv.HouseholdViewForTest("h_kid")
			require.True(t, ok)

			assert.Equal(t, tt.wantSharedWith, v.SharedWith)
			assert.Len(t, v.AddressLines, tt.wantLines)
		})
	}
}

// TestDeceasedContactSuppressionIsPerPerson pins the boundary of §5.4's
// no-contact-details rule. The rule is per-Person, not per-Household: §5.4
// justifies it by there being nobody left to own the details, which does not
// extend to a living Dependent still listed inside a Memorial Household. An
// earlier fix gated on IsMemorial() and hid a living minor's phone number,
// which is the opposite failure from the leak it was fixing.
func TestDeceasedContactSuppressionIsPerPerson(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, v web.HouseholdViewForTest)
	}{
		{
			name: "a deceased adult's details are suppressed",
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				require.Len(t, v.Adults, 1)
				assert.Equal(t, "555-DEAD", v.Adults[0].Phone)
				assert.Equal(t, "dead@example.com", v.Adults[0].Email)
			},
		},
		{
			name: "a living Dependent keeps theirs, even in a Memorial Household",
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				require.Len(t, v.Dependents, 2)
				assert.Equal(t, "555-LIVING", v.Dependents[0].Phone,
					"a living relative's number is exactly what the Editor needs")
				assert.Equal(t, "living@example.com", v.Dependents[0].Email)
			},
		},
		{
			name: "a deceased Dependent's details are suppressed",
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				require.Len(t, v.Dependents, 2)
				assert.Equal(t, "555-GONEDEP", v.Dependents[1].Phone)
				assert.Equal(t, "gonedep@example.com", v.Dependents[1].Email)
			},
		},
		{
			name: "the Household is still Memorial",
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				assert.True(t, v.Memorial, "every adult is deceased")
			},
		},
	}

	doc := &store.Document{Schema: store.CurrentSchema, Households: []rolo.Household{{
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
	}}}

	srv, err := web.New(doc, config.Config{}, testLogger(), web.Meta{},
		func(*store.Document) error { return nil },
		func() (rolo.HouseholdID, error) { return "h_test01", nil },
		func() (rolo.PersonID, error) { return "p_test01", nil },
	)
	require.NoError(t, err)

	v, ok := srv.HouseholdViewForTest("h_mem")
	require.True(t, ok)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, v)
		})
	}
}
