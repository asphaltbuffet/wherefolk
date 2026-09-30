package web_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/config"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/internal/web"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// newTrashServer serves sampleDocument with Clyde/Doris deleted on September 1
// and Dave, beneath them, on September 20 — so restoring Dave must bring his
// parents back with him.
func newTrashServer(t *testing.T, saver *recordingSaver) *web.Server {
	t.Helper()

	doc := sampleDocument()
	take := func(id rolo.HouseholdID) rolo.Household {
		i := slices.IndexFunc(doc.Households, func(h rolo.Household) bool { return h.ID == id })
		require.GreaterOrEqual(t, i, 0)
		h := doc.Households[i]
		doc.Households = slices.Delete(doc.Households, i, i+1)
		return h
	}

	clyde, dave := take("h_clyde"), take("h_dave")

	trash := &store.Trash{
		Schema: store.CurrentTrashSchema,
		Entries: []store.TrashEntry{
			{
				DeletedAt: time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC),
				Path:      "Aden/Nettie › Clyde/Doris",
				Household: clyde,
			},
			{
				DeletedAt: time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC),
				Path:      "Aden/Nettie › Clyde/Doris › Dave",
				Household: dave,
			},
		},
	}

	nextID := sequentialIDs("h_new")
	srv, err := web.New(store.State{Document: doc, Trash: trash}, config.Config{}, testLogger(), web.Meta{},
		saver.save,
		func() (rolo.HouseholdID, error) { return rolo.HouseholdID(nextID()), nil },
		func() (rolo.PersonID, error) { return "p_x", nil },
		&fakeExporter{}, testClock,
	)
	require.NoError(t, err)

	return srv
}

func TestTrashPage(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, srv *web.Server)
	}{
		{
			name: "Recently deleted lists entries newest first, each with a Restore",
			check: func(t *testing.T, srv *web.Server) {
				t.Helper()
				rec := fetch(t, srv, "/trash")
				require.Equal(t, http.StatusOK, rec.Code)

				body := rec.Body.String()
				assert.Less(t, strings.Index(body, "Dave Whitlock"), strings.Index(body, "Clyde &amp; Doris"))
				assert.Contains(t, body, "Aden/Nettie › Clyde/Doris › Dave")
				assert.Contains(t, body, "kept until October 20")
				assert.Contains(t, body, `action="/trash/h_dave/restore"`)
			},
		},
		{
			name: "the tree pane links to Recently deleted with a count",
			check: func(t *testing.T, srv *web.Server) {
				t.Helper()
				assert.Contains(t, fetch(t, srv, "/").Body.String(), "Recently deleted (2)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, newTrashServer(t, &recordingSaver{}))
		})
	}
}

// newDatedTrashServer serves sampleDocument with a Household whose own 30
// days are over but which a newer entry needs, and a separate Household that
// is expired and unanchored.
func newDatedTrashServer(t *testing.T) *web.Server {
	t.Helper()

	doc := sampleDocument()
	take := func(id rolo.HouseholdID) rolo.Household {
		i := slices.IndexFunc(doc.Households, func(h rolo.Household) bool { return h.ID == id })
		require.GreaterOrEqual(t, i, 0)
		h := doc.Households[i]
		doc.Households = slices.Delete(doc.Households, i, i+1)
		return h
	}

	clyde, dave := take("h_clyde"), take("h_dave")

	// testClock is September 25, 2026. Clyde was deleted August 1 (his own 30
	// days are long over) but Dave, beneath him, was deleted September 10
	// (his 30 days end October 10) so Clyde is anchored until then too.
	// A separate, unrelated Household was deleted August 1 as well and its 30
	// days are also over, with nothing anchoring it.
	trash := &store.Trash{
		Schema: store.CurrentTrashSchema,
		Entries: []store.TrashEntry{
			{
				DeletedAt: time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC),
				Path:      "Aden/Nettie › Clyde/Doris",
				Household: clyde,
			},
			{
				DeletedAt: time.Date(2026, time.September, 10, 10, 0, 0, 0, time.UTC),
				Path:      "Aden/Nettie › Clyde/Doris › Dave",
				Household: dave,
			},
			{
				DeletedAt: time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC),
				Path:      "Reeve",
				Household: rolo.Household{
					ID:     "h_unanchored",
					Adults: []rolo.Person{{ID: "p_nobody", Given: "Norma", Surname: "Nobody"}},
				},
			},
		},
	}

	nextID := sequentialIDs("h_new")
	srv, err := web.New(store.State{Document: doc, Trash: trash}, config.Config{}, testLogger(), web.Meta{},
		(&recordingSaver{}).save,
		func() (rolo.HouseholdID, error) { return rolo.HouseholdID(nextID()), nil },
		func() (rolo.PersonID, error) { return "p_x", nil },
		&fakeExporter{}, testClock,
	)
	require.NoError(t, err)

	return srv
}

func TestRecentlyDeletedDates(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, body string)
	}{
		{
			name: "an entry whose own 30 days are over but which a newer entry needs shows the newer entry's date",
			check: func(t *testing.T, body string) {
				t.Helper()
				assert.Contains(t, body, "Clyde &amp; Doris")
				assert.Contains(t, body, "kept until October 10")
				assert.NotContains(t, body, "kept until August 31")
			},
		},
		{
			name: "an expired, unanchored entry is not listed",
			check: func(t *testing.T, body string) {
				t.Helper()
				assert.NotContains(t, body, "Norma Nobody")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fetch(t, newDatedTrashServer(t), "/trash").Body.String()
			tt.check(t, body)
		})
	}
}

// newCountTrashServer serves sampleDocument with the given Trash entries
// directly, for asserting the tree pane's count against the page's list.
func newCountTrashServer(t *testing.T, entries ...store.TrashEntry) *web.Server {
	t.Helper()

	srv, err := web.New(
		store.State{
			Document: sampleDocument(),
			Trash:    &store.Trash{Schema: store.CurrentTrashSchema, Entries: entries},
		},
		config.Config{},
		testLogger(),
		web.Meta{},
		(&recordingSaver{}).save,
		func() (rolo.HouseholdID, error) { return "h_new001", nil },
		func() (rolo.PersonID, error) { return "p_x", nil },
		&fakeExporter{},
		testClock,
	)
	require.NoError(t, err)

	return srv
}

func TestTrashCountMatchesPage(t *testing.T) {
	// testClock is September 25, 2026.
	live := store.TrashEntry{
		DeletedAt: time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC),
		Household: rolo.Household{
			ID:     "h_live",
			Adults: []rolo.Person{{ID: "p_live", Given: "Liv", Surname: "Ongoing"}},
		},
	}
	expiredUnanchored := store.TrashEntry{
		DeletedAt: time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC),
		Household: rolo.Household{
			ID:     "h_unanchored",
			Adults: []rolo.Person{{ID: "p_nobody", Given: "Norma", Surname: "Nobody"}},
		},
	}

	tests := []struct {
		name    string
		entries []store.TrashEntry
		want    string
		notWant string
	}{
		{
			name:    "an expired unanchored entry is not counted alongside a live one",
			entries: []store.TrashEntry{live, expiredUnanchored},
			want:    "Recently deleted (1)",
		},
		{
			name:    "an expired unanchored entry alone shows no link at all",
			entries: []store.TrashEntry{expiredUnanchored},
			notWant: "Recently deleted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fetch(t, newCountTrashServer(t, tt.entries...), "/").Body.String()

			if tt.want != "" {
				assert.Contains(t, body, tt.want)
			}
			if tt.notWant != "" {
				assert.NotContains(t, body, tt.notWant)
			}
		})
	}
}

func TestEmptyTrash(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		want    string
		notWant string
	}{
		{name: "no link when nothing is deleted", target: "/", notWant: "Recently deleted"},
		{name: "the page says so plainly", target: "/trash", want: "Nothing has been deleted recently."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fetch(t, newTestServer(t, sampleDocument(), nil), tt.target).Body.String()
			if tt.want != "" {
				assert.Contains(t, body, tt.want)
			}
			if tt.notWant != "" {
				assert.NotContains(t, body, tt.notWant)
			}
		})
	}
}

func TestRestore(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, srv *web.Server, saver *recordingSaver)
	}{
		{
			name: "restoring brings back the Households it needs and lands on it",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				loc := location(t, postTo(t, srv, "/trash/h_dave/restore", nil))

				assert.Equal(t, "/h/h_dave", loc.Path)
				assert.Contains(t, loc.Query().Get("said"), "Dave Whitlock was restored, along with Clyde & Doris")
				assert.NotEmpty(t, loc.Query().Get("undo"))

				assert.True(t, hasHousehold(saver.saved, "h_dave"))
				assert.True(t, hasHousehold(saver.saved, "h_clyde"))
				assert.Empty(t, saver.savedTrash.Entries)
			},
		},
		{
			name: "restoring a parent leaves its child in the Trash",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				_ = location(t, postTo(t, srv, "/trash/h_clyde/restore", nil))

				assert.True(t, hasHousehold(saver.saved, "h_clyde"))
				require.Len(t, saver.savedTrash.Entries, 1)
				assert.Equal(t, rolo.HouseholdID("h_dave"), saver.savedTrash.Entries[0].Household.ID)
			},
		},
		{
			name: "restoring what is not there says so",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				rec := postTo(t, srv, "/trash/h_nope/restore", nil)

				assert.Equal(t, http.StatusNotFound, rec.Code)
				assert.Contains(t, rec.Body.String(), "no longer in Recently deleted")
				assert.Zero(t, saver.calls)
			},
		},
		{
			name: "Undo after a restore puts both back in the Trash",
			check: func(t *testing.T, srv *web.Server, saver *recordingSaver) {
				t.Helper()
				loc := location(t, postTo(t, srv, "/trash/h_dave/restore", nil))

				back := location(t, undo(t, srv, loc, "h_dave"))
				assert.Equal(t, "/", back.Path, "the page the Editor was on no longer exists")

				assert.False(t, hasHousehold(saver.saved, "h_dave"))
				assert.Len(t, saver.savedTrash.Entries, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saver := &recordingSaver{}
			tt.check(t, newTrashServer(t, saver), saver)
		})
	}
}
