package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// CurrentSchema is the document version this binary writes and is the highest
// it can read.
const CurrentSchema = 1

var (
	// ErrSchemaTooNew means the document was written by a newer binary. This is
	// the rollback case: reading it would silently drop fields this binary does
	// not know about, so the store refuses rather than corrupting the Directory.
	ErrSchemaTooNew = errors.New("document schema is newer than this binary understands")

	// ErrSchemaMissing means the document has no schema field, so its version
	// cannot be established.
	ErrSchemaMissing = errors.New("document has no schema version")
)

// documentPerm keeps the document readable only by its owner. It holds
// relatives' home addresses, birth dates and phone numbers.
const documentPerm os.FileMode = 0o600

// Document is the on-disk shape of the Directory: a schema version and a flat
// slice of Households. The navigation tree is not stored; it is derived from
// the Households' parent links by Tree.
type Document struct {
	Schema     int              `json:"schema"`
	Households []rolo.Household `json:"households"`
}

// Tree derives the navigation tree, validating the parent links.
func (d *Document) Tree() (*rolo.Tree, error) { return rolo.BuildTree(d.Households) }

// Load reads and validates the document at path.
//
// Validation happens here rather than at render time so that a structural
// problem surfaces at startup, where the Operator sees it, rather than when the
// Editor clicks Export.
func Load(path string) (*Document, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read document: %w", err)
	}

	// Decode the schema first: a document from a newer binary may contain
	// shapes this one cannot parse, so its version must be checked before any
	// attempt to read the rest.
	err = checkSchema(b, path, CurrentSchema)
	if err != nil {
		return nil, err
	}

	var doc Document

	err = json.Unmarshal(b, &doc)
	if err != nil {
		return nil, fmt.Errorf("parse document: %w", err)
	}

	_, err = doc.Tree()
	if err != nil {
		return nil, fmt.Errorf("validate document: %w", err)
	}

	return &doc, nil
}

// Save writes the document to path atomically.
func Save(path string, doc *Document) error {
	err := writeJSON(path, doc)
	if err != nil {
		return fmt.Errorf("save document: %w", err)
	}

	return nil
}

// checkSchema reads only the schema field of b and refuses a version newer
// than current. It is shared by every file the store writes, because each has
// the same rollback hazard: a binary that decodes a newer shape silently drops
// the fields it does not know, and its next save erases them.
func checkSchema(b []byte, path string, current int) error {
	var probe struct {
		Schema *int `json:"schema"`
	}

	err := json.Unmarshal(b, &probe)
	if err != nil {
		return fmt.Errorf("read schema of %s: %w", path, err)
	}

	if probe.Schema == nil {
		return fmt.Errorf("%w: %s", ErrSchemaMissing, path)
	}

	if *probe.Schema > current {
		return fmt.Errorf("%w: %s is version %d, this binary reads up to %d",
			ErrSchemaTooNew, path, *probe.Schema, current)
	}

	return nil
}

// writeJSON writes v to path as indented JSON, atomically. Indented because the
// Operator repairs these files by hand over SSH.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	b = append(b, '\n')

	return writeFileAtomic(path, b)
}
