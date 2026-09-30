package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// trashView is the Recently deleted page. The Editor never sees the word
// "Trash" (CONTEXT.md).
type trashView struct {
	Entries []trashEntryView
	// Notice explains a restore that could not happen.
	Notice string
}

type trashEntryView struct {
	ID        rolo.HouseholdID
	Name      string
	Path      string
	Deleted   string
	KeptUntil string
}

// restoreOutcome is what restoreHousehold decided.
type restoreOutcome struct {
	name    string
	message string
	undo    string
	err     error
}

// trashView lists the Trash newest first, since the deletion the Editor is
// looking for is most often the one they just made. Callers hold at least a
// read lock.
func (s *Server) trashView(notice string) trashView {
	// Purge, not the stored Trash: an expired, unanchored entry is still
	// restorable until the next Trash write, but showing it here — with a
	// kept-until date already past — would be dishonest. This is a read; it
	// writes nothing.
	trash, _ := s.trash.Purge(s.now())

	entries := slices.Clone(trash.Entries)
	slices.SortStableFunc(entries, func(a, b store.TrashEntry) int {
		return b.DeletedAt.Compare(a.DeletedAt)
	})

	view := trashView{Notice: notice}

	for _, e := range entries {
		view.Entries = append(view.Entries, trashEntryView{
			ID:        e.Household.ID,
			Name:      e.Household.Name(),
			Path:      e.Path,
			Deleted:   e.DeletedAt.Format("January 2, 2006"),
			KeptUntil: trash.KeptUntil(e.Household.ID).Format("January 2"),
		})
	}

	return view
}

func (s *Server) renderTrash(w http.ResponseWriter, r *http.Request, status int, notice string) {
	s.mu.RLock()
	view := s.trashView(notice)
	s.mu.RUnlock()

	err := s.render(r.Context(), w, status, "trash", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render trash", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) handleTrash(w http.ResponseWriter, r *http.Request) {
	s.renderTrash(w, r, http.StatusOK, "")
}

// handleRestore brings a Household back and lands on it, announcing the
// restore beside an Undo.
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	id := rolo.HouseholdID(r.PathValue("id"))
	outcome := s.restoreHousehold(id)

	switch {
	case errors.Is(outcome.err, store.ErrNotInTrash):
		s.renderTrash(w, r, http.StatusNotFound, "That household is no longer in Recently deleted.")

	case errors.Is(outcome.err, store.ErrUnrestorable):
		s.renderTrash(w, r, http.StatusConflict, fmt.Sprintf(
			"%s cannot be restored, because a household it belongs with is no longer anywhere in the directory.",
			outcome.name))

	case outcome.err != nil:
		s.log.ErrorContext(r.Context(), "restore household", "household", id, "error", outcome.err)
		http.Error(w, "the household could not be restored", http.StatusInternalServerError)

	default:
		q := url.Values{}
		q.Set(saidKey, outcome.message)
		q.Set(undoKey, outcome.undo)

		// No open set to carry: the Trash page has no tree. The selection
		// chain opens the restored Household's ancestors by itself.
		http.Redirect(w, r, pageURL(id, "", "", "", q), http.StatusSeeOther)
	}
}

// restoreHousehold moves id, and whatever it needs, back into the Directory
// under the write lock.
func (s *Server) restoreHousehold(id rolo.HouseholdID) restoreOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()

	var name string
	if e, ok := s.trash.Entry(id); ok {
		name = e.Household.Name()
	}

	doc, trash, restored, err := store.Restore(s.doc, s.trash, id)
	if err != nil {
		return restoreOutcome{name: name, err: err}
	}

	trash, _ = trash.Purge(s.now())

	token, err := s.persist(store.State{Document: doc, Trash: trash, Settings: s.settings})
	if err != nil {
		return restoreOutcome{name: name, err: err}
	}

	message := restored[0].Name() + " was restored"
	if len(restored) > 1 {
		message += ", along with " + joinNames(restored[1:])
	}

	return restoreOutcome{name: name, message: message + ".", undo: token}
}
