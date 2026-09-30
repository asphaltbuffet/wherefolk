// Package live holds the Directory the service is serving: the document, its
// Trash and the Editor's settings as last saved, the tree derived from them,
// and the one step Undo can return to.
//
// It is the only place those change. A write is a function from the current
// Snapshot to the next store.State; a Copy serialises writes, saves each
// through its Writer, and swaps the result in only once that succeeds, so a
// failed save leaves the served Directory matching the disk. Readers take a
// Snapshot and hold no lock: nothing a Snapshot points at is ever mutated,
// only replaced.
package live

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// ErrUnbuildable means a document's Households do not form a tree, so it
// cannot be served. A Copy never writes such a document.
var ErrUnbuildable = errors.New("document does not build a tree")

// Writer persists a change. It is handed the state being replaced as well as
// the new one, so it can tell which files changed and order the writes so that
// a crash cannot lose a Household (store.Persist, ADR-0012). store.Paths
// satisfies it.
//
// It is called synchronously, inside the request that made the change: the
// atomic write in internal/store protects a write that has begun, and graceful
// shutdown waits for in-flight requests, so a write handed to a goroutine
// would escape both.
type Writer interface {
	Write(prev, next store.State) error
}

// WriterFunc adapts a function to Writer.
type WriterFunc func(prev, next store.State) error

// Write calls f.
func (f WriterFunc) Write(prev, next store.State) error { return f(prev, next) }

// Clock reports the current time. The Trash's 30 days are measured against it.
type Clock func() time.Time

// Copy is the Directory being served. Build one with New.
type Copy struct {
	mu      sync.Mutex // serialises writers; readers never take it
	current atomic.Pointer[served]
	writer  Writer
	now     Clock
}

// served is one immutable generation of what a Copy serves.
type served struct {
	state store.State // as stored: the Trash is not purged
	tree  *rolo.Tree
	undo  *undoPoint // nil when there is nothing to undo
}

// undoPoint is the state before the latest save, and the token that names
// that save (CONTEXT.md, Undo).
type undoPoint struct {
	token  string
	before store.State
}

// Snapshot is the served Directory at one moment. Nothing it points at is
// ever mutated; a write that wants to change something builds a new value.
type Snapshot struct {
	Document *store.Document
	Tree     *rolo.Tree
	// Trash is the Trash as the Editor should see it: purged as of Now. An
	// expired entry nothing needs is still stored — and restorable — until
	// the next Trash write, but it must not be listed or counted. Reading it
	// writes nothing.
	Trash    *store.Trash
	Settings *store.Settings
	// Now is the one clock reading this Snapshot was taken at. A write uses
	// it for everything it dates, so a deletion and the purge beside it agree.
	Now time.Time

	stored    *store.Trash
	undoToken string
}

// State is the served state as stored, with the Trash unpurged. It is the
// base every write builds its next State from.
func (s Snapshot) State() store.State {
	return store.State{Document: s.Document, Trash: s.stored, Settings: s.Settings}
}

// CanUndo reports whether token names the latest save, so that an Undo
// offered with it would still work.
func (s Snapshot) CanUndo(token string) bool { return token != "" && token == s.undoToken }

// Saved is what an Update did.
type Saved struct {
	// Snapshot is what is served after the call, whether or not it saved.
	Snapshot Snapshot
	// Changed is false when the build handed back what was already served:
	// nothing was written and no Undo was issued.
	Changed bool
	// Undo is the token of this save's Undo, or empty when nothing changed.
	Undo string
}

// New serves initial. A missing Trash is an empty one and missing settings
// are an untitled Directory, as they are for a missing file.
func New(initial store.State, w Writer, now Clock) (*Copy, error) {
	if initial.Document == nil {
		return nil, errors.New("live: document is nil")
	}

	if w == nil {
		return nil, errors.New("live: writer is nil")
	}

	if now == nil {
		return nil, errors.New("live: clock is nil")
	}

	if initial.Trash == nil {
		initial.Trash = store.NewTrash()
	}

	if initial.Settings == nil {
		initial.Settings = store.NewSettings()
	}

	tree, err := initial.Document.Tree()
	if err != nil {
		return nil, fmt.Errorf("live: %w: %w", ErrUnbuildable, err)
	}

	c := &Copy{writer: w, now: now}
	c.current.Store(&served{state: initial, tree: tree})

	return c, nil
}

// Snapshot is what is served now. It takes no lock.
func (c *Copy) Snapshot() Snapshot { return snapshotOf(c.current.Load(), c.now()) }

// Update makes one change. build receives the current Snapshot and returns
// the next State; it runs while other writers wait, so it must do no I/O and
// must not call back into c.
//
// A field build leaves nil is left as served, and handing back the Snapshot's
// effective Trash means the Trash did not change. When nothing changed,
// Update writes nothing, issues no Undo and leaves the previous Undo standing.
// Otherwise it purges the Trash if the Trash changed, builds the tree, writes,
// and only then serves the result and records it as the one step Undo can
// reverse.
//
// An error from build is returned as it is, so a caller can carry its own
// refusal out; nothing is written. Any error leaves the served state and its
// Undo untouched.
func (c *Copy) Update(build func(Snapshot) (store.State, error)) (Saved, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cur := c.current.Load()
	snap := snapshotOf(cur, c.now())

	next, err := build(snap)
	if err != nil {
		return Saved{Snapshot: snap}, err
	}

	next = resolve(next, snap)
	if next == cur.state {
		return Saved{Snapshot: snap}, nil
	}

	// Purging on every Trash write keeps expiry free of a timer (CONTEXT.md,
	// Trash). A write that leaves the Trash alone does not purge it, so it
	// does not rewrite trash.json.
	if next.Trash != cur.state.Trash {
		next.Trash, _ = next.Trash.Purge(snap.Now)
	}

	sv, err := c.write(cur, next)
	if err != nil {
		return Saved{Snapshot: snap}, err
	}

	sv.undo = &undoPoint{token: rand.Text(), before: cur.state}
	c.current.Store(sv)

	return Saved{Snapshot: snapshotOf(sv, snap.Now), Changed: true, Undo: sv.undo.token}, nil
}

// write builds next's tree and saves it, returning what to serve. It swaps
// nothing: on an error the caller goes on serving cur.
func (c *Copy) write(cur *served, next store.State) (*served, error) {
	tree, err := next.Document.Tree()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnbuildable, err)
	}

	err = c.writer.Write(cur.state, next)
	if err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}

	return &served{state: next, tree: tree}, nil
}

// resolve fills what a build left nil with what is served, and maps the
// effective Trash back to the stored one: either pointer means "the Trash did
// not change", and neither may be mistaken for a change.
func resolve(next store.State, snap Snapshot) store.State {
	if next.Document == nil {
		next.Document = snap.Document
	}

	if next.Trash == nil || next.Trash == snap.Trash {
		next.Trash = snap.stored
	}

	if next.Settings == nil {
		next.Settings = snap.Settings
	}

	return next
}

// snapshotOf is sv as seen at now.
func snapshotOf(sv *served, now time.Time) Snapshot {
	trash, _ := sv.state.Trash.Purge(now)

	snap := Snapshot{
		Document: sv.state.Document,
		Tree:     sv.tree,
		Trash:    trash,
		Settings: sv.state.Settings,
		Now:      now,
		stored:   sv.state.Trash,
	}

	if sv.undo != nil {
		snap.undoToken = sv.undo.token
	}

	return snap
}
