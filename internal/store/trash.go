package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
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

var (
	// ErrNotInTrash means a restore named a Household the Trash does not hold.
	ErrNotInTrash = errors.New("household is not in the trash")

	// ErrUnrestorable means a trashed Household needs another — its parent or
	// its Shared Address — that is neither in the Directory nor in the Trash.
	// The retention rule in Purge prevents this; only a hand edit produces it.
	ErrUnrestorable = errors.New("household needs another that is neither in the directory nor the trash")

	// ErrNotInDocument means a deletion named a Household the document does not hold.
	ErrNotInDocument = errors.New("household is not in the directory")
)

// Entry returns the entry holding id.
func (t *Trash) Entry(id rolo.HouseholdID) (TrashEntry, bool) {
	for _, e := range t.Entries {
		if e.Household.ID == id {
			return e, true
		}
	}

	return TrashEntry{}, false
}

// Expires is when this entry's own 30 days end. It may be kept longer, for as
// long as a newer entry needs it; see Purge.
func (e TrashEntry) Expires() time.Time { return e.DeletedAt.Add(TrashRetention) }

// needs lists the Households this entry cannot be restored without: the
// parent that anchors its Path, and the Household whose Address it shares.
func (e TrashEntry) needs() []rolo.HouseholdID {
	var out []rolo.HouseholdID

	if e.Household.Parent != "" {
		out = append(out, e.Household.Parent)
	}

	if e.Household.Address.SharedWith != "" {
		out = append(out, e.Household.Address.SharedWith)
	}

	return out
}

// filter returns the entries keep accepts. When it accepts them all it returns
// t itself, so an unchanged Trash keeps its identity and Persist skips the
// write.
func (t *Trash) filter(keep func(TrashEntry) bool) (*Trash, bool) {
	kept := make([]TrashEntry, 0, len(t.Entries))

	for _, e := range t.Entries {
		if keep(e) {
			kept = append(kept, e)
		}
	}

	if len(kept) == len(t.Entries) {
		return t, false
	}

	return &Trash{Schema: t.Schema, Entries: kept}, true
}

// liveIDs is the set of Households in doc.
func liveIDs(doc *Document) map[rolo.HouseholdID]bool {
	live := make(map[rolo.HouseholdID]bool, len(doc.Households))
	for _, h := range doc.Households {
		live[h.ID] = true
	}

	return live
}

// Reconcile drops every entry whose Household is also in doc. A crash between
// the two writes of a deletion or a restore leaves such a duplicate by design,
// and the document wins because it is what the Editor last saw (ADR-0012).
func (t *Trash) Reconcile(doc *Document) (*Trash, bool) {
	live := liveIDs(doc)

	return t.filter(func(e TrashEntry) bool { return !live[e.Household.ID] })
}

// Purge drops entries whose time is up.
//
// An entry past its 30 days is still kept while any unexpired entry needs it,
// directly or through another entry. Otherwise deleting Clyde/Doris and then,
// three weeks later, Dave/Diane beneath them would leave Dave/Diane
// unrestorable after only nine days — Restore brings their parent back with
// them, and could not if it were gone.
func (t *Trash) Purge(now time.Time) (*Trash, bool) {
	keep := make(map[rolo.HouseholdID]bool)

	var mark func(id rolo.HouseholdID)
	mark = func(id rolo.HouseholdID) {
		if keep[id] {
			return
		}

		e, ok := t.Entry(id)
		if !ok {
			// In the Directory, or nowhere: either way not the Trash's to keep.
			return
		}

		keep[id] = true

		for _, need := range e.needs() {
			mark(need)
		}
	}

	for _, e := range t.Entries {
		if now.Before(e.Expires()) {
			mark(e.Household.ID)
		}
	}

	return t.filter(func(e TrashEntry) bool { return keep[e.Household.ID] })
}

// Chain lists the Households that must return together to restore id: id
// itself first, then every trashed Household it needs, transitively. Anything
// already in doc needs nothing. If id is both in the Trash and the Document,
// the Document wins over the stale duplicate and the entry is not restorable.
func (t *Trash) Chain(id rolo.HouseholdID, doc *Document) ([]rolo.HouseholdID, error) {
	if _, ok := t.Entry(id); !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotInTrash, id)
	}

	live := liveIDs(doc)

	if live[id] {
		return nil, fmt.Errorf("%w: %s is already in the directory", ErrNotInTrash, id)
	}
	seen := make(map[rolo.HouseholdID]bool)

	var chain []rolo.HouseholdID

	var visit func(id rolo.HouseholdID) error
	visit = func(id rolo.HouseholdID) error {
		if live[id] || seen[id] {
			return nil
		}

		e, ok := t.Entry(id)
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnrestorable, id)
		}

		seen[id] = true
		chain = append(chain, id)

		for _, need := range e.needs() {
			err := visit(need)
			if err != nil {
				return err
			}
		}

		return nil
	}

	err := visit(id)
	if err != nil {
		return nil, err
	}

	return chain, nil
}

// Delete moves entry's Household out of doc and into t, returning new values
// and leaving both inputs untouched. It enforces nothing about *whether* the
// Household may go — that is rolo.Tree.DeleteBlock's, checked by the caller —
// but a caller that skipped the check would still be stopped when the result
// failed to build a tree.
func Delete(doc *Document, t *Trash, entry TrashEntry) (*Document, *Trash, error) {
	id := entry.Household.ID

	i := slices.IndexFunc(doc.Households, func(h rolo.Household) bool { return h.ID == id })
	if i < 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrNotInDocument, id)
	}

	nextDoc := &Document{
		Schema:     doc.Schema,
		Households: slices.Delete(slices.Clone(doc.Households), i, i+1),
	}

	nextTrash := &Trash{
		Schema:  t.Schema,
		Entries: append(slices.Clone(t.Entries), entry),
	}

	return nextDoc, nextTrash, nil
}

// Restore moves id, and every trashed Household it needs, from t back into
// doc. It returns the restored Households with id first, for the announcement
// to name.
func Restore(doc *Document, t *Trash, id rolo.HouseholdID) (*Document, *Trash, []rolo.Household, error) {
	chain, err := t.Chain(id, doc)
	if err != nil {
		return nil, nil, nil, err
	}

	nextDoc := &Document{Schema: doc.Schema, Households: slices.Clone(doc.Households)}
	restored := make([]rolo.Household, 0, len(chain))

	for _, rid := range chain {
		e, _ := t.Entry(rid) // Chain only returns IDs it found in t.
		nextDoc.Households = append(nextDoc.Households, e.Household)
		restored = append(restored, e.Household)
	}

	nextTrash, _ := t.filter(func(e TrashEntry) bool { return !slices.Contains(chain, e.Household.ID) })

	return nextDoc, nextTrash, restored, nil
}
