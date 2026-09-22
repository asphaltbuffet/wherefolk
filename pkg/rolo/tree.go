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
	// ErrDuplicateID means two Households share an ID.
	ErrDuplicateID = errors.New("duplicate household id")
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

	seenPersonIDs := make(map[PersonID]bool)

	for _, h := range households {
		if _, dup := t.byID[h.ID]; dup {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateID, h.ID)
		}
		if len(h.Adults) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrNoAdults, h.ID)
		}
		for _, p := range h.Adults {
			if p.ID == "" {
				continue
			}
			if seenPersonIDs[p.ID] {
				return nil, fmt.Errorf("%w: %s", ErrDuplicateID, p.ID)
			}
			seenPersonIDs[p.ID] = true
		}
		for _, p := range h.Dependents {
			if p.ID == "" {
				continue
			}
			if seenPersonIDs[p.ID] {
				return nil, fmt.Errorf("%w: %s", ErrDuplicateID, p.ID)
			}
			seenPersonIDs[p.ID] = true
		}
		t.byID[h.ID] = h
	}

	for _, h := range households {
		if h.Address.SharedWith != "" {
			if _, ok := t.byID[h.Address.SharedWith]; !ok {
				return nil, fmt.Errorf("%w: %s shares the address of %s", ErrUnknownHousehold, h.ID, h.Address.SharedWith)
			}
		}
	}

	for _, h := range households {
		if h.Parent == "" {
			t.roots = append(t.roots, h.ID)
			continue
		}
		if h.Parent == h.ID {
			return nil, fmt.Errorf("%w: %s is its own parent", ErrCycle, h.ID)
		}
		if _, ok := t.byID[h.Parent]; !ok {
			return nil, fmt.Errorf("%w: %s names parent %s", ErrUnknownParent, h.ID, h.Parent)
		}
		t.children[h.Parent] = append(t.children[h.Parent], h.ID)
	}

	// Every Household must reach a root by following parent links. A node that
	// does not is part of a cycle: it has a valid parent, but that chain loops
	// rather than terminating, so it never appears under any root.
	if err := t.detectCycles(); err != nil {
		return nil, err
	}

	t.sortSiblings()

	return t, nil
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
