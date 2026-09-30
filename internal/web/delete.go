package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherefolk/internal/live"
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
// tree.
func (s *Server) deleteView(snap live.Snapshot, id rolo.HouseholdID, open, pane string) (deleteView, bool) {
	h, ok := snap.Tree.Get(id)
	if !ok {
		return deleteView{}, false
	}

	path, err := snap.Tree.PathString(id)
	if err != nil {
		return deleteView{}, false
	}

	block, err := snap.Tree.DeleteBlock(id)
	if err != nil {
		return deleteView{}, false
	}

	view := deleteView{
		ID:        id,
		Title:     h.Name(),
		Path:      path,
		KeptUntil: keptUntil(snap.Now),
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

	view, ok := s.deleteView(s.live.Snapshot(), id, q.Get("open"), q.Get("pane"))

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
		view, ok := s.deleteView(s.live.Snapshot(), id, open, pane)

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

// deleteHousehold moves id to the Trash and renders nothing. store.Delete
// refuses a Household something depends on; the confirmation page has
// already said why.
func (s *Server) deleteHousehold(id rolo.HouseholdID) deleteOutcome {
	var out deleteOutcome

	saved, err := s.live.Update(func(snap live.Snapshot) (store.State, error) {
		h, ok := snap.Tree.Get(id)
		if !ok {
			out.notFound = true
			return store.State{}, errRefused
		}

		path, err := snap.Tree.PathString(id)
		if err != nil {
			return store.State{}, err
		}

		doc, trash, err := store.Delete(snap.Document, snap.State().Trash,
			store.TrashEntry{DeletedAt: snap.Now, Path: path, Household: h})
		if err != nil {
			return store.State{}, err
		}

		out.name, out.parent, out.at = h.Name(), h.Parent, snap.Now

		return store.State{Document: doc, Trash: trash}, nil
	})

	switch {
	case errors.Is(err, errRefused):
	case errors.Is(err, store.ErrBlocked):
		out.blocked = true
	case err != nil:
		out.err = err
	default:
		out.undo = saved.Undo
	}

	return out
}
