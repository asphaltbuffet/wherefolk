package web

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
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

// tierChoice is the copy for one tier on the export page. The Editor chooses
// by what the recipient will do with the Directory, never by the tier's name
// (§5.2); the name appears only in the printed footer.
type tierChoice struct {
	tier        render.Tier
	description string
	detail      string
}

// tierChoices is the export page's list, in the order tiers admit more.
var tierChoices = []tierChoice{
	{render.Mail, "Sending cards", "Addresses and birthdays"},
	{render.Call, "Phoning relatives", "Adds adults' phone numbers"},
	{render.Digital, "Email and messaging", "Adds adults' email addresses"},
	{render.Full, "Full details",
		"Everything, including children's details and years of birth. Opens only with the family passphrase"},
}

// tierOption is one radio button as the template renders it.
type tierOption struct {
	Key         string
	Description string
	Detail      string
	Checked     bool
	Disabled    bool
}

// previewPage is one page of the preview.
type previewPage struct {
	// Src is a data: URL of the page's SVG. It is a template.URL because
	// html/template rejects data: URLs in src as unsafe and would print
	// "#ZgotmplZ" instead. That trust is sound here and nowhere else: the bytes
	// are Typst's own output from markup this service generated, every value in
	// that markup went through quote, and an SVG loaded through <img> cannot
	// run script.
	Src template.URL
	Alt string
}

// exportResult is the part of the page that changes with the chosen tier.
type exportResult struct {
	DownloadURL string
	Pages       []previewPage
	// Error is an Editor-facing message when the preview could not be made.
	Error string
}

// exportView is the export page.
type exportView struct {
	Options []tierOption
	// Result is nil until a tier is chosen, which is what keeps the page from
	// defaulting to an audience the Editor did not pick.
	Result *exportResult
}

// handleExport renders the export page, or — for htmx — just the result
// fragment for the chosen tier. The tier lives in the URL (ADR-0008), so a
// reload or a bookmark shows the same preview.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	tier, chosen := render.ParseTier(r.URL.Query().Get("tier"))
	if tier == render.Full && !s.fullAvailable() {
		chosen = false
	}

	view := exportView{Options: s.tierOptions(tier, chosen)}

	if chosen {
		res := s.previewFor(r, tier)
		view.Result = &res
	}

	var err error

	if wantsFragment(r) {
		err = s.renderFragment(r.Context(), w, "export", "export_result", view.Result)
	} else {
		err = s.render(r.Context(), w, http.StatusOK, "export", view)
	}

	if err != nil {
		s.log.ErrorContext(r.Context(), "render export", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// tierOptions builds the radio buttons, marking the chosen one and disabling
// Full when there is no passphrase to protect it with.
func (s *Server) tierOptions(chosenTier render.Tier, chosen bool) []tierOption {
	options := make([]tierOption, 0, len(tierChoices))

	for _, c := range tierChoices {
		opt := tierOption{
			Key:         c.tier.Key(),
			Description: c.description,
			Detail:      c.detail,
			Checked:     chosen && c.tier == chosenTier,
		}

		if c.tier == render.Full && !s.fullAvailable() {
			opt.Disabled = true
			opt.Detail = fullUnavailable
		}

		options = append(options, opt)
	}

	return options
}

// previewFor renders tier's Directory as SVG pages. A failure becomes a
// message in the page rather than a 500, so the Editor keeps the tier chooser
// and can simply try again.
func (s *Server) previewFor(r *http.Request, tier render.Tier) exportResult {
	d, _ := s.directoryFor(tier)

	pages, err := s.exporter.SVG(r.Context(), d)
	if err != nil {
		s.log.ErrorContext(r.Context(), "export preview", "tier", tier.Key(), "error", err)
		return exportResult{Error: exportFailed}
	}

	res := exportResult{
		DownloadURL: exportURL(tier),
		Pages:       make([]previewPage, 0, len(pages)),
	}

	for i, p := range pages {
		res.Pages = append(res.Pages, previewPage{
			//nolint:gosec // trusted: Typst's own SVG of markup this service generated; see previewPage.Src.
			Src: template.URL("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(p)),
			Alt: "Page " + strconv.Itoa(i+1) + " of " + strconv.Itoa(len(pages)),
		})
	}

	return res
}
