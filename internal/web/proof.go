package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/asphaltbuffet/wherefolk/internal/live"
	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

const (
	proofNoLiving = "There is no Proof Sheet for this household: nobody in this household is living, " +
		"so there is no one to send it to."
	proofNotFound = "That household is not in the Directory."
	proofFailed   = "Something went wrong making the Proof Sheet. Please try again; " +
		"if it keeps happening, let whoever looks after Wherefolk know."
)

// proofSection is the Proof Sheet part of a Household's pane. URL is empty
// when there is no sheet to make, and Unavailable then says why.
type proofSection struct {
	URL         string
	Unavailable string
	Open        bool
	Reply       replyForm
}

// proofSectionFor offers h's sheet, or explains its absence, with the Reply
// Instructions form either way: they belong to the Directory, not to h.
func proofSectionFor(h rolo.Household, settings *store.Settings, open, pane string) proofSection {
	base := "/h/" + url.PathEscape(string(h.ID)) + "/proof"

	sec := proofSection{Reply: replyForm{
		Action: base + "/reply",
		Value:  settings.Reply,
		Saved:  settings.Reply != "",
		Open:   open,
		Pane:   pane,
	}}

	if !h.HasLivingMember() {
		sec.Unavailable = proofNoLiving
		return sec
	}

	sec.URL = base

	return sec
}

// handleProofPDF renders and downloads one Household's Proof Sheet. It is
// rendered fresh and never cached: it is that family's contact data.
func (s *Server) handleProofPDF(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	id := rolo.HouseholdID(r.PathValue("id"))
	date := exportDate(s.now())

	sheet, err := proofSheet(s.live.Snapshot(), id, date)

	switch {
	case errors.Is(err, render.ErrNoSuchHousehold):
		http.Error(w, proofNotFound, http.StatusNotFound)
		return
	case errors.Is(err, render.ErrNoLivingMember):
		http.Error(w, proofNoLiving, http.StatusNotFound)
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "build proof sheet", "household", id, "error", err)
		http.Error(w, proofFailed, http.StatusInternalServerError)

		return
	}

	pdf, err := s.exporter.ProofPDF(r.Context(), sheet)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render proof sheet", "household", id, "error", err)
		http.Error(w, proofFailed, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, proofFilename(sheet.Name, id, date)))

	//nolint:gosec // served as an application/pdf attachment, never rendered as HTML.
	_, err = w.Write(pdf)
	if err != nil {
		s.log.DebugContext(r.Context(), "client disconnected during proof download", "error", err)
	}
}

// proofSheet builds id's sheet from snap under the served Title and Reply
// Instructions. Nothing is held during the compile that follows.
func proofSheet(snap live.Snapshot, id rolo.HouseholdID, date time.Time) (render.ProofSheet, error) {
	return render.BuildProof(snap.Tree, id, date, snap.Settings.Title, snap.Settings.Reply)
}

// proofFilename is the download name: proof_sheet_<name>_<date>.pdf.
func proofFilename(name string, id rolo.HouseholdID, date time.Time) string {
	return "proof_sheet_" + slug(name, string(id)) + "_" + date.Format(time.DateOnly) + ".pdf"
}

// slug lowercases name to ASCII letters and digits, folding accents ("Zoë" to
// "zoe") and turning every other run into one underscore. The result is safe
// inside a quoted Content-Disposition filename. A name with nothing ASCII
// left falls back to the ID, which is ASCII by construction.
func slug(name, fallback string) string {
	var b strings.Builder

	gap := false

	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			if gap && b.Len() > 0 {
				b.WriteByte('_')
			}

			gap = false

			b.WriteRune(r)
		default:
			gap = true
		}
	}

	if b.Len() == 0 {
		return fallback
	}

	return b.String()
}
