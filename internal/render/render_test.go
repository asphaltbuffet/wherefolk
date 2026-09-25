package render_test

import (
	"context"
	"os/exec"
	"testing"

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
