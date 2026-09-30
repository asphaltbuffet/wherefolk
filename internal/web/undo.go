package web

import (
	"maps"
	"net/http"
	"net/url"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

const (
	undoneMessage    = "Your last change was undone."
	undoStaleMessage = "That change can no longer be undone — only the most recent change can be."
)

// undoForm is the Undo control beside an announcement. It posts back the tree
// as the Editor left it, like the household form does.
type undoForm struct {
	Token string
	At    rolo.HouseholdID
	Open  string
	Pane  string

	// Export sends the Editor back to the export page rather than the tree,
	// with Tier as the preview to show; a title change is announced there.
	Export bool
	Tier   string
}

// handleUndo reverses the most recent save, if token still names it, and
// returns the Editor to where they were. Like a save it redirects (ADR-0008).
func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "could not read the form", http.StatusBadRequest)
		return
	}

	snap, undone, err := s.live.Undo(r.PostForm.Get("token"))
	if err != nil {
		s.log.ErrorContext(r.Context(), "undo", "error", err)
		http.Error(w, "the change could not be undone", http.StatusInternalServerError)

		return
	}

	// A stale or unknown token is not an error: it is a sentence for the
	// Editor.
	msg := undoStaleMessage
	if undone {
		msg = undoneMessage
	}

	if r.PostForm.Get("back") == "export" {
		q := url.Values{}
		q.Set(saidKey, msg)

		if tier, ok := render.ParseTier(r.PostForm.Get("tier")); ok {
			q.Set("tier", tier.Key())
		}

		http.Redirect(w, r, exportPageURL(q), http.StatusSeeOther)

		return
	}

	// Undoing a restore or a promotion can remove the very Household the
	// Editor was looking at; land on the directory instead of a dead link.
	// The check reads the Snapshot the Undo produced, not a later one.
	at := rolo.HouseholdID(r.PostForm.Get("at"))
	if _, ok := snap.Tree.Get(at); !ok {
		at = ""
	}

	q := url.Values{}
	q.Set(saidKey, msg)

	http.Redirect(w, r, pageURL(at, "", r.PostForm.Get("open"), r.PostForm.Get("pane"), q), http.StatusSeeOther)
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
