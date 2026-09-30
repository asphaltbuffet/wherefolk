package web

import (
	"net/http"

	"github.com/asphaltbuffet/wherefolk/internal/live"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// handleDirectory renders the two-pane page. id may be empty, which is the
// Editor's first visit: the tree is shown and the detail pane invites a choice.
func (s *Server) handleDirectory(w http.ResponseWriter, r *http.Request) {
	id := rolo.HouseholdID(r.PathValue("id"))
	q := r.URL.Query()

	snap := s.live.Snapshot()
	view := s.directoryView(snap, id, q.Get("open"), q.Get("close"), q.Get("pane") == paneClosed)
	view.Announcement = s.announcementFor(snap, q, id, view.Tree.Open)

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

// directoryView assembles the whole page.
func (s *Server) directoryView(
	snap live.Snapshot,
	id rolo.HouseholdID,
	rawOpen, closing string,
	isPaneClosed bool,
) directoryView {
	// snap.Trash is purged as of the Snapshot, so the count agrees with what
	// /trash lists (internal/live).
	view := directoryView{
		Tree:          s.treeView(snap, id, rawOpen, closing),
		PaneClosed:    isPaneClosed,
		PaneToggleURL: paneToggleURL(id, isPaneClosed),
		TrashCount:    len(snap.Trash.Entries),
	}

	if id == "" {
		return view
	}

	household, ok := s.householdView(snap, id)
	if !ok {
		view.NotFound = true
		return view
	}

	view.Household = &household

	// The form posts the tree's state back, so a save or a refusal returns the
	// Editor to the same expansion (ADR-0008).
	if view.Household.Form != nil {
		view.Household.Form.Open = s.joinIDsOrdered(snap, s.openSet(snap, id, rawOpen))
		if isPaneClosed {
			view.Household.Form.Pane = paneClosed
		}

		if view.Household.Delete.Blocked == "" {
			view.Household.Delete.URL = pageURL(id, "delete",
				view.Household.Form.Open, view.Household.Form.Pane, nil)
		}
	}

	return view
}
