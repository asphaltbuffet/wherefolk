package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/buildmeta"
	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// sampleDocument mirrors the fixture in internal/store and adds the cases the
// detail pane must render: a withheld phone, a withheld address, a shared
// address, a Memorial Household, an anniversary, and a deceased Dependent.
func sampleDocument() *store.Document {
	adult := func(id rolo.PersonID, given string, birthYear int) rolo.Person {
		return rolo.Person{ID: id, Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
	}

	return &store.Document{
		Schema: store.CurrentSchema,
		Households: []rolo.Household{
			{
				// Memorial: both adults deceased.
				ID: "h_aden",
				Adults: []rolo.Person{
					{
						// Contact details on a deceased adult are deliberate:
						// §5.4 says a Memorial Household publishes none, and a
						// fixture without them cannot tell suppression from
						// absence, so the rule would look covered while going
						// untested.
						ID: "p_aden01", Given: "Aden", Surname: "Whitlock",
						Birth: rolo.Date{Year: 1910, Month: 4, Day: 2},
						Death: rolo.Date{Year: 1989, Month: 11, Day: 17},
						Phone: "555-0100", Email: "aden@example.com",
					},
					{
						ID: "p_nett01", Given: "Nettie", Surname: "Whitlock",
						Birth: rolo.Date{Year: 1912},
						Death: rolo.Date{Year: 1994},
					},
				},
			},
			{
				ID:     "h_clyde",
				Parent: "h_aden",
				Adults: []rolo.Person{
					{
						ID: "p_clyd01", Given: "Clyde", Surname: "Whitlock",
						Birth: rolo.Date{Year: 1938, Month: 6, Day: 1},
						Phone: "555-0142", Email: "clyde@example.com",
					},
					{
						ID: "p_dori01", Given: "Doris", Aka: "Dot", Surname: "Whitlock",
						BirthName: "Kowalski",
						Birth:     rolo.Date{Year: 1940},
						Phone:     "555-0143", Email: "doris@example.com",
						Hidden: rolo.HiddenFields{Email: true},
					},
				},
				Anniversary: rolo.Date{Year: 1962, Month: 6, Day: 14},
				Address:     rolo.Address{Lines: []string{"1412 Oak St", "Springfield, IL 62704"}},
				Dependents: []rolo.Person{
					{
						ID: "p_carl01", Given: "Carl", Surname: "Whitlock",
						Birth: rolo.Date{Year: 1963}, Death: rolo.Date{Year: 1981},
					},
				},
			},
			{
				// Shares its parent's address.
				ID:      "h_dave",
				Parent:  "h_clyde",
				Adults:  []rolo.Person{adult("p_dave01", "Dave", 1971)},
				Address: rolo.Address{SharedWith: "h_clyde"},
			},
			{
				// Standalone root with a withheld address.
				ID: "h_reeve",
				Adults: []rolo.Person{
					{
						ID:      "p_reev01",
						Given:   "Ray",
						Surname: "Reeves",
						Birth:   rolo.Date{Year: 1942},
						Phone:   "555-0199",
						Hidden:  rolo.HiddenFields{Phone: true},
					},
				},
				Address: rolo.Address{Lines: []string{"9 Elm St"}, Hidden: true},
			},
		},
	}
}

// testServer builds a Server with recording stubs for the write path. The
// returned saver captures the last document written, so a test can assert what
// a save produced without touching a filesystem.
//
// attempted records the document passed on every call, success or failure,
// so a test can assert on what save() was given even when it refused it.
// saved records it only when save() succeeded, so a test can assert on what
// the server would go on to serve.
type recordingSaver struct {
	attempted *store.Document
	saved     *store.Document
	err       error
	calls     int
}

func (r *recordingSaver) save(doc *store.Document) error {
	r.calls++
	r.attempted = doc
	if r.err != nil {
		return r.err
	}
	r.saved = doc
	return nil
}

// sequentialIDs hands out predictable identities: h_new001, h_new002, and so on.
func sequentialIDs(prefix string) func() string {
	var n int
	return func() string {
		n++
		return fmt.Sprintf("%s%03d", prefix, n)
	}
}

// newTestServer builds a Server with deterministic ID generation. saver may be
// nil, in which case writes succeed and are discarded.
func newTestServer(t *testing.T, doc *store.Document, saver *recordingSaver) *web.Server {
	t.Helper()

	if saver == nil {
		saver = &recordingSaver{}
	}

	nextHousehold := sequentialIDs("h_new")
	nextPerson := sequentialIDs("p_new")

	srv, err := web.New(doc, config.Config{}, testLogger(),
		web.Meta{DocumentPath: "/tmp/test/directory.json"},
		saver.save,
		func() (rolo.HouseholdID, error) { return rolo.HouseholdID(nextHousehold()), nil },
		func() (rolo.PersonID, error) { return rolo.PersonID(nextPerson()), nil },
	)
	require.NoError(t, err)

	return srv
}

// get issues a request against the server and returns the recorder.
func get(t *testing.T, doc *store.Document, target string) *httptest.ResponseRecorder {
	t.Helper()

	nextHousehold := sequentialIDs("h_new")
	nextPerson := sequentialIDs("p_new")

	srv, err := web.New(doc, config.Config{}, testLogger(),
		web.Meta{
			DocumentPath: "/tmp/test/directory.json",
			TypstVersion: "typst 0.13.1 (test)",
			TemplatePath: "/srv/template",
		},
		func(*store.Document) error { return nil },
		func() (rolo.HouseholdID, error) { return rolo.HouseholdID(nextHousehold()), nil },
		func() (rolo.PersonID, error) { return rolo.PersonID(nextPerson()), nil },
	)
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
				t.Helper()

				assert.Contains(t, body,
					`data-field="version">`+buildmeta.ShortVersion()+`<`,
					"the running build identifies itself")
			},
		},
		{
			name:   "omits build metadata when nothing was injected",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				if buildmeta.BuildInfo() != "" {
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
				t.Helper()

				assert.Contains(t, body, `data-field="households">4<`, "four households in the fixture")
			},
		},
		{
			name:   "reports the person count",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				assert.Contains(t, body, `data-field="people">7<`, "six adults plus one dependent")
			},
		},
		{
			name:   "reports the root count",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				assert.Contains(t, body, `data-field="roots">2<`, "Aden and Ray are roots; Clyde is not")
			},
		},
		{
			name:   "reports the schema version",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				assert.Contains(t, body, "Schema version")
				assert.Contains(t, body, `data-field="schema">1<`)
			},
		},
		{
			name:   "lists root household labels",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				assert.Contains(t, body, "Aden/Nettie")
				assert.Contains(t, body, "Ray")
				assert.NotContains(t, body, "Clyde/Doris", "Clyde is a child, not a root")
			},
		},
		{
			name:   "reports the document path",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				assert.Contains(t, body, `data-field="document">/tmp/test/directory.json<`)
			},
		},
		{
			name:   "reports the typst version",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				assert.Contains(t, body,
					`data-field="typst">typst 0.13.1 (test)<`,
					"the operator can see the renderer was found (§2.4)")
			},
		},
		{
			name:   "reports the template path",
			target: "/status",
			check: func(t *testing.T, body string) {
				t.Helper()

				assert.Contains(t, body,
					`data-field="template">/srv/template<`,
					"the operator needs to know which template is in force (ADR-0004)")
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

func TestStatusReportsPassphrase(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{name: "unset", cfg: config.Config{}, want: `data-field="passphrase">not set`},
		{
			name: "set",
			cfg:  config.Config{FullPassphrase: "correct horse battery"},
			want: `data-field="passphrase">set<`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := web.New(sampleDocument(), tt.cfg, testLogger(), web.Meta{},
				func(*store.Document) error { return nil },
				func() (rolo.HouseholdID, error) { return "h_x", nil },
				func() (rolo.PersonID, error) { return "p_x", nil },
			)
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))

			assert.Contains(t, rec.Body.String(), tt.want)
			assert.NotContains(t, rec.Body.String(), "correct horse battery", "/status reports whether, never what")
		})
	}
}

// getHTMX issues a request the way htmx does, with the HX-Request header set.
// Handlers that serve both audiences branch on it, so a test asserting on a
// fragment must say which one it is simulating rather than relying on the
// handler not distinguishing them.
func getHTMX(t *testing.T, doc *store.Document, target string) *httptest.ResponseRecorder {
	t.Helper()

	nextHousehold := sequentialIDs("h_new")
	nextPerson := sequentialIDs("p_new")

	srv, err := web.New(doc, config.Config{}, testLogger(), web.Meta{DocumentPath: "/tmp/test/directory.json"},
		func(*store.Document) error { return nil },
		func() (rolo.HouseholdID, error) { return rolo.HouseholdID(nextHousehold()), nil },
		func() (rolo.PersonID, error) { return rolo.PersonID(nextPerson()), nil },
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Hx-Request", "true")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	return rec
}
