package web

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/asphaltbuffet/wherefolk/internal/render"
)

// fullUnavailable is what the Editor sees when the Full tier cannot be offered.
// It names the missing thing in the family's words and leaves the fix to the
// Operator, who holds the passphrase (§2.3).
const fullUnavailable = "Not available yet — the family passphrase hasn't been set up."

// exportFailed is the Editor-facing message for a render that failed. The
// cause is logged for the Operator; the Editor gets a next step, not an error.
const exportFailed = "Something went wrong making the Directory. Please try again; " +
	"if it keeps happening, let whoever looks after Wherefolk know."

// exportDate is today's calendar date in now's own zone — the host's TZ — as
// midnight UTC.
//
// rolo.Person.IsMinor places an eighteenth birthday at midnight UTC. Passing a
// raw [time.Now] would put an evening export west of UTC on tomorrow's date and
// could treat someone turning 18 today as a minor for a few hours; normalising
// here keeps the age check, the footer and the filename on the date the Editor
// sees on their own calendar.
func exportDate(now time.Time) time.Time {
	y, m, d := now.Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// fullAvailable reports whether the Full tier can be offered at all.
func (s *Server) fullAvailable() bool { return s.cfg.FullPassphrase.Reveal() != "" }

// directoryFor builds the filtered render model for tier, and returns the date
// it was built for.
//
// It holds the read lock only for render.Build, which reads the tree; the
// result is strings, so the compile that follows runs without the lock. A
// Typst compile under the lock would stall every other request, and every
// save, for its duration.
func (s *Server) directoryFor(tier render.Tier) (render.Directory, time.Time) {
	date := exportDate(s.now())

	s.mu.RLock()
	defer s.mu.RUnlock()

	return render.Build(s.tree, tier, date), date
}

// exportURL is the download link for tier.
//
//nolint:unused // Task 4's export page links to it; this task only wires the download route.
func exportURL(tier render.Tier) string {
	return "/export/pdf?" + url.Values{"tier": {tier.Key()}}.Encode()
}

// noStore marks a response as not to be cached anywhere: every export is the
// family's contact data.
func noStore(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }

// handleExportPDF renders and downloads one tier's Directory.
//
// Full without a passphrase is refused before anything is rendered: an
// unencrypted Full-tier PDF must never exist, even transiently.
func (s *Server) handleExportPDF(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	tier, ok := render.ParseTier(r.URL.Query().Get("tier"))
	if !ok {
		http.Error(w, "Choose who the Directory is for.", http.StatusBadRequest)
		return
	}

	if tier == render.Full && !s.fullAvailable() {
		http.Error(w, fullUnavailable, http.StatusForbidden)
		return
	}

	d, date := s.directoryFor(tier)

	pdf, err := s.exporter.PDF(r.Context(), d)
	if err != nil {
		s.log.ErrorContext(r.Context(), "export pdf", "tier", tier.Key(), "error", err)
		http.Error(w, exportFailed, http.StatusInternalServerError)

		return
	}

	if tier == render.Full {
		pdf, err = render.Encrypt(pdf, s.cfg.FullPassphrase.Reveal())
		if err != nil {
			s.log.ErrorContext(r.Context(), "encrypt full export", "error", err)
			http.Error(w, exportFailed, http.StatusInternalServerError)

			return
		}
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="family_directory_%s_%s.pdf"`, tier.Key(), date.Format(time.DateOnly)))

	_, err = w.Write(pdf)
	if err != nil {
		s.log.DebugContext(r.Context(), "client disconnected during export", "error", err)
	}
}

// handleExport is replaced by Task 4. Until then it answers so the route
// table compiles.
func (s *Server) handleExport(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
