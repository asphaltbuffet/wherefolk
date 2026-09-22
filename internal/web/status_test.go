package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/version"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// sampleDocument mirrors the fixture in internal/store: one two-level Branch
// and one standalone root.
func sampleDocument() *store.Document {
	adult := func(id rolo.PersonID, given string, birthYear int) rolo.Person {
		return rolo.Person{ID: id, Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
	}

	return &store.Document{
		Schema: store.CurrentSchema,
		Households: []rolo.Household{
			{
				ID:     "h_aden",
				Adults: []rolo.Person{adult("p_aden01", "Aden", 1910), adult("p_nett01", "Nettie", 1912)},
			},
			{
				ID:     "h_clyde",
				Parent: "h_aden",
				Adults: []rolo.Person{adult("p_clyd01", "Clyde", 1938), adult("p_dori01", "Doris", 1940)},
				Dependents: []rolo.Person{
					{ID: "p_carl01", Given: "Carl", Surname: "Whitlock", Birth: rolo.Date{Year: 1963}},
				},
			},
			{
				ID:     "h_reeve",
				Adults: []rolo.Person{adult("p_reev01", "Ray", 1942)},
			},
		},
	}
}

// get issues a request against the server and returns the recorder.
func get(t *testing.T, doc *store.Document, target string) *httptest.ResponseRecorder {
	t.Helper()

	srv, err := web.New(doc, web.Meta{DocumentPath: "/tmp/test/directory.json"})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

	return rec
}

func TestStatusPage(t *testing.T) {
	tests := []struct {
		name   string
		target string
		check  func(t *testing.T, body string)
	}{
		{
			name:   "reports the build version",
			target: "/status",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body,
					`data-field="version">`+version.ShortVersion()+`<`,
					"the running build identifies itself")
			},
		},
		{
			name:   "omits build metadata when nothing was injected",
			target: "/status",
			check: func(t *testing.T, body string) {
				if version.BuildInfo() != "" {
					t.Skip("build metadata was injected via ldflags")
				}

				assert.NotContains(t, body, `data-field="build"`,
					"an un-injected build renders no build row at all")
			},
		},
		{
			name:   "reports the household count",
			target: "/status",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body, `data-field="households">3<`, "three households in the fixture")
			},
		},
		{
			name:   "reports the person count",
			target: "/status",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body, `data-field="people">6<`, "five adults plus one dependent")
			},
		},
		{
			name:   "reports the root count",
			target: "/status",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body, `data-field="roots">2<`, "Aden and Ray are roots; Clyde is not")
			},
		},
		{
			name:   "reports the schema version",
			target: "/status",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body, "Schema version")
				assert.Contains(t, body, `data-field="schema">1<`)
			},
		},
		{
			name:   "lists root household labels",
			target: "/status",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body, "Aden/Nettie")
				assert.Contains(t, body, "Ray")
				assert.NotContains(t, body, "Clyde/Doris", "Clyde is a child, not a root")
			},
		},
		{
			name:   "root path serves the same page until item 4 claims it",
			target: "/",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body, "Wherefolk status")
			},
		},
		{
			name:   "reports the document path",
			target: "/status",
			check: func(t *testing.T, body string) {
				assert.Contains(t, body, `data-field="document">/tmp/test/directory.json<`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, sampleDocument(), tt.target)

			require.Equal(t, http.StatusOK, rec.Code)
			tt.check(t, rec.Body.String())
		})
	}
}
