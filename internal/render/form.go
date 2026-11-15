package render

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// FieldKind is what a Proof Sheet field accepts. The template names it on
// each field's marker; AddFields turns it into a PDF field type.
type FieldKind string

// The kinds of field a Proof Sheet carries.
const (
	TextField      FieldKind = "text"
	MultilineField FieldKind = "multiline"
	CheckField     FieldKind = "check"
)

// FormField is one form field to place over a compiled page.
//
// Geometry is in points measured from the page's top-left corner, which is how
// Typst reports a position; AddFields converts to PDF's bottom-left origin.
type FormField struct {
	ID         string
	Kind       FieldKind
	Page       int
	X, Y, W, H float64
	PageHeight float64
	Value      string
	Checked    bool
}

// fieldFontTTF is the font a viewer draws field values in. It is embedded,
// not the template's, because a viewer draws a field's value itself, in a font
// the PDF declares for the field; pdfcpu's standard fonts are WinAnsi and
// cannot draw "Dvořák" (ADR-0015).
//
//go:embed fonts/DejaVuSans.ttf
var fieldFontTTF []byte

const (
	fieldFontName = "DejaVuSans"
	fieldFontSize = 10
)

var (
	formSetupOnce sync.Once
	formRoot      string
	errFormSetup  error
)

// formSetup prepares a private pdfcpu configuration root holding the field
// font, once per process, and returns it.
//
// pdfcpu's stateless configuration, which Encrypt uses, deliberately sees no
// user fonts, and its package-level font directory is process-global state
// another caller could change or load first. A configuration root of our own,
// under the system temp directory rather than $HOME, avoids both: each
// AddFields loads it read-only, and pdfcpu caches fonts per directory.
func formSetup() (string, error) {
	formSetupOnce.Do(func() {
		root, err := os.MkdirTemp("", "wherefolk-pdfcpu-")
		if err != nil {
			errFormSetup = fmt.Errorf("render: pdfcpu configuration directory: %w", err)
			return
		}

		_, _, err = model.InitializeConfiguration(root)
		if err != nil {
			errFormSetup = fmt.Errorf("render: initialise pdfcpu configuration: %w", err)
			return
		}

		err = font.InstallFontFromBytesQuiet(fieldFontDir(root), fieldFontName, fieldFontTTF)
		if err != nil {
			errFormSetup = fmt.Errorf("render: install field font: %w", err)
			return
		}

		formRoot = root
	})

	return formRoot, errFormSetup
}

// fieldFontDir is where pdfcpu looks for user fonts under a configuration root.
func fieldFontDir(root string) string { return filepath.Join(root, "pdfcpu", "fonts") }

// The JSON shapes below are the subset of pdfcpu's create format a Proof Sheet
// needs (pkg/pdfcpu/primitives).
type (
	pdfcpuForm struct {
		Pages map[string]pdfcpuPage `json:"pages"`
	}
	pdfcpuPage struct {
		Content pdfcpuContent `json:"content"`
	}
	pdfcpuContent struct {
		TextFields []pdfcpuTextField `json:"textfield,omitempty"`
		CheckBoxes []pdfcpuCheckBox  `json:"checkbox,omitempty"`
	}
	pdfcpuFont struct {
		Name string `json:"name"`
		Size int    `json:"size"`
	}
	pdfcpuTextField struct {
		ID        string     `json:"id"`
		Value     string     `json:"value"`
		Pos       [2]float64 `json:"pos"`
		Width     float64    `json:"width"`
		Height    float64    `json:"height"`
		Multiline bool       `json:"multiline"`
		Font      pdfcpuFont `json:"font"`
	}
	pdfcpuCheckBox struct {
		ID    string     `json:"id"`
		Value bool       `json:"value"`
		Pos   [2]float64 `json:"pos"`
		Width float64    `json:"width"`
	}
)

// AddFields returns pdf with fields placed over its pages as an AcroForm.
//
// It is a pure []byte → []byte step, like Encrypt (ADR-0011): the layout is
// already final, and nothing here moves or draws page content.
func AddFields(ctx context.Context, pdf []byte, fields []FormField) ([]byte, error) {
	root, err := formSetup()
	if err != nil {
		return nil, err
	}

	conf, err := model.LoadConfigurationReadOnly(root)
	if err != nil {
		return nil, fmt.Errorf("render: load pdfcpu configuration: %w", err)
	}

	form := pdfcpuForm{Pages: map[string]pdfcpuPage{}}

	for _, f := range fields {
		key := strconv.Itoa(f.Page)
		page := form.Pages[key]
		pos := [2]float64{f.X, f.PageHeight - f.Y - f.H}

		switch f.Kind {
		case CheckField:
			page.Content.CheckBoxes = append(page.Content.CheckBoxes,
				pdfcpuCheckBox{ID: f.ID, Value: f.Checked, Pos: pos, Width: f.W})
		case TextField, MultilineField:
			page.Content.TextFields = append(page.Content.TextFields, pdfcpuTextField{
				ID: f.ID, Value: f.Value, Pos: pos, Width: f.W, Height: f.H,
				Multiline: f.Kind == MultilineField,
				Font:      pdfcpuFont{Name: fieldFontName, Size: fieldFontSize},
			})
		default:
			return nil, fmt.Errorf("render: form field %q has unknown kind %q", f.ID, f.Kind)
		}

		form.Pages[key] = page
	}

	spec, err := json.Marshal(form)
	if err != nil {
		return nil, fmt.Errorf("render: encode form: %w", err)
	}

	var out bytes.Buffer

	err = api.Create(ctx, bytes.NewReader(pdf), bytes.NewReader(spec), &out, conf)
	if err != nil {
		return nil, fmt.Errorf("render: add form fields: %w", err)
	}

	return out.Bytes(), nil
}
