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
