package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
)

// TestPersistOrder proves the add-before-remove rule (ADR-0012) by making one
// of the two writes fail and observing whether the other already happened.
func TestPersistOrder(t *testing.T) {
	doc := sampleDocument()
	empty := store.NewTrash()
	full := sampleTrash()

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
			prev:       store.State{Document: doc, Trash: full},
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

			err := store.Persist(docPath, trashPath, tt.prev, tt.next)

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
