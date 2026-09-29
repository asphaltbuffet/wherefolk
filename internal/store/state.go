package store

// State is everything the service persists: the Directory and its Trash.
// Neither is mutated once built; a change is a new State, which is what lets
// Persist tell what changed by comparing pointers.
type State struct {
	Document *Document
	Trash    *Trash
}

// Persist writes whatever changed between prev and next.
//
// No filesystem renames two files atomically, so the order is chosen so that a
// crash between the writes can only ever duplicate a Household, never lose
// one: when the Trash gains an entry (a deletion) it is written first, and
// otherwise (a restore, an edit) the document is. OpenTrash reconciles the
// duplicate on the next start (ADR-0012).
func Persist(docPath, trashPath string, prev, next State) error {
	saveDoc := func() error {
		if next.Document == prev.Document {
			return nil
		}

		return Save(docPath, next.Document)
	}

	saveTrash := func() error {
		if next.Trash == prev.Trash {
			return nil
		}

		return SaveTrash(trashPath, next.Trash)
	}

	first, second := saveDoc, saveTrash
	if gainsEntry(prev.Trash, next.Trash) {
		first, second = saveTrash, saveDoc
	}

	err := first()
	if err != nil {
		return err
	}

	return second()
}

// gainsEntry reports whether next holds a Household prev did not.
func gainsEntry(prev, next *Trash) bool {
	if next == nil || next == prev {
		return false
	}

	for _, e := range next.Entries {
		if prev == nil {
			return true
		}

		if _, ok := prev.Entry(e.Household.ID); !ok {
			return true
		}
	}

	return false
}
