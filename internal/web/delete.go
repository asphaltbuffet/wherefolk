package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// deleteLink is the detail pane's way to deletion. Exactly one of URL and
// Blocked is set: a Household that cannot be deleted says why in place of the
// link, rather than offering a page that can only refuse.
type deleteLink struct {
	URL     string
	Blocked string
}

// deleteView is the confirmation page. ADR-0009 makes deletion "a deliberate
// act with its own confirmation", so it is a page of its own rather than a
// checkbox inside the form that Save submits.
type deleteView struct {
	ID        rolo.HouseholdID
	Title     string
	Path      string
	People    []string
	KeptUntil string
	// Blocked explains why this Household cannot be deleted; when set the page
	// offers no Delete button.
	Blocked string
	Open    string
	Pane    string
	// Back returns to the Household with the tree as the Editor left it.
	Back string
}

// deleteOutcome is what deleteHousehold decided, for handleDelete to turn into
// a response without holding the lock.
type deleteOutcome struct {
	name     string
	parent   rolo.HouseholdID
	undo     string
	at       time.Time
	notFound bool
	blocked  bool
	err      error
}

// keptUntil is the date a Household deleted at now leaves the Trash, at the
// earliest, phrased for the Editor.
func keptUntil(now time.Time) string {
	return now.Add(store.TrashRetention).Format("January 2")
}

// deleteBlockedSentence explains a DeleteBlock to the Editor, naming what must
// change first. It is empty when nothing blocks the deletion.
func deleteBlockedSentence(b rolo.DeleteBlock) string {
	switch {
	case b.Memorial:
		return "A memorial household is kept permanently, so it cannot be deleted."
	case len(b.Children) > 0:
		return "To delete this household, first delete the households beneath it: " +
			joinNames(b.Children) + "."
	case len(b.Sharers) > 0:
		return "This household's address is used by " + joinNames(b.Sharers) +
			", so it cannot be deleted."
	default:
		return ""
	}
}

// joinNames lists Households by Household Name: "A", "A and B", "A, B and C".
func joinNames(hs []rolo.Household) string {
	names := make([]string, 0, len(hs))
	for _, h := range hs {
		names = append(names, h.Name())
	}

	if len(names) == 1 {
		return names[0]
	}

	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// deleteView builds the confirmation page, reporting false if id is not in the
// tree. Callers hold at least a read lock.
func (s *Server) deleteView(id rolo.HouseholdID, open, pane string) (deleteView, bool) {
	h, ok := s.tree.Get(id)
	if !ok {
		return deleteView{}, false
	}

	path, err := s.tree.PathString(id)
	if err != nil {
		return deleteView{}, false
	}

	block, err := s.tree.DeleteBlock(id)
	if err != nil {
		return deleteView{}, false
	}

	view := deleteView{
		ID:        id,
		Title:     h.Name(),
		Path:      path,
		KeptUntil: keptUntil(s.now()),
		Blocked:   deleteBlockedSentence(block),
		Open:      open,
		Pane:      pane,
		Back:      pageURL(id, "", open, pane, nil),
	}

	for _, p := range h.Adults {
		view.People = append(view.People, p.DisplayName())
	}

	for _, p := range h.Dependents {
		view.People = append(view.People, p.DisplayName())
	}

	return view, true
}

// handleDeleteConfirm asks before deleting.
func (s *Server) handleDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	id := rolo.HouseholdID(r.PathValue("id"))
	q := r.URL.Query()

	s.mu.RLock()
	view, ok := s.deleteView(id, q.Get("open"), q.Get("pane"))
	s.mu.RUnlock()

	if !ok {
		s.renderNotFound(w, r)
		return
	}

	err := s.render(r.Context(), w, http.StatusOK, "delete", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render delete", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// handleDelete moves a Household to the Trash and lands on its parent, whose
// page announces the deletion beside an Undo.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := rolo.HouseholdID(r.PathValue("id"))

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "could not read the form", http.StatusBadRequest)
		return
	}

	open, pane := r.PostForm.Get("open"), r.PostForm.Get("pane")
	outcome := s.deleteHousehold(id)

	switch {
	case outcome.notFound:
		s.renderNotFound(w, r)

	case outcome.err != nil:
		s.log.ErrorContext(r.Context(), "delete household", "household", id, "error", outcome.err)
		http.Error(w, "the household could not be deleted", http.StatusInternalServerError)

	case outcome.blocked:
		// Reachable only from a stale page: the pane offers no link when
		// deletion is blocked. The confirmation page carries the reason.
		s.mu.RLock()
		view, ok := s.deleteView(id, open, pane)
		s.mu.RUnlock()

		if !ok {
			s.renderNotFound(w, r)
			return
		}

		err = s.render(r.Context(), w, http.StatusConflict, "delete", view)
		if err != nil {
			s.log.ErrorContext(r.Context(), "render blocked delete", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}

	default:
		q := url.Values{}
		q.Set(saidKey, fmt.Sprintf("%s was deleted. It is kept in Recently deleted until %s.",
			outcome.name, keptUntil(outcome.at)))
		q.Set(undoKey, outcome.undo)

		http.Redirect(w, r, pageURL(outcome.parent, "", open, pane, q), http.StatusSeeOther)
	}
}

// deleteHousehold moves id to the Trash under the write lock and renders
// nothing.
func (s *Server) deleteHousehold(id rolo.HouseholdID) deleteOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, ok := s.tree.Get(id)
	if !ok {
		return deleteOutcome{notFound: true}
	}

	block, err := s.tree.DeleteBlock(id)
	if err != nil {
		return deleteOutcome{err: err}
	}

	if block.Blocked() {
		return deleteOutcome{blocked: true}
	}

	path, err := s.tree.PathString(id)
	if err != nil {
		return deleteOutcome{err: err}
	}

	now := s.now()

	doc, trash, err := store.Delete(s.doc, s.trash, store.TrashEntry{DeletedAt: now, Path: path, Household: h})
	if err != nil {
		return deleteOutcome{err: err}
	}

	// Purging on every Trash write keeps expiry free of a timer (CONTEXT.md, Trash).
	trash, _ = trash.Purge(now)

	token, err := s.persist(store.State{Document: doc, Trash: trash, Settings: s.settings})
	if err != nil {
		return deleteOutcome{err: err}
	}

	return deleteOutcome{name: h.Name(), parent: h.Parent, undo: token, at: now}
}
