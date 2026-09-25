package render_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/internal/store"
)

const examplePath = "../../testdata/directory.json"

// exampleDirectory builds the render model over the canonical example.
func exampleDirectory(t *testing.T) render.Directory {
	t.Helper()

	doc, err := store.Load(examplePath)
	require.NoError(t, err)

	tree, err := doc.Tree()
	require.NoError(t, err)

	return render.Build(tree, time.Date(2026, time.September, 24, 0, 0, 0, 0, time.UTC))
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, d render.Directory)
	}{
		{
			name: "every household appears, flat",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				assert.Len(t, d.Households, 4, "a Household never nests inside its parent (ADR-0002)")
			},
		},
		{
			name: "households follow a depth-first walk",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				labels := make([]string, 0, len(d.Households))
				for _, h := range d.Households {
					labels = append(labels, h.Label)
				}
				assert.Equal(t, []string{
					"Harold/June",
					"Robert/Susan",
					"Daniel/Claire",
					"Patricia",
				}, labels)
			},
		},
		{
			name: "the generation date is stamped",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				assert.Equal(
					t,
					"September 24, 2026",
					d.GeneratedAt,
					"exports are non-reproducible and must say when they were made (§5.7)",
				)
			},
		},
		{
			name: "a memorial household is flagged and carries no address",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				h := d.Households[0]
				assert.True(t, h.Memorial)
				assert.Empty(t, h.AddressLines)
				assert.Equal(t, "1953-05-23", h.Anniversary)
			},
		},
		{
			name: "a memorial household keeps its names and dates",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				adults := d.Households[0].Adults
				require.Len(t, adults, 2)
				assert.Equal(t, "Harold Langford", adults[0].Name)
				assert.Equal(t, "1928-02-14", adults[0].Birth)
				assert.Equal(t, "2011-09-30", adults[0].Death)
			},
		},
		{
			name: "a shared address becomes a back-reference to its target's label",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				h := d.Households[2]
				require.Equal(t, "Daniel/Claire", h.Label)
				assert.Equal(t, "Robert/Susan", h.SharedWith)
				assert.Empty(t, h.AddressLines, "a back-reference replaces the repeated block (§3)")
			},
		},
		{
			name: "an own address keeps all its lines",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				h := d.Households[3]
				require.Equal(t, "Patricia", h.Label)
				assert.Equal(t, []string{
					"88 Oakwood Drive",
					"Shelbyville, IL 62565",
					"P.O. Box 212, Shelbyville, IL 62565",
				}, h.AddressLines)
				assert.Empty(t, h.SharedWith)
			},
		},
		{
			name: "a nickname renders in the display name",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				adults := d.Households[3].Adults
				require.Len(t, adults, 1)
				assert.Equal(t, `Patricia "Pat" Novak`, adults[0].Name)
			},
		},
		{
			name: "dependents carry their own dates and contact details",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				deps := d.Households[1].Dependents
				require.Len(t, deps, 2)
				assert.Equal(t, "Thomas Langford", deps[0].Name)
				assert.Equal(t, "2022-01-08", deps[0].Death)
				assert.Equal(t, "Emma Langford", deps[1].Name)
				assert.Equal(t, "555-201-0020", deps[1].Phone)
			},
		},
		{
			name: "item 7 applies no tier rules whatsoever",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				// A deceased dependent's absent phone is absent because it was
				// never recorded, not because anything suppressed it; a living
				// adult's whole birth date survives untruncated. Both are item
				// 8's job, upstream of here.
				adults := d.Households[1].Adults
				require.NotEmpty(t, adults)
				assert.Equal(t, "1965-03-12", adults[0].Birth, "no truncation in item 7 (§5.3)")
				assert.Equal(t, "robert.langford@example.com", adults[0].Email)
				assert.NotContains(t, adults[0].Phone, "private", "no [private] in item 7 (§5.5)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, exampleDirectory(t))
		})
	}
}
