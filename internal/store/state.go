package store

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// State is everything the service persists: the Directory, its Trash and the
// Editor's settings. None is mutated once built; a change is a new State,
// which is what lets Persist tell what changed by comparing pointers.
type State struct {
	Document *Document
	Trash    *Trash

	// Settings is never written when nil, so a State built without it — by a
	// test that predates the settings file — cannot erase the title on disk.
	Settings *Settings
}

// Paths names the files Persist may write.
type Paths struct {
	Document string
	Trash    string
	Settings string
}

// Write persists next over prev at these paths. It is Persist as a
// live.Writer, so the production adapter needs no glue.
func (p Paths) Write(prev, next State) error { return Persist(p, prev, next) }

// Open loads everything the service persists, in the order the files depend
// on one another: the document, then the Trash reconciled against it — so a
// crash that left a Household in both files is settled before anything is
// served (ADR-0012) — then the settings, where a missing file is an untitled
// Directory (ADR-0013).
func Open(p Paths, now time.Time) (State, error) {
	doc, err := Load(p.Document)
	if err != nil {
		return State{}, fmt.Errorf("load store: %w", err)
	}

	trash, err := OpenTrash(p.Trash, doc, now)
	if err != nil {
		return State{}, fmt.Errorf("load trash: %w", err)
	}

	settings, err := LoadSettings(p.Settings)
	if err != nil {
		return State{}, fmt.Errorf("load settings: %w", err)
	}

	return State{Document: doc, Trash: trash, Settings: settings}, nil
}

// ErrMixedSave means one save changed the settings together with the document
// or the Trash. No save does that today, and Persist relies on it: with only
// one file to write, a title change needs no crash-ordering rule (ADR-0013).
// A future save that needs both must add that rule rather than remove this
// check.
var ErrMixedSave = errors.New("a save changed the settings together with the directory")

// Persist writes whatever changed between prev and next.
//
// A settings change is written on its own; see ErrMixedSave.
//
// No filesystem renames two files atomically, so the order of the document and
// Trash writes is chosen so that a crash between them can only ever duplicate
// a Household, never lose one. The decision is made on the document alone:
// when the document gains a Household it did not have — a restore, or an undo
// reaching back past it — that Household is leaving the Trash, so the document
// is written first; otherwise the Trash is written first. A Trash losing
// entries by purge is never what drives the order: a purge is a deliberate
// loss (an entry's 30 days are up, or it was only being kept for one that
// stays gone), not a Household moving between the two files, and the Trash can
// shrink between prev and next — during a deletion that also purges an
// expired entry, or an undo that reaches back past one — without the document
// gaining anything. OpenTrash reconciles the duplicate on the next start
// (ADR-0012).
func Persist(paths Paths, prev, next State) error {
	if next.Settings != nil && next.Settings != prev.Settings {
		if next.Document != prev.Document || next.Trash != prev.Trash {
			return ErrMixedSave
		}

		return SaveSettings(paths.Settings, next.Settings)
	}

	saveDoc := func() error {
		if next.Document == prev.Document {
			return nil
		}

		return Save(paths.Document, next.Document)
	}

	saveTrash := func() error {
		if next.Trash == prev.Trash {
			return nil
		}

		return SaveTrash(paths.Trash, next.Trash)
	}

	first, second := saveTrash, saveDoc
	if gainsHousehold(prev.Document, next.Document) {
		first, second = saveDoc, saveTrash
	}

	err := first()
	if err != nil {
		return err
	}

	return second()
}

// gainsHousehold reports whether next holds a Household prev did not.
func gainsHousehold(prev, next *Document) bool {
	if next == nil || next == prev {
		return false
	}

	for _, h := range next.Households {
		if prev == nil {
			return true
		}

		i := slices.IndexFunc(prev.Households, func(ph rolo.Household) bool { return ph.ID == h.ID })
		if i < 0 {
			return true
		}
	}

	return false
}
