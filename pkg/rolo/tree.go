package rolo

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// PathSeparator joins Household labels in a rendered Path.
const PathSeparator = " › "

var (
	// ErrUnknownParent means a Household names a parent that is not in the document.
	ErrUnknownParent = errors.New("unknown parent household")
	// ErrDuplicateID means an ID appears twice in the document — either two
	// Households sharing a HouseholdID, or a PersonID appearing in more than
	// one place. IDs are identities, so a collision makes the document
	// ambiguous about who is who.
	ErrDuplicateID = errors.New("duplicate id")
	// ErrCycle means the parent links form a loop, so a Household is its own ancestor.
	ErrCycle = errors.New("cycle in household parentage")
	// ErrNoAdults means a Household has no adults, which cannot be labelled or rendered.
	ErrNoAdults = errors.New("household has no adults")
	// ErrUnknownHousehold means a lookup named a Household that is not in the tree.
	ErrUnknownHousehold = errors.New("unknown household")
)

// Tree is the navigation structure derived from Households' parent links. It
// is never stored: it is rebuilt whenever the document is loaded, so the parent
// links are the single source of truth and no stored child list can drift.
type Tree struct {
	byID     map[HouseholdID]Household
	children map[HouseholdID][]HouseholdID
	roots    []HouseholdID
}

// BuildTree derives the navigation tree from a flat slice of Households,
// validating that the parent links form a forest.
//
// Sibling order is derived from each Household's eldest adult's birth date,
// which is how families list their children. Households whose eldest adult has
// no birth date sort last, by label.
func BuildTree(households []Household) (*Tree, error) {
	t := &Tree{
		byID:     make(map[HouseholdID]Household, len(households)),
		children: make(map[HouseholdID][]HouseholdID),
	}

	// Each pass must complete before the next begins: indexing populates byID,
	// which the address and parent passes both look names up in.
	err := t.index(households)
	if err != nil {
		return nil, err
	}

	err = t.checkSharedAddresses(households)
	if err != nil {
		return nil, err
	}

	err = t.link(households)
	if err != nil {
		return nil, err
	}

	// Every Household must reach a root by following parent links. A node that
	// does not is part of a cycle: it has a valid parent, but that chain loops
	// rather than terminating, so it never appears under any root.
	err = t.detectCycles()
	if err != nil {
		return nil, err
	}

	t.sortSiblings()

	return t, nil
}

// index records every Household by ID, rejecting a document that reuses one.
//
// A Person belongs to exactly one Household, so a PersonID may appear only once
// across the whole document — whether as an adult or as a Dependent.
func (t *Tree) index(households []Household) error {
	seenPersonIDs := make(map[PersonID]bool)

	for _, h := range households {
		if _, dup := t.byID[h.ID]; dup {
			return fmt.Errorf("%w: household %s appears more than once", ErrDuplicateID, h.ID)
		}

		if len(h.Adults) == 0 {
			return fmt.Errorf("%w: %s", ErrNoAdults, h.ID)
		}

		for _, group := range [][]Person{h.Adults, h.Dependents} {
			for _, p := range group {
				if p.ID == "" {
					continue
				}

				if seenPersonIDs[p.ID] {
					return fmt.Errorf("%w: person %s appears more than once", ErrDuplicateID, p.ID)
				}

				seenPersonIDs[p.ID] = true
			}
		}

		t.byID[h.ID] = h
	}

	return nil
}

// checkSharedAddresses verifies that every Shared Address names a Household that
// exists. It runs after index so that a forward reference is still valid.
func (t *Tree) checkSharedAddresses(households []Household) error {
	for _, h := range households {
		if h.Address.SharedWith == "" {
			continue
		}

		if _, ok := t.byID[h.Address.SharedWith]; !ok {
			return fmt.Errorf(
				"%w: %s shares the address of %s",
				ErrUnknownHousehold,
				h.ID,
				h.Address.SharedWith,
			)
		}
	}

	return nil
}

// link fills in roots and children from the Parent references, rejecting a
// parent that does not exist or that is the Household itself.
func (t *Tree) link(households []Household) error {
	for _, h := range households {
		if h.Parent == "" {
			t.roots = append(t.roots, h.ID)
			continue
		}

		if h.Parent == h.ID {
			return fmt.Errorf("%w: %s is its own parent", ErrCycle, h.ID)
		}

		if _, ok := t.byID[h.Parent]; !ok {
			return fmt.Errorf("%w: %s names parent %s", ErrUnknownParent, h.ID, h.Parent)
		}

		t.children[h.Parent] = append(t.children[h.Parent], h.ID)
	}

	return nil
}

// detectCycles walks upward from every Household, bounding each walk by the
// number of Households. A chain longer than that must have revisited a node.
func (t *Tree) detectCycles() error {
	for id := range t.byID {
		current := id
		for steps := 0; ; steps++ {
			h := t.byID[current]
			if h.Parent == "" {
				break
			}
			if steps > len(t.byID) {
				return fmt.Errorf("%w: %s is its own ancestor", ErrCycle, id)
			}
			current = h.Parent
		}
	}
	return nil
}

// sortSiblings orders the roots and every child list by eldest adult's birth
// date, falling back to label for Households with no known birth date.
func (t *Tree) sortSiblings() {
	sortIDs := func(ids []HouseholdID) {
		sort.SliceStable(ids, func(i, j int) bool {
			a, b := t.byID[ids[i]], t.byID[ids[j]]
			ab, bb := a.EldestAdultBirth(), b.EldestAdultBirth()

			switch {
			case ab.IsZero() && bb.IsZero():
				return a.Label() < b.Label()
			case ab.IsZero():
				return false // unknown birth dates sort last
			case bb.IsZero():
				return true
			case ab != bb:
				return dateLess(ab, bb)
			default:
				return a.Label() < b.Label()
			}
		})
	}

	sortIDs(t.roots)
	for parent := range t.children {
		sortIDs(t.children[parent])
	}
}

// Get returns a Household by ID.
func (t *Tree) Get(id HouseholdID) (Household, bool) {
	h, ok := t.byID[id]
	return h, ok
}

// Roots returns the Households with no parent, in sibling order.
func (t *Tree) Roots() []Household { return t.lookup(t.roots) }

// Children returns a Household's children, in sibling order.
func (t *Tree) Children(id HouseholdID) []Household { return t.lookup(t.children[id]) }

func (t *Tree) lookup(ids []HouseholdID) []Household {
	out := make([]Household, 0, len(ids))
	for _, id := range ids {
		out = append(out, t.byID[id])
	}
	return out
}

// Path returns the chain of Households from the root down to id, inclusive.
func (t *Tree) Path(id HouseholdID) ([]Household, error) {
	if _, ok := t.byID[id]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownHousehold, id)
	}

	var chain []Household
	for current := id; current != ""; {
		h := t.byID[current]
		chain = append(chain, h)
		current = h.Parent
	}

	// Reverse: the walk collected leaf-to-root.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain, nil
}

// PathString renders a Household's Path as "Aden/Nettie › Clyde/Doris ›
// Dave/Diane". This is the human-readable identity of a Household, and is what
// disambiguates relatives who share a given name.
func (t *Tree) PathString(id HouseholdID) (string, error) {
	chain, err := t.Path(id)
	if err != nil {
		return "", err
	}

	labels := make([]string, 0, len(chain))
	for _, h := range chain {
		labels = append(labels, h.Label())
	}
	return strings.Join(labels, PathSeparator), nil
}

// Walk visits every Household depth-first in sibling order, which is the order
// the rendered Directory uses. Returning an error from fn stops the walk and
// returns that error.
func (t *Tree) Walk(fn func(h Household, depth int) error) error {
	var visit func(id HouseholdID, depth int) error
	visit = func(id HouseholdID, depth int) error {
		if err := fn(t.byID[id], depth); err != nil {
			return err
		}
		for _, child := range t.children[id] {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}

	for _, root := range t.roots {
		if err := visit(root, 0); err != nil {
			return err
		}
	}
	return nil
}
