package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/asphaltbuffet/wherefolk/internal/live"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// Reply Instructions are a few lines (CONTEXT.md). The caps are sanity limits
// on what fits in the box at a Proof Sheet's head, not measurements of it.
const (
	maxReplyLength = 400
	maxReplyLines  = 6
)

var (
	replyTooLong      = fmt.Sprintf("Reply Instructions can be at most %d characters.", maxReplyLength)
	replyTooManyLines = fmt.Sprintf("Reply Instructions can be at most %d lines.", maxReplyLines)
	replyBadCharacter = "Reply Instructions can contain only ordinary text and line breaks."
)

// replyForm is the Reply Instructions form in a Household's Proof Sheet
// section. Open and Pane carry the tree state, as the household form does.
type replyForm struct {
	Action string
	Value  string
	Error  string
	Open   string
	Pane   string
	Saved  bool
}

// parseReply reads submitted Reply Instructions. It returns the value to
// store, or a sentence for the Editor when the input cannot be stored: a
// refusal, not a Finding (§4.5), like an unusable title.
func parseReply(raw string) (string, string) {
	reply := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))

	if strings.ContainsFunc(reply, isRefusedReplyRune) {
		return "", replyBadCharacter
	}

	if utf8.RuneCountInString(reply) > maxReplyLength {
		return "", replyTooLong
	}

	if strings.Count(reply, "\n")+1 > maxReplyLines {
		return "", replyTooManyLines
	}

	return reply, ""
}

// isRefusedReplyRune is the title's rule with the line break allowed.
func isRefusedReplyRune(r rune) bool { return r != '\n' && isRefusedTitleRune(r) }

// handleReply saves the Reply Instructions from a Household's Proof Sheet
// section and returns the Editor to that Household (ADR-0008 Post/Redirect/Get).
// The Undo beside the announcement returns there too, since announcementFor
// takes the Household as the place to come back to.
func (s *Server) handleReply(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	id := rolo.HouseholdID(r.PathValue("id"))

	if _, ok := s.live.Snapshot().Tree.Get(id); !ok {
		http.Error(w, proofNotFound, http.StatusNotFound)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "could not read the form", http.StatusBadRequest)
		return
	}

	open, pane := r.PostForm.Get("open"), r.PostForm.Get("pane")
	raw := r.PostForm.Get("reply")

	reply, problem := parseReply(raw)
	if problem != "" {
		s.refuseReply(w, r, id, raw, problem)
		return
	}

	token, changed, err := s.setReply(reply)
	if err != nil {
		s.log.ErrorContext(r.Context(), "set reply instructions", "error", err)
		http.Error(w, "the Reply Instructions could not be saved", http.StatusInternalServerError)

		return
	}

	q := url.Values{}
	if changed {
		q.Set(saidKey, replyAnnouncement(reply))
		q.Set(undoKey, token)
	}

	//nolint:gosec // pageURL always builds a local path; the ID is PathEscaped.
	http.Redirect(w, r, pageURL(id, "", open, pane, q), http.StatusSeeOther)
}

// refuseReply re-renders the Household's page with the Editor's own typing
// and the reason, the section open (§4.5).
func (s *Server) refuseReply(w http.ResponseWriter, r *http.Request, id rolo.HouseholdID, raw, problem string) {
	view := s.directoryView(s.live.Snapshot(), id, r.PostForm.Get("open"), "", r.PostForm.Get("pane") == paneClosed)
	if view.Household == nil {
		http.Error(w, proofNotFound, http.StatusNotFound)
		return
	}

	view.Household.Proof.Reply.Value = raw
	view.Household.Proof.Reply.Error = problem
	view.Household.Proof.Open = true

	err := s.render(r.Context(), w, http.StatusUnprocessableEntity, "directory", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render reply refusal", "error", err)
	}
}

// setReply saves reply alone (ADR-0013). Unchanged text is not a save, so it
// does not spend the one Undo step on nothing.
func (s *Server) setReply(reply string) (string, bool, error) {
	saved, err := s.live.Update(func(snap live.Snapshot) (store.State, error) {
		if reply == snap.Settings.Reply {
			return store.State{}, nil
		}

		return store.State{Settings: snap.Settings.WithReply(reply)}, nil
	})
	if err != nil {
		return "", false, err
	}

	return saved.Undo, saved.Changed, nil
}

// replyAnnouncement is the sentence beside a Reply Instructions change's Undo.
func replyAnnouncement(reply string) string {
	if reply == "" {
		return "The Reply Instructions were cleared."
	}

	return "The Reply Instructions were saved."
}
