package live_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/live"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// sampleDocument is three Households: h_aden, h_clyde beneath it, and h_carla
// beneath h_clyde, sharing its Address. Only h_carla can be deleted.
func sampleDocument() *store.Document {
	adult := func(id rolo.PersonID, given string, birthYear int) rolo.Person {
		return rolo.Person{ID: id, Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
	}

	return &store.Document{
		Schema: store.CurrentSchema,
		Households: []rolo.Household{
			{ID: "h_aden", Adults: []rolo.Person{adult("p_aden01", "Aden", 1910)}},
			{
				ID:      "h_clyde",
				Parent:  "h_aden",
				Adults:  []rolo.Person{adult("p_clyd01", "Clyde", 1938)},
				Address: rolo.Address{Lines: []string{"88 Oakwood Drive"}},
			},
			{
				ID:      "h_carla",
				Parent:  "h_clyde",
				Adults:  []rolo.Person{adult("p_carla01", "Carla", 1965)},
				Address: rolo.Address{SharedWith: "h_clyde"},
			},
		},
	}
}

// orphanDocument does not build a tree: its only Household names a parent
// that is not there.
func orphanDocument() *store.Document {
	return &store.Document{
		Schema: store.CurrentSchema,
		Households: []rolo.Household{{
			ID:     "h_orphan",
			Parent: "h_missing",
			Adults: []rolo.Person{{ID: "p_x", Given: "X", Surname: "Y"}},
		}},
	}
}

func day(n int) time.Time {
	return time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n)
}

// expiredTrash holds one standalone Household deleted forty days before day(0),
// so a purge at day(0) removes it.
func expiredTrash() *store.Trash {
	return &store.Trash{
		Schema: store.CurrentTrashSchema,
		Entries: []store.TrashEntry{{
			DeletedAt: day(-40),
			Household: rolo.Household{ID: "h_gone", Adults: []rolo.Person{{ID: "p_gone", Given: "Gone"}}},
		}},
	}
}

func trashIDs(t *store.Trash) []rolo.HouseholdID {
	out := make([]rolo.HouseholdID, 0, len(t.Entries))
	for _, e := range t.Entries {
		out = append(out, e.Household.ID)
	}

	return out
}

type write struct{ prev, next store.State }

// recordingWriter records every write it is asked for, and fails them all
// while err is set.
type recordingWriter struct {
	writes []write
	err    error
}

func (r *recordingWriter) Write(prev, next store.State) error {
	if r.err != nil {
		return r.err
	}

	r.writes = append(r.writes, write{prev: prev, next: next})

	return nil
}

func fixedClock(at time.Time) live.Clock { return func() time.Time { return at } }

func newCopy(t *testing.T, initial store.State, w live.Writer) *live.Copy {
	t.Helper()

	c, err := live.New(initial, w, fixedClock(day(0)))
	require.NoError(t, err)

	return c
}

// retitle is a build that changes only the Directory Title.
func retitle(title string) func(live.Snapshot) (store.State, error) {
	return func(s live.Snapshot) (store.State, error) {
		return store.State{Settings: s.Settings.WithTitle(title)}, nil
	}
}

// deleteCarla is a build that moves h_carla to the Trash.
func deleteCarla(s live.Snapshot) (store.State, error) {
	carla, _ := s.Tree.Get("h_carla")

	doc, trash, err := store.Delete(s.Document, s.State().Trash,
		store.TrashEntry{DeletedAt: s.Now, Household: carla})

	return store.State{Document: doc, Trash: trash}, err
}

// renameCarla is a build that changes only the document.
func renameCarla(s live.Snapshot) (store.State, error) {
	doc := &store.Document{Schema: s.Document.Schema, Households: slices.Clone(s.Document.Households)}
	doc.Households[2].Adults = []rolo.Person{{ID: "p_carla01", Given: "Carly", Surname: "Whitlock"}}

	return store.State{Document: doc}, nil
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		initial store.State
		writer  live.Writer
		clock   live.Clock
		wantErr error // nil: New succeeds
		anyErr  bool
	}{
		{
			name:    "a document alone starts with an empty Trash and an untitled Directory",
			initial: store.State{Document: sampleDocument()},
			writer:  &recordingWriter{},
			clock:   fixedClock(day(0)),
		},
		{
			name:    "a nil document is refused",
			initial: store.State{},
			writer:  &recordingWriter{},
			clock:   fixedClock(day(0)),
			anyErr:  true,
		},
		{
			name:    "a nil writer is refused",
			initial: store.State{Document: sampleDocument()},
			clock:   fixedClock(day(0)),
			anyErr:  true,
		},
		{
			name:    "a nil clock is refused",
			initial: store.State{Document: sampleDocument()},
			writer:  &recordingWriter{},
			anyErr:  true,
		},
		{
			name:    "a document that does not build a tree is refused",
			initial: store.State{Document: orphanDocument()},
			writer:  &recordingWriter{},
			clock:   fixedClock(day(0)),
			wantErr: live.ErrUnbuildable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := live.New(tt.initial, tt.writer, tt.clock)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.anyErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
				snap := c.Snapshot()
				require.NotNil(t, snap.Trash)
				assert.Empty(t, snap.Trash.Entries)
				require.NotNil(t, snap.Settings)
				assert.Empty(t, snap.Settings.Title)
			}
		})
	}
}

func TestSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "the Trash is purged as of the clock, and nothing is written",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument(), Trash: expiredTrash()}, w)

				snap := c.Snapshot()

				assert.Empty(t, snap.Trash.Entries, "an expired entry is not listed or counted")
				assert.Equal(t, []rolo.HouseholdID{"h_gone"}, trashIDs(snap.State().Trash),
					"it is still stored until the next Trash write")
				assert.Empty(t, w.writes)
			},
		},
		{
			name: "nothing is undoable before the first save",
			checkFunc: func(t *testing.T) {
				t.Helper()
				snap := newCopy(t, store.State{Document: sampleDocument()}, &recordingWriter{}).Snapshot()

				assert.False(t, snap.CanUndo(""))
				assert.False(t, snap.CanUndo("anything"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.checkFunc)
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "a change is written, served and undoable",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				before := c.Snapshot()

				saved, err := c.Update(retitle("The Whitlocks"))
				require.NoError(t, err)

				assert.True(t, saved.Changed)
				assert.NotEmpty(t, saved.Undo)
				require.Len(t, w.writes, 1)
				assert.Same(t, before.Settings, w.writes[0].prev.Settings, "the writer is told what it replaces")
				assert.Equal(t, "The Whitlocks", c.Snapshot().Settings.Title)
				assert.Same(t, saved.Snapshot.Settings, c.Snapshot().Settings)
				assert.True(t, c.Snapshot().CanUndo(saved.Undo))
			},
		},
		{
			name: "an unchanged state is not a save, and the previous Undo stands",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				first, err := c.Update(retitle("The Whitlocks"))
				require.NoError(t, err)

				saved, err := c.Update(func(s live.Snapshot) (store.State, error) { return s.State(), nil })
				require.NoError(t, err)

				assert.False(t, saved.Changed)
				assert.Empty(t, saved.Undo)
				assert.Len(t, w.writes, 1)
				assert.True(t, c.Snapshot().CanUndo(first.Undo))
			},
		},
		{
			name: "a field left nil is left as served",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				before := c.Snapshot()

				_, err := c.Update(retitle("The Whitlocks"))
				require.NoError(t, err)

				require.Len(t, w.writes, 1)
				assert.Same(t, before.Document, w.writes[0].next.Document)
				assert.Same(t, before.State().Trash, w.writes[0].next.Trash)
			},
		},
		{
			name: "handing back the effective Trash is not a Trash change",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				stored := expiredTrash()
				c := newCopy(t, store.State{Document: sampleDocument(), Trash: stored}, w)

				_, err := c.Update(func(s live.Snapshot) (store.State, error) {
					return store.State{Document: s.Document, Trash: s.Trash, Settings: s.Settings.WithTitle("T")}, nil
				})
				require.NoError(t, err)

				require.Len(t, w.writes, 1)
				assert.Same(t, stored, w.writes[0].next.Trash, "neither Trash pointer means a change")
			},
		},
		{
			name: "a refused build writes nothing",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				before := c.Snapshot()
				errRefused := errors.New("refused")

				_, err := c.Update(func(live.Snapshot) (store.State, error) {
					return store.State{Document: orphanDocument()}, errRefused
				})

				require.ErrorIs(t, err, errRefused)
				assert.Empty(t, w.writes)
				assert.Same(t, before.Document, c.Snapshot().Document)
			},
		},
		{
			name: "a failed write leaves the served state and its Undo",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				first, err := c.Update(retitle("The Whitlocks"))
				require.NoError(t, err)

				diskFull := errors.New("disk full")
				w.err = diskFull
				saved, err := c.Update(retitle("Something Else"))

				require.ErrorIs(t, err, diskFull)
				assert.False(t, saved.Changed)
				assert.Empty(t, saved.Undo)
				assert.Equal(t, "The Whitlocks", c.Snapshot().Settings.Title)
				assert.True(t, c.Snapshot().CanUndo(first.Undo))
			},
		},
		{
			name: "a document that does not build a tree is not written",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				before := c.Snapshot()

				saved, err := c.Update(func(live.Snapshot) (store.State, error) {
					return store.State{Document: orphanDocument()}, nil
				})

				require.ErrorIs(t, err, live.ErrUnbuildable)
				assert.False(t, saved.Changed)
				assert.Empty(t, saved.Undo)
				assert.Empty(t, w.writes)
				assert.Same(t, before.Document, c.Snapshot().Document)
			},
		},
		{
			name: "the Trash is purged when a write changes it",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument(), Trash: expiredTrash()}, w)

				_, err := c.Update(deleteCarla)
				require.NoError(t, err)

				require.Len(t, w.writes, 1)
				assert.Equal(t, []rolo.HouseholdID{"h_carla"}, trashIDs(w.writes[0].next.Trash))
			},
		},
		{
			name: "the Trash is not purged when only the document changes",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				stored := expiredTrash()
				c := newCopy(t, store.State{Document: sampleDocument(), Trash: stored}, w)
				before := c.Snapshot()

				_, err := c.Update(renameCarla)
				require.NoError(t, err)

				require.Len(t, w.writes, 1)
				assert.Same(t, stored, w.writes[0].next.Trash)
				assert.Same(t, before.Settings, w.writes[0].next.Settings,
					"a write that leaves the settings alone keeps the served ones")
			},
		},
		{
			name: "one clock reading dates the whole write",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				calls := 0
				clock := func() time.Time {
					calls++
					return day(calls)
				}
				c, err := live.New(store.State{Document: sampleDocument()}, w, clock)
				require.NoError(t, err)

				_, err = c.Update(deleteCarla)
				require.NoError(t, err)

				assert.Equal(t, 1, calls)
				require.Len(t, w.writes, 1)
				assert.Equal(t, day(1), w.writes[0].next.Trash.Entries[0].DeletedAt)
			},
		},
		{
			name: "readers see a whole state while a writer works",
			checkFunc: func(t *testing.T) {
				t.Helper()
				c := newCopy(t, store.State{Document: sampleDocument()}, &recordingWriter{})

				var wg sync.WaitGroup
				for range 8 {
					wg.Go(func() {
						for range 200 {
							s := c.Snapshot()
							carla, ok := s.Tree.Get("h_carla")
							assert.True(t, ok)
							assert.NotNil(t, s.Settings)
							assert.Equal(t, s.Document.Households[2].Adults[0].Given, carla.Adults[0].Given,
								"Tree and Document come from one generation")
						}
					})
				}

				for i := range 50 {
					var err error
					if i%2 == 0 {
						_, err = c.Update(retitle(string(rune('A' + i%26))))
					} else {
						given := fmt.Sprintf("Carla%d", i)
						_, err = c.Update(func(s live.Snapshot) (store.State, error) {
							doc := &store.Document{
								Schema:     s.Document.Schema,
								Households: slices.Clone(s.Document.Households),
							}
							doc.Households[2].Adults = []rolo.Person{
								{ID: "p_carla01", Given: given, Surname: "Whitlock"},
							}

							return store.State{Document: doc}, nil
						})
					}
					require.NoError(t, err)
				}

				wg.Wait()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.checkFunc)
	}
}

func TestUndo(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "Undo returns to the state before the latest save",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				original := c.Snapshot().Settings
				saved, err := c.Update(retitle("The Whitlocks"))
				require.NoError(t, err)

				snap, undone, err := c.Undo(saved.Undo)
				require.NoError(t, err)

				assert.True(t, undone)
				assert.Same(t, original, snap.Settings)
				assert.Same(t, original, c.Snapshot().Settings)
				require.Len(t, w.writes, 2)
				assert.Same(t, original, w.writes[1].next.Settings)
			},
		},
		{
			name: "after an Undo there is nothing to undo",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				saved, err := c.Update(retitle("The Whitlocks"))
				require.NoError(t, err)
				_, _, err = c.Undo(saved.Undo)
				require.NoError(t, err)

				snap, undone, err := c.Undo(saved.Undo)
				require.NoError(t, err)

				assert.False(t, undone, "Undo is never a toggle")
				assert.False(t, snap.CanUndo(saved.Undo))
				assert.Len(t, w.writes, 2)
			},
		},
		{
			name: "a token retired by a newer save is refused",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				first, err := c.Update(retitle("One"))
				require.NoError(t, err)
				_, err = c.Update(retitle("Two"))
				require.NoError(t, err)

				_, undone, err := c.Undo(first.Undo)
				require.NoError(t, err)

				assert.False(t, undone)
				assert.Equal(t, "Two", c.Snapshot().Settings.Title)
				assert.Len(t, w.writes, 2)
			},
		},
		{
			name: "an empty token is refused",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)

				_, undone, err := c.Undo("")
				require.NoError(t, err)

				assert.False(t, undone)
				assert.Empty(t, w.writes)
			},
		},
		{
			name: "a failed Undo keeps the Undo",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				c := newCopy(t, store.State{Document: sampleDocument()}, w)
				saved, err := c.Update(retitle("The Whitlocks"))
				require.NoError(t, err)

				diskFull := errors.New("disk full")
				w.err = diskFull
				_, undone, err := c.Undo(saved.Undo)

				require.ErrorIs(t, err, diskFull)
				assert.False(t, undone)
				assert.Equal(t, "The Whitlocks", c.Snapshot().Settings.Title)
				assert.True(t, c.Snapshot().CanUndo(saved.Undo))
			},
		},
		{
			name: "Undo restores the Trash exactly, without purging it",
			checkFunc: func(t *testing.T) {
				t.Helper()
				w := &recordingWriter{}
				stored := expiredTrash()
				c := newCopy(t, store.State{Document: sampleDocument(), Trash: stored}, w)
				saved, err := c.Update(deleteCarla) // also purges h_gone
				require.NoError(t, err)

				_, undone, err := c.Undo(saved.Undo)
				require.NoError(t, err)

				assert.True(t, undone)
				require.Len(t, w.writes, 2)
				assert.Same(t, stored, w.writes[1].next.Trash)
				assert.Equal(t, []rolo.HouseholdID{"h_gone"}, trashIDs(w.writes[1].next.Trash))
			},
		},
		{
			name: "a deletion and its Undo round-trip through the files",
			checkFunc: func(t *testing.T) {
				t.Helper()
				dir := t.TempDir()
				paths := store.Paths{
					Document: filepath.Join(dir, "directory.json"),
					Trash:    filepath.Join(dir, "trash.json"),
					Settings: filepath.Join(dir, "settings.json"),
				}
				require.NoError(t, store.Save(paths.Document, sampleDocument()))

				initial, err := store.Open(paths, day(0))
				require.NoError(t, err)
				c := newCopy(t, initial, paths)

				saved, err := c.Update(deleteCarla)
				require.NoError(t, err)

				onDisk, err := store.Open(paths, day(0))
				require.NoError(t, err)
				assert.Len(t, onDisk.Document.Households, 2)
				assert.Equal(t, []rolo.HouseholdID{"h_carla"}, trashIDs(onDisk.Trash))

				_, undone, err := c.Undo(saved.Undo)
				require.NoError(t, err)
				require.True(t, undone)

				back, err := store.Open(paths, day(0))
				require.NoError(t, err)
				assert.Len(t, back.Document.Households, 3)
				assert.Empty(t, back.Trash.Entries)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.checkFunc)
	}
}
