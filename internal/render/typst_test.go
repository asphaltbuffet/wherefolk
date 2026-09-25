package render_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
)

// templateDir is where the on-disk Typst template lives, relative to this
// package.
const templateDir = "../../template"

// requireTypst skips a test when the binary is absent.
//
// Typst is a host dependency, not vendored (ADR-0004), and the devShell is
// where it is pinned. Skipping rather than failing keeps `go test ./...` usable
// outside `nix develop`; the markup generation these tests sit on top of is
// pure string work and always runs.
func requireTypst(t *testing.T) render.Typst {
	t.Helper()

	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not on PATH; run inside `nix develop` to exercise the compile path")
	}

	typst, err := render.NewTypst()
	require.NoError(t, err)

	return typst
}

func TestTypstVersion(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, version string)
	}{
		{
			name: "reports a version string",
			checkFunc: func(t *testing.T, version string) {
				t.Helper()
				assert.NotEmpty(t, version)
				assert.Contains(t, version, "typst")
			},
		},
	}

	typst := requireTypst(t)

	version, err := typst.Version(context.Background())
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, version)
		})
	}
}

func TestTypstCompilePDF(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, out []byte)
	}{
		{
			name: "carries the pdf magic bytes",
			checkFunc: func(t *testing.T, out []byte) {
				t.Helper()
				require.Greater(t, len(out), 4)
				assert.Equal(t, "%PDF", string(out[:4]))
			},
		},
	}

	typst := requireTypst(t)

	out, err := typst.CompilePDF(context.Background(), render.Markup(exampleDirectory(t)), templateDir)
	require.NoError(t, err, "the example Directory must compile through the real template")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, out)
		})
	}
}

func TestTypstCompileSVG(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, pages [][]byte)
	}{
		{
			name: "produces at least one page",
			checkFunc: func(t *testing.T, pages [][]byte) {
				t.Helper()
				require.NotEmpty(t, pages)
			},
		},
		{
			name: "every page is an svg document",
			checkFunc: func(t *testing.T, pages [][]byte) {
				t.Helper()
				for i, p := range pages {
					assert.Contains(t, string(p), "<svg", "page %d", i+1)
				}
			},
		},
	}

	typst := requireTypst(t)

	pages, err := typst.CompileSVG(context.Background(), render.Markup(exampleDirectory(t)), templateDir)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, pages)
		})
	}
}

// manyHouseholds builds a Directory large enough to span more than ten SVG
// pages, which is the only size at which the page-ordering bug is visible:
// Typst writes out-1.svg … out-N.svg, and a lexical directory listing sorts
// out-10.svg before out-2.svg.
func manyHouseholds(n int) render.Directory {
	d := render.Directory{GeneratedAt: "2026-09-24"}

	for i := range n {
		d.Households = append(d.Households, render.Household{
			Label:        fmt.Sprintf("House%02d", i),
			AddressLines: []string{"42 Elm Street", "Springfield, IL 62701"},
			Adults: []render.Person{
				{Name: fmt.Sprintf("Adult %02d", i), Birth: "1965-03-12"},
			},
			Dependents: []render.Person{
				{Name: fmt.Sprintf("Dep %02d", i), Birth: "1999-12-25"},
			},
		})
	}

	return d
}

// referencePages compiles markup with typst directly, into a directory this
// test controls, and returns the pages read back strictly by page number.
//
// It deliberately does not share code with CompileSVG: it is the independent
// oracle that CompileSVG's own ordering is checked against, so reusing the
// implementation would make the comparison vacuous.
func referencePages(t *testing.T, markup string, count int) [][]byte {
	t.Helper()

	dir := t.TempDir()

	tmpl, err := os.ReadFile(filepath.Join(templateDir, "directory.typ"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "directory.typ"), tmpl, 0o600))

	src := filepath.Join(dir, "reference.typ")
	require.NoError(t, os.WriteFile(src, []byte(markup), 0o600))

	cmd := exec.CommandContext(t.Context(), "typst",
		"compile", "--root", dir, "--format", "svg", src, filepath.Join(dir, "ref-{p}.svg"))

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "reference compile failed: %s", out)

	pages := make([][]byte, count)

	for i := range count {
		b, readErr := os.ReadFile(filepath.Join(dir, fmt.Sprintf("ref-%d.svg", i+1)))
		require.NoError(t, readErr)

		pages[i] = b
	}

	return pages
}

// TestTypstCompileSVGPageOrder is the regression test for the ordering trap.
//
// CompileSVG reads its pages back by index rather than by the order a glob
// returns, because a lexical listing puts out-10.svg before out-2.svg. A
// document of one or two pages cannot detect that mistake, so this test forces
// one past ten pages and compares every page against an independently compiled
// reference, byte for byte and in order.
//
// Byte comparison rather than a text search is deliberate: Typst renders text
// as glyph paths in SVG, so a page contains no searchable label to assert on.
func TestTypstCompileSVGPageOrder(t *testing.T) {
	typst := requireTypst(t)
	markup := render.Markup(manyHouseholds(60))

	pages, err := typst.CompileSVG(t.Context(), markup, templateDir)
	require.NoError(t, err)
	require.Greater(t, len(pages), 10,
		"the fixture must span more than ten pages or the ordering bug is invisible")

	want := referencePages(t, markup, len(pages))

	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "every page matches the reference page of the same number",
			checkFunc: func(t *testing.T) {
				t.Helper()

				for i := range want {
					assert.Equal(t, sha256.Sum256(want[i]), sha256.Sum256(pages[i]),
						"page %d differs from the reference; pages are out of order", i+1)
				}
			},
		},
		{
			name: "pages past the tenth are not transposed",
			checkFunc: func(t *testing.T) {
				t.Helper()

				// The specific failure a glob-ordered read produces: page 2
				// holds what should be page 10, and vice versa.
				assert.NotEqual(t, sha256.Sum256(want[9]), sha256.Sum256(pages[1]),
					"page 2 holds page 10's content, so the read sorted lexically")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t)
		})
	}
}

func TestTypstCompileRejectsBadMarkup(t *testing.T) {
	tests := []struct {
		name   string
		markup string
	}{
		{
			name:   "an unknown function is a compile error",
			markup: "#no_such_function()",
		},
		{
			name:   "an unclosed content block is a compile error",
			markup: `#import "directory.typ": directory` + "\n#directory(generated: \"x\")[",
		},
	}

	typst := requireTypst(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := typst.CompilePDF(context.Background(), tt.markup, templateDir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "typst", "the operator must be able to see which tool failed")
		})
	}
}
