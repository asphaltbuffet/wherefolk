package web

import (
	"net/http"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// maxResults caps a result list. A query like "a" matches most of the family,
// and a list of three hundred names is not a shortcut — it is the tree again,
// unsorted and without its structure. Truncation is announced so the Editor
// knows to narrow the query rather than assuming they have seen everything.
const maxResults = 25

// resultView is one search hit. §4.3 fixes the shape: a name, and the Path that
// distinguishes it from the four other people with that name.
type resultView struct {
	Household rolo.HouseholdID
	Name      string
	Path      string
}

// resultsView is the result list fragment.
type resultsView struct {
	Query     string
	Results   []resultView
	Truncated bool
}

// handleSearch renders the result list for htmx to swap under the search box.
//
// Results are links, not htmx actions: selecting one is an ordinary navigation
// to /h/{id}, which is the same code path as clicking the tree. §4.3 requires
// that selecting a result expand the tree to that node rather than open a
// detached editor, and sharing the handler is what guarantees it rather than
// merely arranging for it.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")

	// htmx sets this header on every request it makes, which is what lets one
	// route serve both audiences. Without the branch, submitting the form with
	// JavaScript unavailable — or before htmx has loaded — navigates the browser
	// to a bare <ul> with no chrome, no stylesheet, and no way back: an
	// unstyled orphan page, which for this Editor is an error screen. Search has
	// no pushed URL, so wantsFragment's history-restore exclusion never applies
	// here — it just reads better than repeating the Hx-Request check inline.
	if !wantsFragment(r) {
		s.renderSearchPage(w, r, query)
		return
	}

	s.mu.RLock()
	view := s.searchResults(query)
	s.mu.RUnlock()

	err := s.renderFragment(r.Context(), w, "directory", "results", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render search results", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// renderSearchPage answers a plain browser navigation to /search with the whole
// two-pane page, results included, so the Editor keeps the tree and can carry on
// without going back. It is also what makes a search deep-linkable, which
// ADR-0008 claims of every view.
func (s *Server) renderSearchPage(w http.ResponseWriter, r *http.Request, query string) {
	q := r.URL.Query()

	s.mu.RLock()
	view := s.directoryView("", q.Get("open"), q.Get("close"), q.Get("pane") == paneClosed)
	view.Results = s.searchResults(query)
	s.mu.RUnlock()

	err := s.render(r.Context(), w, http.StatusOK, "directory", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render search page", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// searchResults runs the search and caps the list. Named for what it does rather
// than what it returns, so it does not collide with the resultsView type.
//
// Callers hold at least a read lock.
func (s *Server) searchResults(query string) resultsView {
	matches := s.tree.SearchPeople(query)

	view := resultsView{Query: query}

	if len(matches) > maxResults {
		view.Truncated = true
		matches = matches[:maxResults]
	}

	view.Results = make([]resultView, 0, len(matches))
	for _, m := range matches {
		view.Results = append(view.Results, resultView{
			Household: m.Household,
			Name:      m.Person.DisplayName(),
			Path:      m.Path,
		})
	}

	return view
}
