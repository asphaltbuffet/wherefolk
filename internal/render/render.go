package render

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

// proofFieldSelector is the label template/directory.typ puts on every Proof
// Sheet field marker.
const proofFieldSelector = "<proof-field>"

// fieldMark is one marker as `typst query` reports it.
type fieldMark struct {
	ID   string    `json:"id"`
	Kind FieldKind `json:"kind"`
	Page int       `json:"page"`
	X    float64   `json:"x"`
	Y    float64   `json:"y"`
	W    float64   `json:"w"`
	H    float64   `json:"h"`
	PH   float64   `json:"ph"`
}

// ProofPDF renders s as a fillable PDF (ADR-0015). It is never encrypted: it
// holds only its recipient's own data (ADR-0005).
func (r Renderer) ProofPDF(ctx context.Context, s ProofSheet) ([]byte, error) {
	pdf, raw, err := r.Typst.CompilePDFAndQuery(ctx, ProofMarkup(s), r.TemplateDir, proofFieldSelector)
	if err != nil {
		return nil, err
	}

	var marks []fieldMark

	err = json.Unmarshal(raw, &marks)
	if err != nil {
		return nil, fmt.Errorf("render: read proof field positions: %w", err)
	}

	fields, err := placeFields(marks, s)
	if err != nil {
		return nil, err
	}

	return AddFields(ctx, pdf, fields)
}

// placeFields joins the template's markers with the sheet's values. The two
// must match exactly: a marker the sheet has no value for is a field nobody
// asked for, and a value with no marker is a question the recipient is never
// asked, the same gap TestFormRendersEveryEditableField guards in the editor.
func placeFields(marks []fieldMark, s ProofSheet) ([]FormField, error) {
	texts, checks := s.FormValues()
	seen := make(map[string]bool, len(marks))
	out := make([]FormField, 0, len(marks))

	for _, m := range marks {
		if seen[m.ID] {
			return nil, fmt.Errorf("render: proof field %q placed twice", m.ID)
		}

		seen[m.ID] = true

		f := FormField{ID: m.ID, Kind: m.Kind, Page: m.Page, X: m.X, Y: m.Y, W: m.W, H: m.H, PageHeight: m.PH}

		if m.Kind == CheckField {
			checked, ok := checks[m.ID]
			if !ok {
				return nil, fmt.Errorf("render: template placed check %q the sheet does not have", m.ID)
			}

			f.Checked = checked
		} else {
			v, ok := texts[m.ID]
			if !ok {
				return nil, fmt.Errorf("render: template placed field %q the sheet does not have", m.ID)
			}

			f.Value = v
		}

		out = append(out, f)
	}

	for _, id := range slices.Concat(slices.Sorted(maps.Keys(texts)), slices.Sorted(maps.Keys(checks))) {
		if !seen[id] {
			return nil, fmt.Errorf("render: template did not place field %q", id)
		}
	}

	return out, nil
}
