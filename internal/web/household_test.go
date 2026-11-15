package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// assertNoMaskedValue checks that "[private]" never appears as a substituted
// field value: ADR-0010 says the editing UI never masks. Every Hide toggle
// carries fixed hover text that itself quotes the marker ("Print [private]
// instead of this in the Directory"), so that known-good text is stripped
// out first — comparing counts instead would let a missing toggle title and
// a real masked value cancel out and pass undetected.
func assertNoMaskedValue(t *testing.T, body, msg string) {
	t.Helper()
	stripped := strings.ReplaceAll(body, `title="Print [private] instead of this in the Directory"`, "")
	assert.NotContains(t, stripped, "[private]", msg)
}

// personCard returns the markup of one person's card: from its opening
// fieldset to the next closing one. Person cards never nest fieldsets.
func personCard(t *testing.T, body, key string) string {
	t.Helper()
	start := strings.Index(body, `data-person="`+key+`"`)
	require.NotEqual(t, -1, start, "card for %s", key)
	end := strings.Index(body[start:], "</fieldset>")
	require.NotEqual(t, -1, end)
	return body[start : start+end]
}

func TestDirectoryPage(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		wantStatus int
		checkFunc  func(t *testing.T, body string)
	}{
		{
			name:       "the root shows the tree with no Household selected",
			target:     "/",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "<!DOCTYPE html>", "the root is a page, not a fragment")
				assert.Contains(t, body, "Aden &amp; Nettie Whitlock")
				assert.Contains(t, body, "Choose a household", "an empty detail pane explains itself")
			},
		},
		{
			name:       "selecting a Household renders both panes",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "<h1>Clyde &amp; Doris (Kowalski) Whitlock")
				assert.Contains(t, body, "1412 Oak St")
				assert.Contains(t, body, "Aden &amp; Nettie Whitlock", "the tree pane is still there")
			},
		},
		{
			name:       "the Path breadcrumb is always visible",
			target:     "/h/h_dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "›", "§4.1: the Path is the disambiguation mechanism")
				assert.Contains(t, body, "Clyde/Doris")
			},
		},
		{
			name:       "a withheld field still shows its value; hiding is an export concern",
			target:     "/h/h_reeve",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				// ADR-0010: the editing UI never masks. h_reeve withholds its
				// address and Ray's phone in the fixture, which affects
				// export only.
				assertNoMaskedValue(t, body,
					"the editing UI never substitutes the marker")
				assert.Contains(t, body, "9 Elm St")
				assert.Contains(t, body, "555-0199")
			},
		},
		{
			name:       "a shared address is a back-reference, not a repeated block",
			target:     "/h/h_dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Same address as Clyde &amp; Doris (Kowalski) Whitlock")
				// "1412 Oak St" only ever appears on h_clyde's own page, so
				// asserting its absence here proves nothing on its own — it
				// would pass even if the back-reference were dropped entirely.
				// The real guard is that an address row exists and names the
				// parent rather than repeating any address text.
				assert.NotContains(t, body, "1412 Oak St", "§3: a Shared Address renders as a reference")
				assertNoMaskedValue(t, body,
					"h_dave withholds nothing; a marker here would mean the switch fell through")
			},
		},
		{
			name:       "a Memorial Household still shows its recorded contact details",
			target:     "/h/h_aden",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Memorial")
				assert.Contains(t, body, "1989-11-17", "§5.3: a deceased person's dates are whole")

				// ADR-0010: suppression is export-only. The Editor must be
				// able to see and clear a deceased person's recorded details.
				assert.Contains(t, body, "555-0100")
				assert.Contains(t, body, "aden@example.com")
				assertNoMaskedValue(t, body,
					"the editing UI never substitutes the marker")
			},
		},
		{
			name:       "an unknown Household explains itself in plain English",
			target:     "/h/h_ghost",
			wantStatus: http.StatusNotFound,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "not in the directory")
				assert.NotContains(t, body, "404", "the Editor never sees an error code")
				assert.Contains(t, body, "Aden &amp; Nettie Whitlock", "the tree is still navigable from the error")
				assert.NotContains(t, body, `class="breadcrumb"`, "no Household means no Path to show")
				assert.Equal(
					t,
					1,
					strings.Count(body, "Make a Directory to send"),
					"the top bar still offers the export link",
				)
			},
		},
		{
			name:       "the breadcrumb and export link share the top line, above the heading",
			target:     "/h/h_dave",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				top := strings.Index(body, `class="detail-top"`)
				crumb := strings.Index(body, `class="breadcrumb"`)
				link := strings.Index(body, "Make a Directory to send")
				heading := strings.Index(body, "<h1>")
				require.NotEqual(t, -1, top, "the detail pane has a top bar")
				assert.Less(t, top, crumb)
				assert.Less(t, crumb, link, "breadcrumb on the left, link on the right")
				assert.Less(t, link, heading, "both above the heading")
				assert.Equal(t, 1, strings.Count(body, "Make a Directory to send"), "the link appears once")

				bar := body[top : top+strings.Index(body[top:], "</div>")]
				assert.Contains(t, bar, `class="breadcrumb"`, "the breadcrumb sits inside .detail-top")
				assert.Contains(t, bar, "Make a Directory to send", "the export link sits inside .detail-top")
			},
		},
		{
			name:       "a root Household still shows its breadcrumb",
			target:     "/h/h_aden",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, `<a href="/h/h_aden">Aden/Nettie</a>`)
			},
		},
		{
			name:       "with nothing selected the top bar holds the link alone",
			target:     "/",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, `class="detail-top"`)
				assert.NotContains(t, body, `class="breadcrumb"`)
				assert.Equal(t, 1, strings.Count(body, "Make a Directory to send"))
			},
		},
		{
			name:       "each withheld-field toggle is an inline Keep private with an explanation",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "Keep out of the printed directory")
				assert.Contains(t, body, `title="Print [private] instead of this in the Directory"`)
				for _, field := range []string{"birth date", "phone", "email", "address"} {
					assert.Contains(t, body, `aria-label="Keep `+field+` private in the printed directory"`)
				}
				// Pairs every toggle with a row; a stray toggle outside a row, or
				// a row without its toggle, breaks the equality.
				assert.Equal(t, strings.Count(body, `aria-label="Keep `), strings.Count(body, `class="field-row"`),
					"every Keep private sits in a field row with its field")
				assert.Equal(
					t,
					strings.Count(body, `title="Print [private] instead of this in the Directory"`),
					strings.Count(body, `aria-label="Keep `),
					"every toggle carries its hover text",
				)
			},
		},
		{
			name:       "date fields show the format, not a sample date",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, `placeholder="1998-06-14"`)
				assert.NotContains(t, body, `placeholder="1971-03-02"`)
				// -birth" and -death" alone would match the preceding <label
				// for="..."> before the <input id="...">, since both share the
				// same value; anchoring on " name=\"person." finds the input.
				for _, field := range []string{`id="anniversary"`, `-birth" name="person.`, `-death" name="person.`} {
					i := strings.Index(body, field)
					require.NotEqual(t, -1, i, field)
					tag := body[i : i+strings.Index(body[i:], ">")]
					assert.Contains(t, tag, `placeholder="YYYY-MM-DD"`, field)
				}
			},
		},
		{
			name:       "/status still works",
			target:     "/status",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Wherefolk status", "the Operator's bookmark does not move")
			},
		},
		{
			name:       "the household summary sits under the heading",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				heading := strings.Index(body, "<h1>")
				summary := strings.Index(
					body,
					`<p class="summary">Two adults, one dependent · Married June 14, 1962</p>`,
				)
				require.NotEqual(t, -1, summary)
				assert.Less(t, heading, summary)
			},
		},
		{
			name:       "fields are grouped as Name, Dates and Contact on every person card",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				card := personCard(t, body, "p_clyd01")
				for _, group := range []string{`aria-label="Name"`, `aria-label="Dates"`, `aria-label="Contact"`} {
					assert.Contains(t, card, group)
				}
			},
		},
		{
			name:       "a deceased person's card says so and still shows every field",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				card := personCard(t, body, "p_carl01")
				assert.Contains(t, card, "In memory, 1963–1981")
				assert.Contains(t, card, `name="person.p_carl01.phone"`, "ADR-0010: the editor never masks")
			},
		},
		{
			name:       "structural controls live in a closed disclosure on a GET",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				card := personCard(t, body, "p_carl01")
				details := strings.Index(card, `<details class="more">`)
				require.NotEqual(t, -1, details, "closed when nothing is ticked")
				assert.Less(t, details, strings.Index(card, `name="person.p_carl01.promote"`))
				assert.Less(t, details, strings.Index(card, `name="person.p_carl01.remove"`))
			},
		},
		{
			name:       "dependents sit under their own heading and the button says what it does",
			target:     "/h/h_clyde",
			wantStatus: http.StatusOK,
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Children living at home")
				assert.Contains(t, body, ">Save changes</button>")
				assert.Contains(t, body, `<label for="q">Find someone by name</label>`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, tt.wantStatus, rec.Code)
			tt.checkFunc(t, rec.Body.String())
		})
	}
}

// TestTreePaneCollapses covers §4.1's "the tree pane is collapsible", which the
// plan cited the section for but never transcribed into a requirement — the same
// way it dropped §5.4's no-contact-details rule. The state rides in the URL like
// the rest of the navigation state (ADR-0008), so it survives a reload.
func TestTreePaneCollapses(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		checkFunc func(t *testing.T, body string)
	}{
		{
			name:   "the pane is open by default",
			target: "/h/h_clyde",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, `aria-expanded="true"`)
				assert.Contains(t, body, "Aden &amp; Nettie Whitlock", "the tree is rendered")
				assert.NotContains(t, body, "panes-collapsed")
			},
		},
		{
			name:   "pane=closed collapses it and offers to reopen",
			target: "/h/h_clyde?pane=closed",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "panes-collapsed")
				assert.Contains(t, body, `aria-expanded="false"`)
				assert.Contains(t, body, "Show the household list")
				assert.Contains(t, body, "Clyde Whitlock", "the detail pane is unaffected")
			},
		},
		{
			name:   "the toggle keeps the Editor on the same Household",
			target: "/h/h_clyde",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, `href="/h/h_clyde?pane=closed"`,
					"collapsing must not lose the selection")
			},
		},
		{
			name:   "closed, the toggle links back without the parameter",
			target: "/h/h_clyde?pane=closed",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, `href="/h/h_clyde"`)
			},
		},
		{
			name:   "an unrecognised pane value leaves the pane open",
			target: "/h/h_clyde?pane=banana",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "panes-collapsed",
					"a typo in a bookmark must not hide the primary interface")
			},
		},
		{
			name:   "the root page collapses too, without a Household",
			target: "/?pane=closed",
			checkFunc: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "panes-collapsed")
				assert.Contains(t, body, `href="/"`, "the reopen link works with no selection")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, http.StatusOK, rec.Code)
			tt.checkFunc(t, rec.Body.String())
		})
	}
}

func TestFindingsRenderOnTheirCard(t *testing.T) {
	doc := sampleDocument()
	// h_clyde is index 1. A malformed email gives Clyde a Finding; an
	// anniversary before Doris (1940) was born gives the Household one.
	doc.Households[1].Adults[0].Email = "clyde-at-example"
	doc.Households[1].Anniversary = rolo.Date{Year: 1930}

	rec := get(t, doc, "/h/h_clyde")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "a person's finding is on their card",
			checkFunc: func(t *testing.T) {
				t.Helper()
				assert.Contains(t, personCard(t, body, "p_clyd01"), "does not look like an email address")
			},
		},
		{
			name: "a household finding is on the household card",
			checkFunc: func(t *testing.T) {
				t.Helper()
				start := strings.Index(body, `data-card="household"`)
				require.NotEqual(t, -1, start)
				card := body[start : start+strings.Index(body[start:], "</fieldset>")]
				assert.Contains(t, card, "the anniversary 1930 is before")
			},
		},
		{
			name: "no finding is shown twice",
			checkFunc: func(t *testing.T) {
				t.Helper()
				assert.Equal(t, 1, strings.Count(body, "does not look like an email address"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.checkFunc(t) })
	}
}
