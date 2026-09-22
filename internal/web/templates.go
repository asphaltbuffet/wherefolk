package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

// templates is parsed once at startup: the embedded FS is immutable, and a
// malformed template should stop the process rather than surface as a 500 on
// the first request.
var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

// render writes a named template to w, buffering first so that a template
// execution error does not emit a half-written page under a 200 status.
func render(w http.ResponseWriter, name string, data any) error {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := buf.WriteTo(w)

	return err
}
