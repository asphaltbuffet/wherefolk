package web

import (
	"net/http"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// handleDirectory renders the two-pane page. id may be empty, which is the
// Editor's first visit: the tree is shown and the detail pane invites a choice.
func (s *Server) handleDirectory(w http.ResponseWriter, r *http.Request) {
	id := rolo.HouseholdID(r.PathValue("id"))
	q := r.URL.Query()

	s.mu.RLock()
	view := s.directoryView(id, q.Get("open"), q.Get("close"), q.Get("pane") == paneClosed)
	s.mu.RUnlock()

	// A selection that is not in the document is the Editor following a stale
	// bookmark or a link from before a restructure. It is a 404 for anything
	// automated, but for the Editor it is a sentence and a navigable tree —
	// never a code and never a stack trace.
	status := http.StatusOK
	if view.NotFound {
		status = http.StatusNotFound
	}

	err := s.render(r.Context(), w, status, "directory", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render directory", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// directoryView assembles the whole page. Callers hold at least a read lock.
func (s *Server) directoryView(id rolo.HouseholdID, rawOpen, closing string, paneClosed bool) directoryView {
	view := directoryView{
		Tree:          s.treeView(id, rawOpen, closing),
		PaneClosed:    paneClosed,
		PaneToggleURL: paneToggleURL(id, paneClosed),
	}

	if id == "" {
		return view
	}

	household, ok := s.householdView(id)
	if !ok {
		view.NotFound = true
		return view
	}

	view.Household = &household

	return view
}
