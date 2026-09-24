package web

import (
	"fmt"
	"net/http"
	"net/url"

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
)

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

	// The write lock covers the mutation and nothing else. Rendering a page and
	// writing it to a possibly-slow client must not happen under it: one Editor
	// on a bad connection would otherwise block every reader for the length of
	// the response. handleDirectory releases its read lock before rendering for
	// the same reason.
	outcome := s.commit(r, id)

	switch {
	case outcome.notFound:
		s.mu.RLock()
		view := s.directoryView("", "", "", false)
		s.mu.RUnlock()

		view.NotFound = true

		err = s.render(r.Context(), w, http.StatusNotFound, "directory", view)
		if err != nil {
			s.log.ErrorContext(r.Context(), "render not found", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}

	case outcome.err != nil:
		s.log.ErrorContext(r.Context(), "save document", "household", id, "error", outcome.err)
		http.Error(w, "the directory could not be saved", http.StatusInternalServerError)

	case len(outcome.fieldErrs) > 0:
		s.refuse(r, w, id, outcome.sub, outcome.fieldErrs)

	default:
		http.Redirect(w, r, redirectAfterSave(id, outcome.sub, outcome.changes), http.StatusSeeOther)
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
}

// commit parses, applies, normalises, saves, and swaps in the new document.
// It holds the write lock for exactly that and renders nothing.
func (s *Server) commit(r *http.Request, id rolo.HouseholdID) saveOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.tree.Get(id)
	if !ok {
		// Consistent with the GET path: a stale bookmark is a sentence and a
		// navigable tree, not a code.
		return saveOutcome{notFound: true}
	}

	sub, fieldErrs := parseSubmission(r.Form, current)
	if len(fieldErrs) > 0 {
		return saveOutcome{sub: sub, fieldErrs: fieldErrs}
	}

	// Every mutation lands on a clone. The clone is swapped in only once the
	// save returns, so a full disk leaves the in-memory Directory matching the
	// document on disk — which is the state the Operator's hand-repair path
	// assumes.
	next := cloneDocument(s.doc)

	changes, err := s.applySubmission(next, id, sub)
	if err != nil {
		// A submission that would produce a document the store cannot load is
		// the Editor's to fix, so it comes back as a field error rather than a
		// 500: the message names what is wrong with what they asked for.
		s.log.InfoContext(r.Context(), "submission refused", "household", id, "error", err)

		return saveOutcome{sub: sub, fieldErrs: []fieldError{{Message: err.Error()}}}
	}

	// §4.4: normalise on save, then display the normalised value. Iterating by
	// index is required — Normalize takes a pointer receiver and a range copy
	// would be discarded silently.
	for i := range next.Households {
		next.Households[i].Normalize()
	}

	tree, err := next.Tree()
	if err != nil {
		return saveOutcome{sub: sub, err: fmt.Errorf("edited document does not build a tree: %w", err)}
	}

	err = s.save(next)
	if err != nil {
		return saveOutcome{sub: sub, err: fmt.Errorf("save document: %w", err)}
	}

	s.doc = next
	s.tree = tree

	return saveOutcome{sub: sub, changes: changes}
}

// refuse re-renders the form with the Editor's own values and an explanation
// against each field that could not be read.
//
// It takes the read lock itself: commit has already released the write lock by
// the time handleSave decides on a response, and directoryView reads the tree.
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
	s.mu.RLock()
	view := s.directoryView(id, sub.Open, "", sub.Pane == paneClosed)
	s.mu.RUnlock()

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
// exactly as they left it and carries any announcement.
//
// A promotion redirects to the *parent* rather than the new Household: the
// announcement names where the person went and links there, so the Editor
// chooses whether to follow. Being moved somewhere unasked would be the
// surprise §3 introduced the announcement to prevent.
func redirectAfterSave(id rolo.HouseholdID, sub submission, changes []change) string {
	q := url.Values{}

	if sub.Open != "" {
		q.Set("open", sub.Open)
	}

	if sub.Pane == paneClosed {
		q.Set("pane", paneClosed)
	}

	if len(changes) > 0 {
		// Only the first change is announced. A save that both adds and
		// removes shows one sentence rather than a list.
		first := changes[0]

		q.Set(saidKey, first.Message)

		if first.Kind == changePromoted {
			q.Set(movedKey, string(first.Household))
		}
	}

	target := "/h/" + url.PathEscape(string(id))

	if len(q) == 0 {
		return target
	}

	return target + "?" + q.Encode()
}

// announcementFor reads a structural-change announcement back off the query
// string and renders it for display. said is the message text itself, carried
// verbatim from apply.go; moved is a promotion's destination Household ID,
// present only for that one kind of change. It returns a zero announcement
// when there is nothing to say.
//
// When moved is set but does not resolve in the tree — a stale link — the
// message still renders, just without a destination to link to.
//
// Callers hold at least a read lock.
func (s *Server) announcementFor(said, moved string) announcement {
	if said == "" {
		return announcement{}
	}

	a := announcement{Message: said}

	if moved == "" {
		return a
	}

	household, ok := s.tree.Get(rolo.HouseholdID(moved))
	if !ok {
		return a
	}

	a.Link = "/h/" + url.PathEscape(moved)
	a.Label = household.Label()

	return a
}

// announcement is a structural change as the page reports it.
//
// Undo is item 6's: §4.4 promises one beside this sentence, and the Safety net
// fills the slot the template leaves for it. Until then Message must name the
// change precisely enough for the Editor to reverse by hand from the nightly
// snapshot.
type announcement struct {
	Message string
	Link    string
	Label   string
}
