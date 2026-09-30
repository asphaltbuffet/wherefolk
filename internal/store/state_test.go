package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// TestPersistOrder proves the add-before-remove rule (ADR-0012) by making one
// of the two writes fail and observing whether the other already happened.
func TestPersistOrder(t *testing.T) {
	doc := sampleDocument()
	empty := store.NewTrash()
	full := sampleTrash()

	// withoutCarla is doc missing h_carla, standing in for the document just
	// after a deletion (or just before a restore) of that Household.
	withoutCarla := func() *store.Document {
		d := sampleDocument()
		d.Households = d.Households[:2]
		return d
	}

	// docWithX/docWithoutX and trash entries for h_x and h_p model an undo
	// that reaches back past a purge: deleting h_x also purged h_p (already
	// expired); undoing that deletion must restore h_x to the document and
	// h_p to the Trash, in that order, or a crash mid-write loses h_x
	// entirely (neither file holds it).
	docWithX := func() *store.Document {
		d := sampleDocument()
		d.Households = append(d.Households, rolo.Household{ID: "h_x"})
		return d
	}
	docWithoutX := sampleDocument
	trashX := trashOf(entry("h_x", "", "", day(0)))
	trashP := trashOf(entry("h_p", "", "", day(0)))

	tests := []struct {
		name       string
		prev, next store.State
		breakDoc   bool // the document's directory does not exist
		breakTrash bool // the Trash's directory does not exist
		wantErr    bool
		wantDoc    bool // directory.json exists afterwards
		wantTrash  bool // trash.json exists afterwards
	}{
		{
			name:      "a deletion writes the Trash before the document",
			prev:      store.State{Document: doc, Trash: empty},
			next:      store.State{Document: sampleDocument(), Trash: full},
			breakDoc:  true,
			wantErr:   true,
			wantTrash: true,
		},
		{
			name:       "a restore writes the document before the Trash",
			prev:       store.State{Document: withoutCarla(), Trash: full},
			next:       store.State{Document: sampleDocument(), Trash: empty},
			breakTrash: true,
			wantErr:    true,
			wantDoc:    true,
		},
		{
			name:       "an unchanged Trash is not written",
			prev:       store.State{Document: doc, Trash: full},
			next:       store.State{Document: sampleDocument(), Trash: full},
			breakTrash: true,
			wantDoc:    true,
		},
		{
			name:      "an unchanged document is not written",
			prev:      store.State{Document: doc, Trash: empty},
			next:      store.State{Document: doc, Trash: full},
			breakDoc:  true,
			wantTrash: true,
		},
		{
			name:       "undoing a deletion that purged an expired entry writes the document first",
			prev:       store.State{Document: docWithoutX(), Trash: trashX},
			next:       store.State{Document: docWithX(), Trash: trashP},
			breakTrash: true,
			wantErr:    true,
			wantDoc:    true,
		},
		{
			name:      "a deletion that also purges writes the Trash first",
			prev:      store.State{Document: docWithX(), Trash: trashP},
			next:      store.State{Document: docWithoutX(), Trash: trashX},
			breakDoc:  true,
			wantErr:   true,
			wantTrash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			docPath := filepath.Join(dir, "directory.json")
			trashPath := filepath.Join(dir, "trash.json")

			if tt.breakDoc {
				docPath = filepath.Join(dir, "missing", "directory.json")
			}
			if tt.breakTrash {
				trashPath = filepath.Join(dir, "missing", "trash.json")
			}

			err := store.Persist(store.Paths{
				Document: docPath,
				Trash:    trashPath,
				Settings: filepath.Join(dir, "settings.json"),
			}, tt.prev, tt.next)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			_, docErr := os.Stat(docPath)
			_, trashErr := os.Stat(trashPath)
			assert.Equal(t, tt.wantDoc, docErr == nil, "directory.json written")
			assert.Equal(t, tt.wantTrash, trashErr == nil, "trash.json written")
		})
	}
}

// TestPersistSettings proves a title change touches only settings.json, and
// that a save mixing it with a document or Trash change is refused before
// anything is written (ADR-0013).
func TestPersistSettings(t *testing.T) {
	doc := sampleDocument()
	trash := store.NewTrash()
	before := store.NewSettings()
	after := before.WithTitle("The Langford Family Directory")

	tests := []struct {
		name         string
		prev, next   store.State
		wantErr      error
		wantDoc      bool
		wantTrash    bool
		wantSettings bool
	}{
		{
			name:         "a title change writes only the settings file",
			prev:         store.State{Document: doc, Trash: trash, Settings: before},
			next:         store.State{Document: doc, Trash: trash, Settings: after},
			wantSettings: true,
		},
		{
			name:    "an unchanged settings file is not written",
			prev:    store.State{Document: doc, Trash: trash, Settings: before},
			next:    store.State{Document: sampleDocument(), Trash: trash, Settings: before},
			wantDoc: true,
		},
		{
			name:    "a save changing the settings and the document is refused before any write",
			prev:    store.State{Document: doc, Trash: trash, Settings: before},
			next:    store.State{Document: sampleDocument(), Trash: trash, Settings: after},
			wantErr: store.ErrMixedSave,
		},
		{
			name:    "a save changing the settings and the Trash is refused before any write",
			prev:    store.State{Document: doc, Trash: trash, Settings: before},
			next:    store.State{Document: doc, Trash: sampleTrash(), Settings: after},
			wantErr: store.ErrMixedSave,
		},
		{
			name: "a nil settings is never written",
			prev: store.State{Document: doc, Trash: trash, Settings: before},
			next: store.State{Document: doc, Trash: trash},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			paths := store.Paths{
				Document: filepath.Join(dir, "directory.json"),
				Trash:    filepath.Join(dir, "trash.json"),
				Settings: filepath.Join(dir, "settings.json"),
			}

			err := store.Persist(paths, tt.prev, tt.next)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			_, docErr := os.Stat(paths.Document)
			_, trashErr := os.Stat(paths.Trash)
			_, settingsErr := os.Stat(paths.Settings)
			assert.Equal(t, tt.wantDoc, docErr == nil, "directory.json written")
			assert.Equal(t, tt.wantTrash, trashErr == nil, "trash.json written")
			assert.Equal(t, tt.wantSettings, settingsErr == nil, "settings.json written")
		})
	}
}

// tempPaths names the three files in a fresh directory.
func tempPaths(t *testing.T) store.Paths {
	t.Helper()
	dir := t.TempDir()

	return store.Paths{
		Document: filepath.Join(dir, "directory.json"),
		Trash:    filepath.Join(dir, "trash.json"),
		Settings: filepath.Join(dir, "settings.json"),
	}
}

func TestOpen(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "a document alone opens with an empty Trash and an untitled Directory",
			checkFunc: func(t *testing.T) {
				t.Helper()
				paths := tempPaths(t)
				require.NoError(t, store.Save(paths.Document, sampleDocument()))

				got, err := store.Open(paths, day(0))
				require.NoError(t, err)

				assert.Len(t, got.Document.Households, 3)
				require.NotNil(t, got.Trash)
				assert.Empty(t, got.Trash.Entries)
				require.NotNil(t, got.Settings)
				assert.Empty(t, got.Settings.Title)
			},
		},
		{
			name: "all three files are read",
			checkFunc: func(t *testing.T) {
				t.Helper()
				paths := tempPaths(t)
				require.NoError(t, store.Save(paths.Document, sampleDocument()))
				require.NoError(t, store.SaveTrash(paths.Trash, trashOf(entry("h_gone", "", "", day(0)))))
				require.NoError(t, store.SaveSettings(paths.Settings, store.NewSettings().WithTitle("The Whitlocks")))

				got, err := store.Open(paths, day(1))
				require.NoError(t, err)

				assert.Equal(t, []rolo.HouseholdID{"h_gone"}, ids(got.Trash))
				assert.Equal(t, "The Whitlocks", got.Settings.Title)
			},
		},
		{
			name: "the Trash is reconciled against the document before it is served",
			checkFunc: func(t *testing.T) {
				t.Helper()
				paths := tempPaths(t)
				require.NoError(t, store.Save(paths.Document, sampleDocument()))
				// h_carla is in both files, as a crash between two writes leaves it.
				require.NoError(t, store.SaveTrash(paths.Trash, trashOf(entry("h_carla", "h_clyde", "", day(0)))))

				got, err := store.Open(paths, day(1))
				require.NoError(t, err)

				assert.Empty(t, got.Trash.Entries, "the document wins")
			},
		},
		{
			name: "a missing document is refused",
			checkFunc: func(t *testing.T) {
				t.Helper()

				_, err := store.Open(tempPaths(t), day(0))
				require.ErrorContains(t, err, "load store")
			},
		},
		{
			name: "an unreadable Trash is refused",
			checkFunc: func(t *testing.T) {
				t.Helper()
				paths := tempPaths(t)
				require.NoError(t, store.Save(paths.Document, sampleDocument()))
				require.NoError(t, os.WriteFile(paths.Trash, []byte("{not json"), 0o600))

				_, err := store.Open(paths, day(0))
				require.ErrorContains(t, err, "load trash")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.checkFunc)
	}
}

func TestPathsWrite(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T)
	}{
		{
			name: "a settings change is written through Persist",
			checkFunc: func(t *testing.T) {
				t.Helper()
				paths := tempPaths(t)
				prev := store.State{Document: sampleDocument(), Trash: store.NewTrash(), Settings: store.NewSettings()}
				next := prev
				next.Settings = prev.Settings.WithTitle("The Whitlocks")

				require.NoError(t, paths.Write(prev, next))

				got, err := store.LoadSettings(paths.Settings)
				require.NoError(t, err)
				assert.Equal(t, "The Whitlocks", got.Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.checkFunc)
	}
}
