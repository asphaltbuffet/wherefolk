package web

import (
	"crypto/rand"
	"fmt"
	"maps"
	"net/http"
	"net/url"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

const (
	undoneMessage    = "Your last change was undone."
	undoStaleMessage = "That change can no longer be undone — only the most recent change can be."
)

// undoPoint is the state before the most recent save: the one step Undo can
// return to (CONTEXT.md, Undo). The server is the single writer, so this is the
// whole of the "session" §3 speaks of — it lasts as long as the process, which
// ADR-0001's lack of sessions leaves as the only lifetime there is.
//
// token names the save this point undoes. It rides in the redirect URL
// (ADR-0008), and a token that no longer matches is refused, so an Undo left
// on screen can never discard work saved after it.
type undoPoint struct {
	token  string
	before store.State
}

// undoForm is the Undo control beside an announcement. It posts back the tree
// as the Editor left it, like the household form does.
type undoForm struct {
	Token string
	At    rolo.HouseholdID
	Open  string
	Pane  string
}

// persist makes next the served state: it builds the tree, saves, swaps both
// in, and records what it replaced as the one step Undo can return to. It
// returns that step's token. On any error nothing is swapped and the previous
// undo point stands.
//
// Callers hold the write lock.
func (s *Server) persist(next store.State) (string, error) {
	tree, err := next.Document.Tree()
	if err != nil {
		return "", fmt.Errorf("changed document does not build a tree: %w", err)
	}

	prev := s.state()

	err = s.save(prev, next)
	if err != nil {
		return "", fmt.Errorf("save document: %w", err)
	}

	s.doc, s.trash, s.tree = next.Document, next.Trash, tree

	token := rand.Text()
	s.undo = &undoPoint{token: token, before: prev}

	return token, nil
}

// handleUndo reverses the most recent save, if token still names it, and
// returns the Editor to where they were. Like a save it redirects (ADR-0008).
func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "could not read the form", http.StatusBadRequest)
		return
	}

	msg, err := s.undoLatest(r.PostForm.Get("token"))
	if err != nil {
		s.log.ErrorContext(r.Context(), "undo", "error", err)
		http.Error(w, "the change could not be undone", http.StatusInternalServerError)

		return
	}

	// Undoing a restore or a promotion can remove the very Household the
	// Editor was looking at; land on the directory instead of a dead link.
	at := rolo.HouseholdID(r.PostForm.Get("at"))

	s.mu.RLock()
	_, ok := s.tree.Get(at)
	s.mu.RUnlock()

	if !ok {
		at = ""
	}

	q := url.Values{}
	q.Set(saidKey, msg)

	http.Redirect(w, r, pageURL(at, "", r.PostForm.Get("open"), r.PostForm.Get("pane"), q), http.StatusSeeOther)
}

// undoLatest restores the undo point if token names it. A stale or unknown
// token is not an error: it is a sentence for the Editor.
func (s *Server) undoLatest(token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if token == "" || s.undo == nil || token != s.undo.token {
		return undoStaleMessage, nil
	}

	_, err := s.persist(s.undo.before)
	if err != nil {
		return "", fmt.Errorf("undo: %w", err)
	}

	// One step, no redo: persist recorded the undone state as a new point, and
	// offering it would make Undo a toggle.
	s.undo = nil

	return undoneMessage, nil
}

// pageURL links to a Household's page, or with action to one of its sub-pages
// such as "delete", keeping the tree as the Editor left it (ADR-0008). An empty
// id is the directory root, which has no sub-pages.
func pageURL(id rolo.HouseholdID, action, open, pane string, extra url.Values) string {
	q := url.Values{}
	maps.Copy(q, extra)

	if open != "" {
		q.Set("open", open)
	}

	if pane == paneClosed {
		q.Set("pane", paneClosed)
	}

	target := "/"
	if id != "" {
		target = "/h/" + url.PathEscape(string(id))
		if action != "" {
			target += "/" + action
		}
	}

	if len(q) == 0 {
		return target
	}

	return target + "?" + q.Encode()
}
