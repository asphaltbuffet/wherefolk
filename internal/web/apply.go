package web

import (
	"errors"
	"fmt"
	"slices"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// maxAdults is the most adults a Household holds. §3 fixes it at two: the
// Household is the block that renders one couple.
const maxAdults = 2

// changeKind distinguishes the structural changes that are announced. A plain
// field edit is not one of them and produces no change at all.
type changeKind int

const (
	// changePromoted marks a Dependent who became a Household of their own.
	changePromoted changeKind = iota
	// changeAdded marks a Person added to the Household.
	changeAdded
	// changeRemoved marks a Person taken out of the Household.
	changeRemoved
)

// change is one structural change, phrased for display. §4.4 requires the UI to
// say plainly what happened; Household names where the Editor should look next,
// which is what the announcement links to.
//
// The undo control §4.4 promises beside this sentence is item 6's. Until it
// lands the Editor's recovery is the nightly snapshot, so Message must name the
// change precisely enough to reverse by hand.
type change struct {
	Kind      changeKind
	Person    string
	Household rolo.HouseholdID
	Message   string
}

// cloneDocument makes a copy deep enough to mutate without touching the
// original.
//
// The shallow copy a struct assignment gives would share the Households slice,
// and each Household's Adults, Dependents and Address.Lines. Mutating that copy
// would corrupt the served Directory before the save that is meant to authorise
// the change had even been attempted — which is the whole point of copying.
func cloneDocument(doc *store.Document) *store.Document {
	clone := &store.Document{
		Schema:     doc.Schema,
		Households: make([]rolo.Household, len(doc.Households)),
	}

	for i, h := range doc.Households {
		h.Adults = slices.Clone(h.Adults)
		h.Dependents = slices.Clone(h.Dependents)
		h.Address.Lines = slices.Clone(h.Address.Lines)

		clone.Households[i] = h
	}

	return clone
}

// applySubmission writes a parsed submission into doc, which must be a clone:
// it mutates in place and a failure part-way through leaves doc inconsistent.
//
// It returns the structural changes for announcement. An error means the
// submission would produce a document the store could not load — too many
// adults, or a Household with none — and nothing should be saved.
func (s *Server) applySubmission(
	doc *store.Document,
	id rolo.HouseholdID,
	sub submission,
) ([]change, error) {
	index := slices.IndexFunc(doc.Households, func(h rolo.Household) bool { return h.ID == id })
	if index < 0 {
		return nil, fmt.Errorf("apply: household %q is not in the document", id)
	}

	var changes []change

	h := doc.Households[index]

	h.Address.Lines = sub.AddressLines
	h.Address.Hidden = sub.AddressHidden
	h.Anniversary = sub.Anniversary

	// Existing people are updated in place; the submission carries only those
	// the form named, so anyone it omits keeps what the document holds.
	edits := make(map[rolo.PersonID]personSubmission)

	var additions []personSubmission

	for _, p := range sub.People {
		if p.New {
			additions = append(additions, p)
			continue
		}

		edits[p.ID] = p
	}

	adults, adultChanges := applyToGroup(h.Adults, edits, false)
	dependents, dependentChanges := applyToGroup(h.Dependents, edits, true)

	changes = append(changes, adultChanges...)
	changes = append(changes, dependentChanges...)

	// Promotion reads the ORIGINAL h.Dependents, not the rebuilt slice:
	// applyToGroup has already filtered promoted people out of `dependents`,
	// so this is where they are still visible. It runs before h.Dependents is
	// reassigned below.
	promotions, err := s.promote(doc, h, edits)
	if err != nil {
		return nil, err
	}

	changes = append(changes, promotions...)

	h.Adults = adults
	h.Dependents = dependents

	// A new person joins the adults while there is room, and becomes a
	// Dependent otherwise. §3 caps a Household at two adults, so a third is a
	// submission that cannot be stored rather than a silent demotion.
	for _, add := range additions {
		// The room check comes before the mint: minting first would burn an ID
		// on a submission that is about to be refused, which makes the IDs a
		// test sees depend on how many earlier submissions failed.
		if len(h.Adults) >= maxAdults {
			return nil, errors.New("apply: a household holds at most two adults")
		}

		newID, idErr := s.newPersonID()
		if idErr != nil {
			return nil, fmt.Errorf("apply: mint person id: %w", idErr)
		}

		person := applyPerson(rolo.Person{ID: newID}, add)
		h.Adults = append(h.Adults, person)

		changes = append(changes, change{
			Kind:      changeAdded,
			Person:    person.DisplayName(),
			Household: h.ID,
			Message: fmt.Sprintf("%s was added to %s.",
				person.DisplayName(), h.Label()),
		})
	}

	if len(h.Adults) == 0 {
		return nil, errors.New("apply: a household must keep at least one adult")
	}

	doc.Households[index] = h

	return changes, nil
}

// applyToGroup rebuilds one group of people — adults or dependents — applying
// edits and dropping anyone removed or promoted out of it.
//
// promotable says whether a Promote flag means anything for this group: only a
// Dependent can be promoted, and a Promote on an adult is ignored rather than
// treated as an error, because it can only come from a crafted form.
func applyToGroup(
	people []rolo.Person,
	edits map[rolo.PersonID]personSubmission,
	promotable bool,
) ([]rolo.Person, []change) {
	kept := make([]rolo.Person, 0, len(people))

	var changes []change

	for _, p := range people {
		edit, ok := edits[p.ID]
		if !ok {
			kept = append(kept, p)
			continue
		}

		if edit.Remove {
			changes = append(changes, change{
				Kind:   changeRemoved,
				Person: p.DisplayName(),
				Message: fmt.Sprintf("%s was removed from the directory.",
					p.DisplayName()),
			})

			continue
		}

		if promotable && edit.Promote {
			// The promoted person is appended to their new Household by the
			// caller, which has the document to append to.
			continue
		}

		kept = append(kept, applyPerson(p, edit))
	}

	return kept, changes
}

// promote moves every Dependent the submission marked into a Household of its
// own, appending each to doc and returning the announcements.
//
// It reads h.Dependents as the document holds it, because applyToGroup has
// already dropped the promoted people from the rebuilt slice — this is the last
// place they are visible. The caller must therefore call it before assigning
// the rebuilt dependents back onto h.
//
// The new Household carries only the promoted Person. §3's other two triggers —
// a spouse, an Address — are filled in on the new Household's own pane, because
// this pane governs exactly one Household. Nothing is removed from doc:
// promotion is one-way (ADR-0009).
func (s *Server) promote(
	doc *store.Document,
	h rolo.Household,
	edits map[rolo.PersonID]personSubmission,
) ([]change, error) {
	var changes []change

	for _, p := range h.Dependents {
		edit, ok := edits[p.ID]
		if !ok || !edit.Promote {
			continue
		}

		newID, err := s.newHouseholdID()
		if err != nil {
			return nil, fmt.Errorf("apply: mint household id: %w", err)
		}

		person := applyPerson(p, edit)

		doc.Households = append(doc.Households, rolo.Household{
			ID:     newID,
			Parent: h.ID,
			Adults: []rolo.Person{person},
		})

		changes = append(changes, change{
			Kind:      changePromoted,
			Person:    person.DisplayName(),
			Household: newID,
			Message: fmt.Sprintf(
				"%s now has a household of their own, beneath %s.",
				person.DisplayName(), h.Label()),
		})
	}

	return changes, nil
}

// applyPerson writes a submission's fields onto a Person, preserving the
// identity. Only the fields the form carries are written: the ID is stable and
// never submitted.
func applyPerson(p rolo.Person, edit personSubmission) rolo.Person {
	p.Given = edit.Given
	p.Surname = edit.Surname
	p.BirthName = edit.BirthName
	p.Aka = edit.Aka
	p.Phone = edit.Phone
	p.Email = edit.Email
	p.Birth = edit.Birth
	p.Death = edit.Death
	p.Hidden = edit.Hidden

	return p
}
