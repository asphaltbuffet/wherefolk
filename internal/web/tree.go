package web

import (
	"net/http"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// treeView is what _tree.html renders. It carries the selection alongside the
// nodes because the template writes the selection into every node's link, so
// that clicking a sibling preserves the expansion the Editor has built up.
type treeView struct {
	Nodes    []treeNode
	Selected rolo.HouseholdID
	// Open is the canonical comma-separated open set, which every link and
	// toggle in the fragment round-trips back to the server.
	Open string
}

// handleTree renders the tree pane on its own, for htmx to swap in after an
// expand or collapse. The full page renders the same fragment inline on first
// load, so the two can never disagree about how a node looks.
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	selected := rolo.HouseholdID(r.URL.Query().Get("selected"))

	s.mu.RLock()
	view := s.treeView(selected, r.URL.Query().Get("open"), r.URL.Query().Get("close"))
	s.mu.RUnlock()

	err := s.renderFragment(r.Context(), w, "directory", "tree", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render tree", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// treeView assembles the tree pane's view model.
//
// closing, if set, is removed from the open set: collapsing is expressed as
// "everything that was open, minus this" so that a toggle link is a plain URL
// and the browser's back button retraces expansions exactly.
//
// The selection's ancestors are re-added last, after the close request is
// applied, so no close parameter can hide the Household the detail pane is
// showing — that would break §4.3's promise that navigation stays inside the
// Editor's mental model.
//
// Callers hold at least a read lock.
func (s *Server) treeView(selected rolo.HouseholdID, rawOpen, closing string) treeView {
	open := make(map[rolo.HouseholdID]bool)

	for _, id := range parseIDs(rawOpen) {
		if _, ok := s.tree.Get(id); ok {
			open[id] = true
		}
	}

	for _, id := range parseIDs(closing) {
		delete(open, id)
	}

	for _, id := range s.selectionChain(selected) {
		open[id] = true
	}

	return treeView{
		Nodes:    s.treeNodes(selected, open),
		Selected: selected,
		Open:     s.joinIDsOrdered(open),
	}
}
