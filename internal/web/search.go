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
	query := r.URL.Query().Get("q")

	s.mu.RLock()
	view := s.resultsView(query)
	s.mu.RUnlock()

	err := s.renderFragment(r.Context(), w, "directory", "results", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render search results", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// resultsView runs the search and caps the list. Callers hold at least a read lock.
func (s *Server) resultsView(query string) resultsView {
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
