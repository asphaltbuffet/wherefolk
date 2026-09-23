package web

import (
	"strings"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// treeNode is one row in the tree pane.
//
// Children is populated only when Open, so the rendered tree contains exactly
// what is visible. That keeps a deep family's first paint small and means the
// template needs no visibility logic of its own — if a node has children in the
// view model, they are on screen.
//
// HasChildren is therefore separate from len(Children): a collapsed node still
// needs a disclosure triangle, and the template cannot infer one from the other.
type treeNode struct {
	ID          rolo.HouseholdID
	Label       string
	Memorial    bool
	Selected    bool
	Open        bool
	HasChildren bool
	Children    []treeNode
}

// treeNodes builds the visible tree. selected is the Household the detail pane
// is showing, which may be empty; open is the set of Households whose children
// are expanded.
//
// It renders the open set it is given and does not compute one. Deciding what
// is open — in particular the rule that a selection's ancestors are always
// open, so the tree can never hide the Household the detail pane is showing —
// belongs to openSet and treeView. Duplicating that rule here would put it in
// two places that could drift, and would silently override treeView's
// deliberate ordering, which applies a close request before re-adding the
// selection's chain.
//
// Callers hold at least a read lock.
func (s *Server) treeNodes(selected rolo.HouseholdID, open map[rolo.HouseholdID]bool) []treeNode {
	var build func(households []rolo.Household) []treeNode

	build = func(households []rolo.Household) []treeNode {
		nodes := make([]treeNode, 0, len(households))

		for _, h := range households {
			children := s.tree.Children(h.ID)

			node := treeNode{
				ID:          h.ID,
				Label:       h.Label(),
				Memorial:    h.IsMemorial(),
				Selected:    h.ID == selected,
				Open:        open[h.ID],
				HasChildren: len(children) > 0,
			}

			if node.Open {
				node.Children = build(children)
			}

			nodes = append(nodes, node)
		}

		return nodes
	}

	return build(s.tree.Roots())
}

// selectionChain returns the Households that must be open for selected to be
// visible: itself and every ancestor. The selection itself is included so its
// children are visible — an Editor who has navigated to a Household is usually
// on their way further down.
//
// An unknown or empty selection yields nothing, which leaves the tree rendering
// normally beside whatever the caller puts in the detail pane.
//
// Callers hold at least a read lock.
func (s *Server) selectionChain(selected rolo.HouseholdID) []rolo.HouseholdID {
	if selected == "" {
		return nil
	}

	chain, err := s.tree.Path(selected)
	if err != nil {
		return nil
	}

	ids := make([]rolo.HouseholdID, 0, len(chain))
	for _, h := range chain {
		ids = append(ids, h.ID)
	}

	return ids
}

// openSet resolves which Households are expanded, given an explicit set and a
// selection.
//
// Every ancestor of the selection is always open, regardless of what the request
// asked for: a selected Household the Editor cannot see in the tree would break
// §4.3's promise that search relocates them *within* their mental model rather
// than bypassing it. The explicit set from the query string is layered under it,
// so collapsing a Branch the Editor is not standing in still works.
//
// See treeView for the variant that also honours a close request.
//
// Callers hold at least a read lock.
func (s *Server) openSet(selected rolo.HouseholdID, raw string) map[rolo.HouseholdID]bool {
	open := make(map[rolo.HouseholdID]bool)

	for _, id := range parseIDs(raw) {
		if _, ok := s.tree.Get(id); ok {
			open[id] = true
		}
	}

	for _, id := range s.selectionChain(selected) {
		open[id] = true
	}

	return open
}

// parseIDs splits a comma-separated list of Household IDs, discarding empties.
// It does no validation: openSet checks each against the tree, because an ID
// from a stale bookmark is an ordinary occurrence and not an error.
func parseIDs(raw string) []rolo.HouseholdID {
	if raw == "" {
		return nil
	}

	var ids []rolo.HouseholdID

	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		ids = append(ids, rolo.HouseholdID(part))
	}

	return ids
}

// joinIDsOrdered renders an open set as the comma-separated string the query
// string carries. The order is the tree's own depth-first order rather than map
// order, so the same expansion always produces the same URL and the Editor's
// history does not fill with URLs that differ only by shuffling.
func (s *Server) joinIDsOrdered(open map[rolo.HouseholdID]bool) string {
	var ids []string

	_ = s.tree.Walk(func(h rolo.Household, _ int) error {
		if open[h.ID] {
			ids = append(ids, string(h.ID))
		}
		return nil
	})

	return strings.Join(ids, ",")
}
