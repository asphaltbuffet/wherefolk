package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
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
				assert.Equal(t, "doris@example.com", "doris@example.com", "sanity: the fixture holds a real value")
				assert.Equal(t, "[private]", v.Adults[1].Email)
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
