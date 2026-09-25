package render

import (
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// Build renders every Household in t, in the depth-first order the printed
// Directory uses.
//
// It applies no tier rules: every stored value is copied through as written.
// Item 8's tier filter replaces this constructor rather than wrapping it, so
// that withholding, suppression and truncation live in exactly one place and
// the markup generator can never see an unfiltered value by accident.
//
// asOf is a parameter rather than [time.Now] so the caller owns the time
// dependency; §5.7 makes that dependency the reason exports are
// non-reproducible.
func Build(t *rolo.Tree, asOf time.Time) Directory {
	d := Directory{GeneratedAt: asOf.Format(dateStamp)}

	// Walk visits roots in sibling order and descends depth-first, which is
	// precisely §5.1's ordering. The callback never errors, so the returned
	// error is always nil.
	_ = t.Walk(func(h rolo.Household, _ int) error {
		d.Households = append(d.Households, buildHousehold(t, h))

		return nil
	})

	return d
}

// buildHousehold renders one Household block.
func buildHousehold(t *rolo.Tree, h rolo.Household) Household {
	out := Household{
		Label:       h.Label(),
		Memorial:    h.IsMemorial(),
		Anniversary: h.Anniversary.String(),
		Adults:      buildPeople(h.Adults),
		Dependents:  buildPeople(h.Dependents),
	}

	switch {
	case h.SharesAddress():
		out.SharedWith = sharedLabel(t, h.Address.SharedWith)
	default:
		out.AddressLines = h.Address.Lines
	}

	return out
}

// sharedLabel names the Household whose address is being shared.
//
// BuildTree guarantees the target exists, so a lookup failure here would be a
// corrupt tree rather than bad data; falling back to the raw ID keeps the block
// printable instead of dropping the address entirely.
func sharedLabel(t *rolo.Tree, id rolo.HouseholdID) string {
	target, ok := t.Get(id)
	if !ok {
		return string(id)
	}

	return target.Label()
}

// buildPeople renders a slice of people, preserving order.
func buildPeople(people []rolo.Person) []Person {
	if len(people) == 0 {
		return nil
	}

	out := make([]Person, 0, len(people))
	for _, p := range people {
		out = append(out, Person{
			Name:  p.DisplayName(),
			Birth: p.Birth.String(),
			Death: p.Death.String(),
			Phone: p.Phone,
			Email: p.Email,
		})
	}

	return out
}
