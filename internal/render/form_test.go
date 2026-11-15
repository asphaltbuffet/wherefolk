package render

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/stretchr/testify/require"
)

// usLetterHeight is the page height Typst reports for paper: "us-letter".
const usLetterHeight = 792.0

// basePDF compiles a two-page US Letter PDF to place fields over.
func basePDF(t *testing.T) []byte {
	t.Helper()

	typst, err := NewTypst()
	if err != nil {
		t.Skip("typst not on PATH")
	}

	pdf, err := typst.CompilePDF(t.Context(),
		"#set page(paper: \"us-letter\")\nFirst\n#pagebreak()\nSecond\n", t.TempDir())
	require.NoError(t, err)

	return pdf
}

// formFields reads back every field pdfcpu finds, by name.
func formFields(t *testing.T, pdf []byte) map[string]string {
	t.Helper()

	fields, err := api.FormFields(t.Context(), bytes.NewReader(pdf), model.NewStatelessConfiguration())
	require.NoError(t, err)

	out := map[string]string{}
	for _, f := range fields {
		out[f.Name] = f.V
	}

	return out
}

func TestAddFields(t *testing.T) {
	base := basePDF(t)

	tests := []struct {
		name   string
		fields []FormField
		want   map[string]string
	}{
		{
			name: "plain value",
			fields: []FormField{{ID: "p1-phone", Kind: TextField, Page: 1,
				X: 72, Y: 100, W: 200, H: 14, PageHeight: usLetterHeight, Value: "(555) 201-4433"}},
			want: map[string]string{"p1-phone": "(555) 201-4433"},
		},
		{
			name: "latin-ext value",
			fields: []FormField{{ID: "p1-surname", Kind: TextField, Page: 1,
				X: 72, Y: 130, W: 200, H: 14, PageHeight: usLetterHeight, Value: "Dvořák Łukasz"}},
			want: map[string]string{"p1-surname": "Dvořák Łukasz"},
		},
		{
			name: "multiline on page two",
			fields: []FormField{{ID: "notes", Kind: MultilineField, Page: 2,
				X: 72, Y: 100, W: 400, H: 60, PageHeight: usLetterHeight}},
			want: map[string]string{"notes": ""},
		},
		{
			name: "checkbox",
			fields: []FormField{{ID: "p1-keep-phone", Kind: CheckField, Page: 1,
				X: 300, Y: 100, W: 9, H: 9, PageHeight: usLetterHeight, Checked: true}},
			want: map[string]string{"p1-keep-phone": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := AddFields(context.Background(), base, tt.fields)
			require.NoError(t, err)

			got := formFields(t, out)
			for name, v := range tt.want {
				require.Contains(t, got, name)

				if v != "" {
					require.Equal(t, v, got[name])
				}
			}
		})
	}
}

func TestAddFieldsRejectsUnknownKind(t *testing.T) {
	base := basePDF(t)

	tests := []struct {
		name string
		kind FieldKind
	}{
		{name: "empty kind", kind: ""},
		{name: "typo kind", kind: "txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AddFields(t.Context(), base, []FormField{{ID: "x", Kind: tt.kind, Page: 1,
				X: 1, Y: 1, W: 10, H: 10, PageHeight: usLetterHeight}})
			require.ErrorContains(t, err, "unknown kind")
		})
	}
}

// pdfcpu loads user fonts once per process. A Full export runs Encrypt, which
// may be the first pdfcpu call the process makes; the field font must survive it.
func TestFormFontSurvivesEncryptFirst(t *testing.T) {
	base := basePDF(t)

	tests := []struct{ name string }{{name: "encrypt then add fields"}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encrypt(t.Context(), base, "spacefreepassphrase")
			require.NoError(t, err)

			var wg sync.WaitGroup
			for range 4 {
				wg.Go(func() {
					_, addErr := AddFields(context.Background(), base, []FormField{{ID: "a", Kind: TextField,
						Page: 1, X: 72, Y: 72, W: 100, H: 14, PageHeight: usLetterHeight, Value: "Zoë"}})
					require.NoError(t, addErr)
				})
			}
			wg.Wait()

			root, setupErr := formSetup()
			require.NoError(t, setupErr)

			registered, fontErr := font.RepositoryForDir(fieldFontDir(root)).IsUserFont(t.Context(), fieldFontName)
			require.NoError(t, fontErr)
			require.True(t, registered, "field font must be registered")
		})
	}
}
