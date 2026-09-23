package web

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// layoutFile is the shared chrome every page clones. It is the one template
// file that is not itself a page.
const layoutFile = "templates/layout.html"

// pages maps a page name to its own template set: the layout, cloned, with that
// page's file parsed into it. Each page therefore gets a private namespace for
// "title" and "body", so adding a page can never collide with an existing one.
//
// Parsing happens once at startup: the embedded FS is immutable, and a malformed
// template should stop the process rather than surface as a 500 on the first
// request.
var pages = mustParsePages()

// mustParsePages builds the per-page sets, panicking on any malformed template.
// It is called from a package-level var, so a failure stops the process at
// startup rather than on the first request.
func mustParsePages() map[string]*template.Template {
	layout := template.Must(template.ParseFS(templateFS, layoutFile))

	files, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		panic(fmt.Sprintf("web: glob templates: %v", err))
	}

	parsed := make(map[string]*template.Template, len(files))

	for _, file := range files {
		if file == layoutFile {
			continue
		}

		// Clone before parsing: a clone copies the set as it stands, so parsing
		// the page into the shared layout first would reintroduce the very
		// collision this indirection exists to prevent.
		var set *template.Template

		set, err = layout.Clone()
		if err != nil {
			panic(fmt.Sprintf("web: clone layout for %s: %v", file, err))
		}

		_, err = set.ParseFS(templateFS, file)
		if err != nil {
			panic(fmt.Sprintf("web: parse %s: %v", file, err))
		}

		parsed[strings.TrimSuffix(path.Base(file), ".html")] = set
	}

	return parsed
}

// render writes a page to w, buffering first so that a template execution error
// leaves w untouched: the caller can still send a 500 cleanly. A failure during
// the write itself is not recoverable — the status and headers are already on
// the wire by then — but that is a disconnected client, not a bug.
//
// name is the page, not the template: "status" renders templates/status.html
// wrapped in the layout.
func (s *Server) render(ctx context.Context, w http.ResponseWriter, name string, data any) error {
	set, ok := pages[name]
	if !ok {
		return fmt.Errorf("render %s: no such page", name)
	}

	var buf bytes.Buffer

	err := set.ExecuteTemplate(&buf, "layout", data)
	if err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, err = buf.WriteTo(w)
	if err != nil {
		// The status and headers are already on the wire, so this is not
		// recoverable — and it is a disconnected client, not a bug.
		s.log.DebugContext(ctx, "client disconnected during render", "page", name, "error", err)
	}

	return nil
}
