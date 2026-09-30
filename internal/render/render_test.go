package render_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
)

func requireRenderer(t *testing.T) render.Renderer {
	t.Helper()

	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not on PATH; run inside `nix develop` to exercise the compile path")
	}

	r, err := render.NewRenderer(templateDir)
	require.NoError(t, err)

	return r
}

func TestRenderer(t *testing.T) {
	// PDF returns one document and SVG returns a page each, so each row renders
	// for itself and asserts inside checkFunc rather than sharing one signature.
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, r render.Renderer, d render.Directory)
	}{
		{
			name: "pdf export is a single document",
			checkFunc: func(t *testing.T, r render.Renderer, d render.Directory) {
				t.Helper()
				out, err := r.PDF(context.Background(), d)
				require.NoError(t, err)
				require.Greater(t, len(out), 4)
				assert.Equal(t, "%PDF", string(out[:4]))
			},
		},
		{
			name: "svg preview is one entry per page",
			checkFunc: func(t *testing.T, r render.Renderer, d render.Directory) {
				t.Helper()
				pages, err := r.SVG(context.Background(), d)
				require.NoError(t, err)
				require.NotEmpty(t, pages)
				assert.Contains(t, string(pages[0]), "<svg")
			},
		},
		{
			name: "an empty directory still renders",
			checkFunc: func(t *testing.T, r render.Renderer, _ render.Directory) {
				t.Helper()
				out, err := r.PDF(context.Background(), render.Directory{GeneratedAt: "2026-09-24"})
				require.NoError(t, err, "a Directory with no Households must still produce a document")
				assert.Equal(t, "%PDF", string(out[:4]))
			},
		},
	}

	r := requireRenderer(t)
	d := exampleDirectory(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, r, d)
		})
	}
}

// typstQuery stages d's markup beside the on-disk template and returns what
// `typst query` prints, as JSON, for selector's field.
func typstQuery(t *testing.T, bin string, d render.Directory, selector, field string) []byte {
	t.Helper()

	dir := t.TempDir()

	tmpl, err := os.ReadFile(filepath.Join(templateDir, render.TemplateName))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, render.TemplateName), tmpl, 0o600))

	src := filepath.Join(dir, "main.typ")
	require.NoError(t, os.WriteFile(src, []byte(render.Markup(d)), 0o600))

	out, err := exec.CommandContext(t.Context(), bin, "query", "--root", dir, src,
		selector, "--field", field, "--format", "json").Output()
	require.NoError(t, err)

	return out
}

// outlined asks typst which headings the Table of Contents lists, in document
// order, by querying the generated markup against the on-disk template.
func outlined(t *testing.T, bin string, d render.Directory) []string {
	t.Helper()

	var bodies []struct {
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(typstQuery(t, bin, d, "heading.where(outlined: true)", "body"), &bodies))

	names := make([]string, 0, len(bodies))
	for _, b := range bodies {
		names = append(names, b.Text)
	}

	return names
}

// labels asks typst for a label-valued field of every element selector
// matches, in document order: "<h_lang01>" for a label, as typst prints it.
func labels(t *testing.T, bin string, d render.Directory, selector, field string) []string {
	t.Helper()

	var out []string
	require.NoError(t, json.Unmarshal(typstQuery(t, bin, d, selector, field), &out))

	return out
}

func TestRenderedStructure(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, r render.Renderer, d render.Directory)
	}{
		{
			name: "the table of contents lists the sections and the first-generation branches in order",
			checkFunc: func(t *testing.T, r render.Renderer, d render.Directory) {
				t.Helper()
				assert.Equal(t, []string{
					"Households",
					"Harold & June (Whitfield) Langford",
					"Robert & Susan (Marsh) Langford",
					"Patricia Novak",
					"Birthdays",
				}, outlined(t, r.Typst.Bin, d), "Daniel & Claire are a grandchild and are not listed")
			},
		},
		{
			name: "the pdf's metadata title is the directory title",
			checkFunc: func(t *testing.T, r render.Renderer, d render.Directory) {
				t.Helper()
				d.Title = "The Langford Family Directory"

				out, err := r.PDF(t.Context(), d)
				require.NoError(t, err)

				api.DisableConfigDir()
				info, err := api.PDFInfo(
					bytes.NewReader(out), "directory.pdf", nil, false, model.NewDefaultConfiguration(),
				)
				require.NoError(t, err)
				assert.Equal(t, "The Langford Family Directory", info.Title)
			},
		},
		{
			name: "a title page and a contents page precede the households",
			checkFunc: func(t *testing.T, r render.Renderer, d render.Directory) {
				t.Helper()
				without, err := r.SVG(t.Context(), render.Directory{GeneratedAt: d.GeneratedAt})
				require.NoError(t, err)
				assert.Len(t, without, 2, "an empty Directory is its Title page and its Table of Contents")
			},
		},
		{
			name: "every household block is a link target, in directory order",
			checkFunc: func(t *testing.T, r render.Renderer, d render.Directory) {
				t.Helper()
				assert.Equal(t,
					[]string{"<h_meml01>", "<h_lang01>", "<h_lang02>", "<h_nova01>"},
					labels(t, r.Typst.Bin, d, "metadata", "label"),
					"a Memorial block is a target too: a living Dependent's row may point at it")
			},
		},
	}

	r := requireRenderer(t)
	d := exampleDirectory(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, r, d)
		})
	}
}

func TestNewRendererRejectsAMissingTemplate(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{
			name: "no such directory",
			dir:  "../../template-does-not-exist",
		},
		{
			name: "a directory without directory.typ",
			dir:  "../../testdata",
		},
	}

	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not on PATH; run inside `nix develop` to exercise the compile path")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := render.NewRenderer(tt.dir)
			require.Error(t, err, "a missing template must fail at construction, not at export time")
		})
	}
}
