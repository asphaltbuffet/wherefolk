package render

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// TemplateName is the layout file the generated markup imports. It lives on
// disk rather than embedded in the binary so that a layout adjustment is a file
// edit and a restart rather than a rebuild (ADR-0004).
const TemplateName = "directory.typ"

// Renderer produces the printed Directory and its preview.
//
// One renderer serves both, so the in-app preview cannot drift from what prints
// (§5.1a).
type Renderer struct {
	Typst       Typst
	TemplateDir string
}

// NewRenderer resolves the typst binary and checks the template is present.
//
// Both faults are Operator-facing deployment problems, so they surface at
// construction — while the Operator is watching startup — rather than at export
// time in front of the Editor (ADR-0004).
func NewRenderer(templateDir string) (Renderer, error) {
	typst, err := NewTypst()
	if err != nil {
		return Renderer{}, err
	}

	path := filepath.Join(templateDir, TemplateName)

	_, err = os.Stat(path)
	if err != nil {
		return Renderer{}, fmt.Errorf("render: template %s: %w", path, err)
	}

	return Renderer{Typst: typst, TemplateDir: templateDir}, nil
}

// PDF renders d for export.
func (r Renderer) PDF(ctx context.Context, d Directory) ([]byte, error) {
	return r.Typst.CompilePDF(ctx, Markup(d), r.TemplateDir)
}

// SVG renders d for the in-app preview, one entry per page.
func (r Renderer) SVG(ctx context.Context, d Directory) ([][]byte, error) {
	return r.Typst.CompileSVG(ctx, Markup(d), r.TemplateDir)
}
