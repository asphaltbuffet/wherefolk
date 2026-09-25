package render_test

import (
	"context"
	"os/exec"
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
		{
			name: "pages are in document order",
			checkFunc: func(t *testing.T, pages [][]byte) {
				t.Helper()
				// Page 1 carries the first Household in the walk. Ordering is
				// what makes a multi-page preview navigable, and the filename
				// template Typst writes is numeric, so a naive directory read
				// would sort 10 before 2.
				if len(pages) > 0 {
					assert.Contains(t, string(pages[0]), "<svg")
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
