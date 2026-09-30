package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/asphaltbuffet/wherefolk/internal/live"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// movedKey carries a promotion's destination Household ID through the
// redirect, so the banner can link there. saidKey carries the announcement
// text itself: every kind of structural change has one, but only a promotion
// has a Household to re-resolve it from. ADR-0001 removed sessions, so there
// is nowhere server-side to keep either and ADR-0008 puts navigation state in
// the URL regardless.
const (
	movedKey = "moved"
	saidKey  = "said"
	// undoKey carries the token of the save this page may undo (undo.go).
	undoKey = "undo"
)

// errRefused ends an Update that decided not to save. What it decided — not
// found, the Editor's field errors, a blocked deletion — travels out in the
// caller's own outcome.
var errRefused = errors.New("change refused")

// handleSave applies one Household's form.
//
// The order is deliberate. Parsing comes first and may refuse the submit
// outright, because input that cannot be stored is not a Finding to display
// beside a saved value — there is no saved value (§4.5). Everything after that
// happens on a clone, so a refusal at any later point leaves the served
// Directory exactly as the disk holds it.
//
// It answers with a redirect rather than a fragment: a save can rename a
// Household or add one, so the tree and the detail pane must re-render together
// from one code path. See ADR-0008.
func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	id := rolo.HouseholdID(r.PathValue("id"))

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "could not read the form", http.StatusBadRequest)
		return
	}

	outcome := s.commit(r, id)

	switch {
	case outcome.notFound:
		s.renderNotFound(w, r)

	case outcome.err != nil:
		s.log.ErrorContext(r.Context(), "save document", "household", id, "error", outcome.err)
		http.Error(w, "the directory could not be saved", http.StatusInternalServerError)

	case len(outcome.fieldErrs) > 0:
		s.refuse(r, w, id, outcome.sub, outcome.fieldErrs)

	default:
		http.Redirect(w, r, redirectAfterSave(id, outcome), http.StatusSeeOther)
	}
}

// renderNotFound answers a request for a Household that is not in the tree:
// for the Editor a sentence and a navigable tree, never a bare code.
func (s *Server) renderNotFound(w http.ResponseWriter, r *http.Request) {
	view := s.directoryView(s.live.Snapshot(), "", "", "", false)

	view.NotFound = true

	err := s.render(r.Context(), w, http.StatusNotFound, "directory", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render not found", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// saveOutcome is what commit decided, for handleSave to turn into a response.
// Separating the two keeps the write lock off the rendering path.
type saveOutcome struct {
	sub       submission
	changes   []change
	fieldErrs []fieldError
	notFound  bool
	// err is a failure the Editor cannot fix by retyping — a failed write, or a
	// document that would not rebuild. It is distinct from fieldErrs, which the
	// Editor can correct.
	err error
	// name is the saved Household's Household Name, for the announcement.
	name string
	// undo is the token of this save's undo point.
	undo string
}

// commit parses, applies and normalises inside one live.Copy.Update, and
// renders nothing: the Copy serialises writers for exactly that long.
func (s *Server) commit(r *http.Request, id rolo.HouseholdID) saveOutcome {
	var out saveOutcome

	saved, err := s.live.Update(func(snap live.Snapshot) (store.State, error) {
		current, ok := snap.Tree.Get(id)
		if !ok {
			// Consistent with the GET path: a stale bookmark is a sentence and
			// a navigable tree, not a code.
			out.notFound = true
			return store.State{}, errRefused
		}

		sub, fieldErrs := parseSubmission(r.Form, current)
		out.sub = sub

		if len(fieldErrs) > 0 {
			out.fieldErrs = fieldErrs
			return store.State{}, errRefused
		}

		// Every mutation lands on a clone: a Snapshot is never mutated, so a
		// failed save leaves the served Directory matching the disk — which
		// is the state the Operator's hand-repair path assumes.
		next := cloneDocument(snap.Document)

		changes, err := s.applySubmission(next, id, sub)
		if err != nil {
			// A submission that would produce a document the store cannot
			// load is the Editor's to fix, so it comes back as a field error
			// rather than a 500: the message names what is wrong with what
			// they asked for.
			s.log.InfoContext(r.Context(), "submission refused", "household", id, "error", err)
			out.fieldErrs = []fieldError{{Message: err.Error()}}

			return store.State{}, errRefused
		}

		// §4.4: normalise on save, then display the normalised value.
		// Iterating by index is required — Normalize takes a pointer receiver
		// and a range copy would be discarded silently.
		for i := range next.Households {
			next.Households[i].Normalize()
		}

		out.changes = changes

		return store.State{Document: next}, nil
	})

	switch {
	case errors.Is(err, errRefused):
		return out
	case err != nil:
		out.err = err
		return out
	}

	h, _ := saved.Snapshot.Tree.Get(id) // the tree was just built from next, which holds id.
	out.name = h.Name()
	out.undo = saved.Undo

	return out
}

// refuse re-renders the form with the Editor's own values and an explanation
// against each field that could not be read.
//
// It takes its own Snapshot: commit's Update has finished by the time
// handleSave decides on a response, and directoryView reads the tree.
//
// It answers 422 rather than redirecting, because a redirect would discard the
// submission and with it everything the Editor typed.
func (s *Server) refuse(
	r *http.Request,
	w http.ResponseWriter,
	id rolo.HouseholdID,
	sub submission,
	fieldErrs []fieldError,
) {
	view := s.directoryView(s.live.Snapshot(), id, sub.Open, "", sub.Pane == paneClosed)

	if view.Household != nil {
		form := formViewFromSubmission(id, sub, fieldErrs)
		form.Crumbs = view.Household.Crumbs
		form.Title = view.Household.Title
		view.Household.Form = &form
	}

	err := s.render(r.Context(), w, http.StatusUnprocessableEntity, "directory", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render refused submission", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// redirectAfterSave builds the URL the Editor lands on. It keeps the tree
// exactly as they left it and carries the announcement and its Undo.
//
// Every save is announced — a plain edit with a plain sentence — because every
// save can be undone, and the Undo belongs beside a sentence saying what it
// would undo (§4.4).
//
// A promotion redirects to the *parent* rather than the new Household: the
// announcement names where the person went and links there, so the Editor
// chooses whether to follow. Being moved somewhere unasked would be the
// surprise §3 introduced the announcement to prevent.
func redirectAfterSave(id rolo.HouseholdID, o saveOutcome) string {
	q := url.Values{}
	q.Set(saidKey, fmt.Sprintf("Your changes to %s were saved.", o.name))

	if len(o.changes) > 0 {
		// Only the first change is announced. A save that both adds and
		// removes shows one sentence rather than a list.
		first := o.changes[0]

		q.Set(saidKey, first.Message)

		if first.Kind == changePromoted {
			q.Set(movedKey, string(first.Household))
		}
	}

	q.Set(undoKey, o.undo)

	return pageURL(id, "", o.sub.Open, o.sub.Pane, q)
}

// announcementFor reads an announcement back off the query string. said is
// the message itself; moved is a promotion's destination Household ID; undo is
// the token of the save it reports. at and open describe the page it appears
// on, for the Undo form to return to. It returns a zero announcement when
// there is nothing to say.
//
// Undo is offered only while it would still work: once a newer save or a
// restart has retired the token, a reload shows the sentence alone rather than
// a button that can only refuse.
func (s *Server) announcementFor(snap live.Snapshot, q url.Values, at rolo.HouseholdID, open string) announcement {
	said := q.Get(saidKey)
	if said == "" {
		return announcement{}
	}

	a := announcement{Message: said}

	if token := q.Get(undoKey); snap.CanUndo(token) {
		a.Undo = &undoForm{Token: token, At: at, Open: open, Pane: q.Get("pane")}
	}

	moved := q.Get(movedKey)
	if moved == "" {
		return a
	}

	household, ok := snap.Tree.Get(rolo.HouseholdID(moved))
	if !ok {
		return a
	}

	a.Link = "/h/" + url.PathEscape(moved)
	a.Label = household.Name()

	return a
}

// announcement is a change as the page reports it, with the Undo that
// reverses it while that is still possible.
type announcement struct {
	Message string
	Link    string
	Label   string
	Undo    *undoForm
}
