package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// CurrentTrashSchema is the Trash file version this binary writes and is the
// highest it can read. It is versioned apart from the document: ADR-0012 keeps
// the Trash out of the document precisely so the two can change separately.
const CurrentTrashSchema = 1

// TrashRetention is the least time a deleted Household is kept (§3).
const TrashRetention = 30 * 24 * time.Hour

// Trash holds deleted Households until they are purged. It lives in its own
// file beside the document; see ADR-0012.
//
// Like Document, a Trash is never mutated once built: every change returns a
// new value. Persist relies on that, comparing pointers to decide which files
// changed.
type Trash struct {
	Schema  int          `json:"schema"`
	Entries []TrashEntry `json:"entries"`
}

// TrashEntry is one deleted Household, exactly as it was when deleted.
type TrashEntry struct {
	DeletedAt time.Time `json:"deleted_at"`
	// Path is the Household's Path at the moment of deletion. It is for display
	// only: the entry's own Parent is what a restore follows, and the Path can
	// no longer be derived once the Household has left the tree.
	Path      string         `json:"path"`
	Household rolo.Household `json:"household"`
}

// NewTrash returns an empty Trash at the current schema. Entries is non-nil so
// the file reads "entries": [] rather than null.
func NewTrash() *Trash {
	return &Trash{Schema: CurrentTrashSchema, Entries: []TrashEntry{}}
}

// LoadTrash reads the Trash at path. A missing file is an empty Trash: nothing
// has been deleted yet, which is every installation's first state.
func LoadTrash(path string) (*Trash, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return NewTrash(), nil
	}

	if err != nil {
		return nil, fmt.Errorf("read trash: %w", err)
	}

	err = checkSchema(b, path, CurrentTrashSchema)
	if err != nil {
		return nil, err
	}

	t := NewTrash()

	err = json.Unmarshal(b, t)
	if err != nil {
		return nil, fmt.Errorf("parse trash: %w", err)
	}

	return t, nil
}

// SaveTrash writes the Trash to path atomically, with the document's
// permissions: it holds the same relatives' addresses and birth dates.
func SaveTrash(path string, t *Trash) error {
	err := writeJSON(path, t)
	if err != nil {
		return fmt.Errorf("save trash: %w", err)
	}

	return nil
}
