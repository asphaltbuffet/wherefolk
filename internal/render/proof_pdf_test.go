package render_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestProofPDF(t *testing.T) {
	typst := requireTypst(t)
	r := render.Renderer{Typst: typst, TemplateDir: templateDir}

	kids := make([]rolo.Person, 0, 8)
	for i := range 8 {
		kids = append(kids, rolo.Person{ID: rolo.PersonID("p_k" + strconv.Itoa(i)), Given: "Kid", Surname: "Dvořák"})
	}

	tests := []struct {
		name         string
		sheet        render.ProofSheet
		wantLastPage int
	}{
		{name: "one page", sheet: yoderSheet(t, "Send to Dawn"), wantLastPage: 1},
		{name: "two pages", sheet: yoderSheet(t, "", kids...), wantLastPage: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf, err := r.ProofPDF(t.Context(), tt.sheet)
			require.NoError(t, err)

			fields, err := api.FormFields(t.Context(), bytes.NewReader(pdf), model.NewStatelessConfiguration())
			require.NoError(t, err)

			texts, checks := tt.sheet.FormValues()
			require.Len(t, fields, len(texts)+len(checks))

			lastPage := 0

			for _, f := range fields {
				if v, ok := texts[f.Name]; ok && v != "" {
					require.Equal(t, v, f.V, f.Name)
				}

				for _, p := range f.Pages {
					lastPage = max(lastPage, p)
				}
			}

			require.GreaterOrEqual(t, lastPage, tt.wantLastPage)

			if tt.wantLastPage == 1 {
				require.Equal(t, 1, lastPage)
			}
		})
	}
}
