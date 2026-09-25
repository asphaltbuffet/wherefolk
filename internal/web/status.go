package web

import (
	"net/http"

	"github.com/asphaltbuffet/wherefolk/internal/buildmeta"
)

// statusView is what status.html renders. It is Operator-facing diagnostics —
// the human-readable companion to item 14's /healthz — and deliberately not part
// of the Editor's two-pane interface (§4.1).
type statusView struct {
	Version      string
	BuildInfo    string
	Schema       int
	Households   int
	People       int
	Roots        int
	RootLabels   []string
	DocumentPath string
	TypstVersion string
	TemplatePath string
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	view := s.statusView()
	s.mu.RUnlock()

	err := s.render(r.Context(), w, http.StatusOK, "status", view)
	if err != nil {
		s.log.ErrorContext(r.Context(), "render status", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// statusView summarises the document. Callers hold at least a read lock.
func (s *Server) statusView() statusView {
	roots := s.tree.Roots()

	labels := make([]string, 0, len(roots))
	for _, h := range roots {
		labels = append(labels, h.Label())
	}

	people := 0
	for _, h := range s.doc.Households {
		people += len(h.Adults) + len(h.Dependents)
	}

	return statusView{
		Version:      buildmeta.ShortVersion(),
		BuildInfo:    buildmeta.BuildInfo(),
		Schema:       s.doc.Schema,
		Households:   len(s.doc.Households),
		People:       people,
		Roots:        len(roots),
		RootLabels:   labels,
		DocumentPath: s.meta.DocumentPath,
		TypstVersion: s.meta.TypstVersion,
		TemplatePath: s.meta.TemplatePath,
	}
}
