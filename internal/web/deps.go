package web

import (
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
