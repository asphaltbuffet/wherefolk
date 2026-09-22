// Package store persists the Directory as a single atomically-written JSON
// document and generates the stable identities its records carry.
package store

import (
	"fmt"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

const (
	// IDAlphabet is Crockford base32 in lowercase: the digits and the letters,
	// less i, l, o and u. Those are the characters people confuse when reading
	// an ID aloud or retyping one while repairing the document by hand.
	IDAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

	// IDSize is the number of random characters after the prefix. 32^6 is
	// about a billion values, which is ample for a directory of a few hundred
	// people; the store still rejects a collision at load rather than assuming.
	IDSize = 6

	personPrefix    = "p_"
	householdPrefix = "h_"
)

// NewPersonID generates a stable identity for a Person.
func NewPersonID() (rolo.PersonID, error) {
	body, err := gonanoid.Generate(IDAlphabet, IDSize)
	if err != nil {
		return "", fmt.Errorf("generate person id: %w", err)
	}
	return rolo.PersonID(personPrefix + body), nil
}

// NewHouseholdID generates a stable identity for a Household.
func NewHouseholdID() (rolo.HouseholdID, error) {
	body, err := gonanoid.Generate(IDAlphabet, IDSize)
	if err != nil {
		return "", fmt.Errorf("generate household id: %w", err)
	}
	return rolo.HouseholdID(householdPrefix + body), nil
}
