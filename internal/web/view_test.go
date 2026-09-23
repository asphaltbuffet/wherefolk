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
			name:   "a withheld email renders as [private], never as the value",
			id:     "h_clyde",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				require.Len(t, v.Adults, 2)
				assert.Equal(t, "[private]", v.Adults[1].Email)
				assert.NotContains(t, v.Adults[1].Email, "@",
					"the withheld address is replaced, not merely flagged alongside itself")
				assert.Equal(t, "555-0143", v.Adults[1].Phone, "only the marked field is withheld")
			},
		},
		{
			name:   "a withheld address renders as [private]",
			id:     "h_reeve",
			wantOK: true,
			checkFunc: func(t *testing.T, v web.HouseholdViewForTest) {
				t.Helper()
				assert.True(t, v.AddressPrivate)
				assert.Empty(t, v.AddressLines, "a withheld address does not leak its lines into the view model")
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

				// Aden carries a phone and an email in the fixture; §5.4 says
				// a Memorial Household publishes neither, and §5.5 says the
				// suppression is silent rather than marked.
				assert.Empty(t, v.Adults[0].Phone, "§5.4: no contact details")
				assert.Empty(t, v.Adults[0].Email, "§5.4: no contact details")
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

// TestAddressPrecedence pins the order of householdView's address switch.
// Hidden must be tested before SharesAddress: a Household that both withholds
// its address and references a parent's must render [private], never the
// back-reference. Reordering those two cases would leak the fact that the
// Household lives at its parent's address, which is itself the withheld
// information.
func TestAddressPrecedence(t *testing.T) {
	tests := []struct {
		name           string
		address        rolo.Address
		wantPrivate    bool
		wantSharedWith string
		wantLines      int
	}{
		{
			name:        "hidden beats shared",
			address:     rolo.Address{SharedWith: "h_root", Lines: []string{"1 Leak Ln"}, Hidden: true},
			wantPrivate: true,
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

			srv, err := web.New(doc, config.Config{}, testLogger(), web.Meta{})
			require.NoError(t, err)

			v, ok := srv.HouseholdViewForTest("h_kid")
			require.True(t, ok)

			assert.Equal(t, tt.wantPrivate, v.AddressPrivate)
			assert.Equal(t, tt.wantSharedWith, v.SharedWith)
			assert.Len(t, v.AddressLines, tt.wantLines)

			if tt.wantPrivate {
				assert.Empty(t, v.SharedWith, "a withheld address must not reveal whose it is")
				assert.Empty(t, v.AddressLines, "a withheld address must not carry its lines")
			}
		})
	}
}
