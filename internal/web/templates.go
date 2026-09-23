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

// isFragment reports whether a template file is a fragment rather than a page.
//
// A fragment is swapped into an existing page by htmx, so it must never be
// wrapped in the layout's <html>. The convention is a leading underscore on the
// basename, which mirrors how partials are named in most template systems and
// needs no registry to keep in sync with the files on disk.
func isFragment(file string) bool { return strings.HasPrefix(path.Base(file), "_") }

// mustParsePages builds the per-page sets, panicking on any malformed template.
// It is called from a package-level var, so a failure stops the process at
// startup rather than on the first request.
//
// Every page's set contains every fragment. The two-pane page renders the tree,
// detail, and results fragments inline on first load and htmx re-renders them
// individually afterwards, so the same template text must be reachable both
// ways — otherwise the first paint and the first swap could silently diverge.
func mustParsePages() map[string]*template.Template {
	files, err := fs.Glob(templateFS, "templates/*.html")
	if err != nil {
		panic(fmt.Sprintf("web: glob templates: %v", err))
	}

	var fragments []string
	for _, file := range files {
		if isFragment(file) {
			fragments = append(fragments, file)
		}
	}

	// The base set is the layout plus every fragment. Pages clone it, so a page
	// can override neither, and adding a fragment cannot collide with a page's
	// own "title" or "body".
	base := template.Must(template.ParseFS(templateFS, layoutFile))

	if len(fragments) > 0 {
		base = template.Must(base.ParseFS(templateFS, fragments...))
	}

	parsed := make(map[string]*template.Template, len(files))

	for _, file := range files {
		if file == layoutFile || isFragment(file) {
			continue
		}

		// Clone before parsing: a clone copies the set as it stands, so parsing
		// the page into the shared base first would reintroduce the very
		// collision this indirection exists to prevent.
		var set *template.Template

		set, err = base.Clone()
		if err != nil {
			panic(fmt.Sprintf("web: clone base for %s: %v", file, err))
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

// renderFragment writes a single named template from a page's set, without the
// surrounding layout. htmx swaps the result into an element on a page that is
// already loaded, so emitting the layout here would nest a whole document
// inside a <div>.
//
// page names the set to look the fragment up in; fragment names the template
// within it. Buffering is for the same reason as render: a template error must
// leave w untouched so the caller can still send a clean 500.
func (s *Server) renderFragment(ctx context.Context, w http.ResponseWriter, page, fragment string, data any) error {
	set, ok := pages[page]
	if !ok {
		return fmt.Errorf("render fragment %s: no such page %s", fragment, page)
	}

	// An undefined fragment name needs no guard of its own: ExecuteTemplate
	// reports it, and the wrap below adds the page it was looked up in — the
	// half the library's own error cannot know. One guard then execute is also
	// exactly render's shape, so the two read the same way.
	var buf bytes.Buffer

	err := set.ExecuteTemplate(&buf, fragment, data)
	if err != nil {
		return fmt.Errorf("render fragment %s in page %s: %w", fragment, page, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, err = buf.WriteTo(w)
	if err != nil {
		s.log.DebugContext(ctx, "client disconnected during fragment render", "fragment", fragment, "error", err)
	}

	return nil
}
