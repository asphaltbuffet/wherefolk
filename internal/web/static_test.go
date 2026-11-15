package web_test

import (
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaticFonts pins the self-hosted fonts: each is served with its media
// type and declared by the stylesheet. latin-ext is there for surnames such as
// Łukasiewicz or Dvořák, which would otherwise fall back to a system font.
func TestStaticFonts(t *testing.T) {
	css := get(t, sampleDocument(), "/static/app.css")
	require.Equal(t, http.StatusOK, css.Code)
	stylesheet := css.Body.String()

	tests := []struct {
		name string
		file string
	}{
		{name: "Atkinson latin", file: "atkinson-hyperlegible-next-latin-wght-normal.woff2"},
		{name: "Atkinson latin italic", file: "atkinson-hyperlegible-next-latin-wght-italic.woff2"},
		{name: "Atkinson latin-ext", file: "atkinson-hyperlegible-next-latin-ext-wght-normal.woff2"},
		{name: "Atkinson latin-ext italic", file: "atkinson-hyperlegible-next-latin-ext-wght-italic.woff2"},
		{name: "Besley latin", file: "besley-latin-wght-normal.woff2"},
		{name: "Besley latin italic", file: "besley-latin-wght-italic.woff2"},
		{name: "Besley latin-ext", file: "besley-latin-ext-wght-normal.woff2"},
		{name: "Besley latin-ext italic", file: "besley-latin-ext-wght-italic.woff2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), "/static/fonts/"+tt.file)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "font/woff2", rec.Header().Get("Content-Type"),
				"a font type that does not depend on the host's mime table")

			body, err := io.ReadAll(rec.Body)
			require.NoError(t, err)
			assert.Equal(t, "wOF2", string(body[:4]), "the file is a woff2 font")

			assert.Contains(t, stylesheet, "url(/static/fonts/"+tt.file+")",
				"the stylesheet declares the font it ships")
		})
	}
}

// offTailnet matches an asset reference that would make the browser fetch
// from the open internet: a family's PII must not announce who is reading it.
var offTailnet = regexp.MustCompile(
	`(?i)<(link|script|img)[^>]+(href|src)\s*=\s*["']?\s*(https?:)?//|url\(\s*["']?\s*(https?:)?//|@import`,
)

func TestNoOffTailnetAssets(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "the directory", target: "/"},
		{name: "a household", target: "/h/h_clyde"},
		{name: "recently deleted", target: "/trash"},
		{name: "the export page", target: "/export"},
		{name: "a delete confirmation", target: "/h/h_dave/delete"},
		{name: "the status page", target: "/status"},
		{name: "the stylesheet", target: "/static/app.css"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.NotRegexp(t, offTailnet, rec.Body.String())
		})
	}
}

// TestStylesheetKeepsFocusFindable pins the rules a keyboard user depends on:
// a field's focus ring is solid --focus, the sticky Save bar never covers the
// field that just took focus, and a long unbroken name wraps instead of
// pushing the page sideways.
func TestStylesheetKeepsFocusFindable(t *testing.T) {
	rec := get(t, sampleDocument(), "/static/app.css")
	require.Equal(t, http.StatusOK, rec.Code)
	css := rec.Body.String()

	tests := []struct {
		name string
		rule string
	}{
		{name: "a focused field has a solid ring", rule: "box-shadow: 0 0 0 3px var(--focus);"},
		{name: "scrolling leaves room for the save bar", rule: ".pane-detail { scroll-padding-bottom: 6rem; }"},
		{name: "search results wrap", rule: ".results a { font-weight: 700; overflow-wrap: anywhere; }"},
		{
			name: "the path wraps",
			rule: ".breadcrumb { color: var(--on-desk-muted); min-width: 0; overflow-wrap: anywhere; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, css, tt.rule)
		})
	}
}
