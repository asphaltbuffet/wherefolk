package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/internal/store"
)

// maxTitleLength is a sanity cap on the submitted title, in characters, not a
// measurement of what fits on the Title page: "one line" means the input
// carries no line break, and the Title page may still wrap a long title.
const maxTitleLength = 80

var (
	titleTooLong    = fmt.Sprintf("A title can be at most %d characters.", maxTitleLength)
	titleNotOneLine = "A title must be a single line."
)

// parseTitle reads a submitted Directory Title. It returns the value to store,
// or — when the input cannot be stored — a sentence for the Editor instead.
// Like an unparseable date, that refuses the submit rather than becoming a
// Finding: there is no stored value to observe (§4.5).
func parseTitle(raw string) (string, string) {
	title := strings.TrimSpace(raw)

	if strings.ContainsFunc(title, isRefusedTitleRune) {
		return "", titleNotOneLine
	}

	if utf8.RuneCountInString(title) > maxTitleLength {
		return "", titleTooLong
	}

	return title, ""
}

// isRefusedTitleRune reports whether r breaks the single-line rule: a control
// character, a Unicode line or paragraph separator (U+2028, U+2029), or a
// format character such as a bidi override (U+202E) that could make the
// rendered title read differently than it was typed.
func isRefusedTitleRune(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp, unicode.Cf)
}

// exportPageURL is the export page carrying q.
func exportPageURL(q url.Values) string {
	if len(q) == 0 {
		return "/export"
	}

	return "/export?" + q.Encode()
}

// handleTitle sets the Directory Title from the export page's form and
// returns the Editor there, to the preview they were looking at, announcing
// the change beside an Undo (ADR-0008 Post/Redirect/Get).
func (s *Server) handleTitle(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "could not read the form", http.StatusBadRequest)
		return
	}

	tier, chosen := render.ParseTier(r.PostForm.Get("tier"))
	if tier == render.Full && !s.fullAvailable() {
		chosen = false
	}

	raw := r.PostForm.Get("title")

	title, problem := parseTitle(raw)
	if problem != "" {
		s.renderExport(w, r, http.StatusUnprocessableEntity, tier, chosen,
			exportTitle{Value: raw, Error: problem}, announcement{})

		return
	}

	token, changed, err := s.setTitle(title)
	if err != nil {
		s.log.ErrorContext(r.Context(), "set title", "error", err)
		http.Error(w, "the title could not be saved", http.StatusInternalServerError)

		return
	}

	q := url.Values{}
	if chosen {
		q.Set("tier", tier.Key())
	}

	if changed {
		q.Set(saidKey, titleAnnouncement(title))
		q.Set(undoKey, token)
	}

	http.Redirect(w, r, exportPageURL(q), http.StatusSeeOther)
}

// setTitle saves title if it differs from the served one. An unchanged title
// is not a save: it would spend the one Undo step on nothing.
func (s *Server) setTitle(title string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if title == s.settings.Title {
		return "", false, nil
	}

	token, err := s.persist(store.State{
		Document: s.doc,
		Trash:    s.trash,
		Settings: s.settings.WithTitle(title),
	})
	if err != nil {
		return "", false, err
	}

	return token, true, nil
}

// titleAnnouncement is the sentence beside a title change's Undo.
func titleAnnouncement(title string) string {
	if title == "" {
		return fmt.Sprintf("The title was cleared, so the Directory is titled “%s”.", render.DefaultTitle)
	}

	return fmt.Sprintf("The Directory is now titled “%s”.", title)
}
