package render_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func yoderSheet(t *testing.T, reply string, extra ...rolo.Person) render.ProofSheet {
	t.Helper()

	h := yoders()
	h.Dependents = append(h.Dependents, extra...)

	s, err := render.BuildProof(proofTree(t, h), "h_yod", proofAsOf, "The Yoder Directory", reply)
	require.NoError(t, err)

	return s
}

func TestProofMarkup(t *testing.T) {
	tests := []struct {
		name      string
		reply     string
		checkFunc func(t *testing.T, markup string)
	}{
		{
			name: "imports proof and names the household",
			checkFunc: func(t *testing.T, m string) {
				require.Contains(t, m, `#import "directory.typ": proof`)
				require.Contains(t, m, `name: "Daryl & Dawn (Mitchell) Yoder"`)
				require.Contains(t, m, `title: "The Yoder Directory"`)
			},
		},
		{
			name: "values never reach Typst",
			checkFunc: func(t *testing.T, m string) {
				require.NotContains(t, m, "(555) 201-4433")
				require.NotContains(t, m, "12 Elm St")
				require.Contains(t, m, `(id: "p_dar-phone", withheld: true)`)
			},
		},
		{
			name:  "reply escapes and keeps line breaks",
			reply: "Mail \"Dawn\" at C:\\box\n#1 Elm St",
			checkFunc: func(t *testing.T, m string) {
				require.Contains(t, m, `reply: "Mail \"Dawn\" at C:\\box\n#1 Elm St"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, render.ProofMarkup(yoderSheet(t, tt.reply)))
		})
	}
}

type proofMark struct {
	ID   string  `json:"id"`
	Kind string  `json:"kind"`
	Page int     `json:"page"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	PH   float64 `json:"ph"`
}

// proofMarks compiles the sheet against the on-disk template and returns every
// field marker typst reports.
func proofMarks(t *testing.T, bin string, s render.ProofSheet) []proofMark {
	t.Helper()

	dir := t.TempDir()

	tmpl, err := os.ReadFile(filepath.Join(templateDir, render.TemplateName))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, render.TemplateName), tmpl, 0o600))

	src := filepath.Join(dir, "main.typ")
	require.NoError(t, os.WriteFile(src, []byte(render.ProofMarkup(s)), 0o600))

	// stdout only: typst 0.15 warns on stderr that query is deprecated.
	out, err := exec.CommandContext(t.Context(), bin, "query", "--root", dir, src,
		"<proof-field>", "--field", "value", "--format", "json").Output()
	require.NoError(t, err)

	var marks []proofMark
	require.NoError(t, json.Unmarshal(out, &marks))

	return marks
}

func TestProofTemplatePlacesEveryField(t *testing.T) {
	typst := requireTypst(t)

	kids := make([]rolo.Person, 0, 6)
	for i := range 6 {
		kids = append(kids, rolo.Person{ID: rolo.PersonID("p_k" + strconv.Itoa(i)), Given: "Kid", Surname: "Yoder"})
	}

	ordinary := yoderSheet(t, "Send to Dawn")
	ordinaryPages := 0

	for _, m := range proofMarks(t, typst.Bin, ordinary) {
		ordinaryPages = max(ordinaryPages, m.Page)
	}

	tests := []struct {
		name         string
		sheet        render.ProofSheet
		minLastPage  int
		wantMorePage bool
	}{
		{name: "ordinary household", sheet: ordinary, minLastPage: 1},
		{name: "six dependents spill to a later page", sheet: yoderSheet(t, "", kids...), minLastPage: 2,
			wantMorePage: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marks := proofMarks(t, typst.Bin, tt.sheet)
			texts, checks := tt.sheet.FormValues()

			placed := map[string]string{}
			personPages := map[string]map[int]bool{}
			maxPage := 0

			for _, m := range marks {
				require.NotContains(t, placed, m.ID, "placed twice")
				placed[m.ID] = m.Kind
				maxPage = max(maxPage, m.Page)

				// A person's fields share their ID prefix; their block never
				// splits, so they must all be on one page.
				if who, _, ok := strings.Cut(m.ID, "-"); ok && (strings.HasPrefix(who, "p_") ||
					strings.HasPrefix(who, "missing")) {
					if personPages[who] == nil {
						personPages[who] = map[int]bool{}
					}

					personPages[who][m.Page] = true
				}

				require.Positive(t, m.W, m.ID)
				require.Positive(t, m.H, m.ID)
				require.GreaterOrEqual(t, m.X, 0.0, m.ID)
				require.LessOrEqual(t, m.X+m.W, 612.0, m.ID)
				require.GreaterOrEqual(t, m.Y, 0.0, m.ID)
				require.LessOrEqual(t, m.Y+m.H, m.PH, m.ID)
			}

			require.Len(t, placed, len(texts)+len(checks))

			for id := range texts {
				require.Contains(t, []string{"text", "multiline"}, placed[id], id)
			}

			for id := range checks {
				require.Equal(t, "check", placed[id], id)
			}

			require.Equal(t, "multiline", placed["notes"])

			for who, pages := range personPages {
				require.Len(t, pages, 1, "%s split across pages", who)
			}

			require.GreaterOrEqual(t, maxPage, tt.minLastPage)

			if tt.wantMorePage {
				require.Greater(t, maxPage, ordinaryPages)
			}
		})
	}
}
