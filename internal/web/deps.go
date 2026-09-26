package web

import (
	"context"
	"time"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// Saver persists the whole Directory. It is injected rather than called
// directly so that internal/web keeps the freedom from filesystem concerns
// that New's signature exists to preserve: main closes over the document path
// and this package never learns it.
//
// It is called synchronously inside the request that triggered it. The atomic
// temp/fsync/rename in internal/store protects a write that has begun, not one
// that never got to run, and main's graceful shutdown waits for in-flight
// requests — a save handed to a goroutine would escape both.
type Saver func(*store.Document) error

// NewHouseholdIDFunc mints a stable identity for a new Household. Injected so
// that tests can assert on whole documents with fixed IDs rather than matching
// a prefix and looking the record up.
type NewHouseholdIDFunc func() (rolo.HouseholdID, error)

// NewPersonIDFunc mints a stable identity for a new Person, injected for the
// same reason as NewHouseholdIDFunc.
type NewPersonIDFunc func() (rolo.PersonID, error)

// Exporter compiles a filtered Directory. render.Renderer satisfies it; it is
// an interface so this package's tests need no typst binary, and so the web
// layer holds no path to a template or a binary (see Meta).
type Exporter interface {
	PDF(ctx context.Context, d render.Directory) ([]byte, error)
	SVG(ctx context.Context, d render.Directory) ([][]byte, error)
}

// Clock reports the current time. Injected because an export's content depends
// on the date — ages are computed at export time (§5.7) — so tests must pin it.
type Clock func() time.Time
