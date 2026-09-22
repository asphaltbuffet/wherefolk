package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

// templates is parsed once at startup: the embedded FS is immutable, and a
// malformed template should stop the process rather than surface as a 500 on
// the first request.
var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// render writes a named template to w, buffering first so that a template
// execution error leaves w untouched: the caller can still send a 500 cleanly.
// A failure during the write itself is not recoverable — the status and headers
// are already on the wire by then — but that is a disconnected client, not a bug.
func render(w http.ResponseWriter, name string, data any) error {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := buf.WriteTo(w); err != nil {
		// The status and headers are already on the wire, so this is not
		// recoverable — and it is a disconnected client, not a bug.
		slog.Debug("client disconnected during render", "template", name, "error", err)
	}

	return nil
}
