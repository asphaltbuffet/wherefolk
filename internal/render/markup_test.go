package render_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/internal/render"
)

func TestMarkup(t *testing.T) {
	tests := []struct {
		name      string
		in        render.Directory
		checkFunc func(t *testing.T, got string)
	}{
		{
			name: "imports the on-disk template",
			in:   render.Directory{GeneratedAt: "2026-09-24"},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `#import "directory.typ": directory, household, memorial`,
					"layout lives in the template on disk (ADR-0004)")
			},
		},
		{
			name: "stamps the generation date",
			in:   render.Directory{GeneratedAt: "2026-09-24"},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `generated: "2026-09-24"`)
			},
		},
		{
			name: "a household becomes a household call",
			in: render.Directory{
				GeneratedAt: "2026-09-24",
				Households: []render.Household{{
					Label:        "Robert/Susan",
					AddressLines: []string{"42 Elm Street", "Springfield, IL 62701"},
					Anniversary:  "1991-06-15",
					Adults: []render.Person{{
						Name:  "Robert Langford",
						Birth: "1965-03-12",
						Phone: "555-201-0001",
						Email: "robert.langford@example.com",
					}},
				}},
			},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `#household(`)
				assert.Contains(t, got, `label: "Robert/Susan"`)
				assert.Contains(t, got, `address: ("42 Elm Street", "Springfield, IL 62701",)`)
				assert.Contains(t, got, `anniversary: "1991-06-15"`)
				assert.Contains(t, got, `name: "Robert Langford"`)
				assert.Contains(t, got, `birth: "1965-03-12"`)
				assert.Contains(t, got, `phone: "555-201-0001"`)
			},
		},
		{
			name: "a memorial household uses the compact call",
			in: render.Directory{
				GeneratedAt: "2026-09-24",
				Households: []render.Household{{
					Label:       "Harold/June",
					Memorial:    true,
					Anniversary: "1953-05-23",
					Adults: []render.Person{{
						Name:  "Harold Langford",
						Birth: "1928-02-14",
						Death: "2011-09-30",
					}},
				}},
			},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `#memorial(`)
				assert.NotContains(t, got, `#household(`,
					"a memorial renders more compactly than a live household (§5.4)")
				assert.Contains(t, got, `death: "2011-09-30"`)
			},
		},
		{
			name: "a shared address renders as a back-reference, not lines",
			in: render.Directory{
				GeneratedAt: "2026-09-24",
				Households: []render.Household{{
					Label:      "Daniel/Claire",
					SharedWith: "Robert/Susan",
				}},
			},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `shared: "Robert/Susan"`)
				assert.Contains(t, got, `address: ()`)
			},
		},
		{
			name: "markup metacharacters pass through a string literal unescaped",
			in: render.Directory{
				GeneratedAt: "2026-09-24",
				Households: []render.Household{{
					Label:        "Pat",
					AddressLines: []string{"#1 Elm St"},
					Adults: []render.Person{{
						Name:  "Patricia Novak",
						Email: "pat@example.com",
					}},
				}},
			},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				// Verified against typst 0.14.2: inside a string literal these
				// are ordinary characters. Escaping them would put a literal
				// backslash into the printed Directory.
				assert.Contains(t, got, `"#1 Elm St"`)
				assert.Contains(t, got, `"pat@example.com"`)
				assert.NotContains(t, got, `\#`, "a backslash here would print as a backslash")
				assert.NotContains(t, got, `\@`)
			},
		},
		{
			name: "a backslash and a quote are the escapes a string literal does need",
			in: render.Directory{
				GeneratedAt: "2026-09-24",
				Households: []render.Household{{
					Label: `C:\Users`,
					Adults: []render.Person{{
						Name: `Patricia "Pat" Novak`,
					}},
				}},
			},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `"C:\\Users"`, "an unescaped backslash would start an escape sequence")
				assert.Contains(t, got, `"Patricia \"Pat\" Novak"`, "an unescaped quote would end the string")
			},
		},
		{
			name: "names the tier for the footer",
			in:   render.Directory{GeneratedAt: "September 24, 2026", Tier: "Mail"},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `tier: "Mail"`, "every export names its tier (§5.8)")
				assert.Contains(t, got, `restricted: false`)
			},
		},
		{
			name: "marks a restricted directory",
			in:   render.Directory{GeneratedAt: "September 24, 2026", Tier: "Full", Restricted: true},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `restricted: true`, "Full carries DO NOT DISTRIBUTE (§5.2)")
			},
		},
		{
			name: "an empty directory still produces compilable markup",
			in:   render.Directory{GeneratedAt: "2026-09-24"},
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `#directory(`)
				assert.True(t, strings.HasSuffix(strings.TrimSpace(got), "]"),
					"the directory content block must be closed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, render.Markup(tt.in))
		})
	}
}

func TestMarkupOverExampleDirectory(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, got string)
	}{
		{
			name: "every household in the example is emitted",
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Equal(t, 3, strings.Count(got, "#household("))
				assert.Equal(t, 1, strings.Count(got, "#memorial("))
			},
		},
		{
			name: "the shared address is a back-reference",
			checkFunc: func(t *testing.T, got string) {
				t.Helper()
				assert.Contains(t, got, `shared: "Robert/Susan"`)

				// The invariant is that the target's address appears ONCE — in
				// its own block — and is not repeated into the sharer's. A
				// NotContains on some address absent from the fixture would be
				// vacuously true and would survive deleting the SharesAddress
				// branch entirely, which is the regression this guards.
				assert.Equal(t, 1, strings.Count(got, `"42 Elm Street"`),
					"a shared address is never repeated as lines (§3)")
			},
		},
	}

	markup := render.Markup(exampleDirectory(t))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, markup)
		})
	}
}
