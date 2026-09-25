package render

import (
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// Private is what a withheld field prints: the Editor marked it hidden, and
// the marker stops anyone "helpfully" re-collecting it next year (§5.5).
const Private = "[private]"

// Build renders every Household in t for one audience, in the depth-first
// order the printed Directory uses.
//
// This is the tier filter (§5.2–§5.5, §5.7). Every rule about what an audience
// may see is applied here, while the values are still rolo types; what leaves
// is strings, so Markup and the template cannot see an unfiltered value by
// accident. There is deliberately no unfiltered constructor.
//
// asOf is a parameter rather than [time.Now] so the caller owns the time
// dependency: age gating makes exports non-reproducible (§5.7).
func Build(t *rolo.Tree, tier Tier, asOf time.Time) Directory {
	f := filter{tree: t, tier: tier, asOf: asOf}

	d := Directory{
		GeneratedAt: asOf.Format(dateStamp),
		Tier:        tier.String(),
		Restricted:  tier.full(),
	}

	// Walk visits roots in sibling order and descends depth-first, which is
	// precisely §5.1's ordering. The callback never errors, so the returned
	// error is always nil.
	_ = t.Walk(func(h rolo.Household, _ int) error {
		d.Households = append(d.Households, f.household(h))

		return nil
	})

	return d
}

// filter carries what every rule needs: the tree for Shared Address lookups,
// the audience, and the date ages are computed at.
type filter struct {
	tree *rolo.Tree
	tier Tier
	asOf time.Time
}

// household renders one Household block.
//
// A Memorial Household's Anniversary is Whole because every adult it concerns
// has died, and it publishes no contact details for anyone in it (§5.4).
func (f filter) household(h rolo.Household) Household {
	memorial := h.IsMemorial()

	out := Household{
		Label:       h.Label(),
		Memorial:    memorial,
		Anniversary: f.date(h.Anniversary, memorial),
		Adults:      f.people(h.Adults, memorial),
		Dependents:  f.people(h.Dependents, memorial),
	}

	switch {
	case h.SharesAddress():
		out.SharedWith = sharedLabel(f.tree, h.Address.SharedWith)
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

// people renders a slice of people, preserving order.
func (f filter) people(ps []rolo.Person, memorial bool) []Person {
	if len(ps) == 0 {
		return nil
	}

	out := make([]Person, 0, len(ps))
	for _, p := range ps {
		out = append(out, f.person(p, memorial))
	}

	return out
}

// person renders one person for this audience.
//
// Suppression is decided first and prints nothing; withholding is applied only
// to what survives it, so [private] never appears where the audience would not
// have seen a value anyway (CONTEXT.md, Withheld field).
func (f filter) person(p rolo.Person, memorial bool) Person {
	deceased := p.IsDeceased()

	out := Person{
		Name:  p.DisplayName(),
		Birth: withhold(f.date(p.Birth, deceased), p.Hidden.Birth),
		Death: wholeDate(p.Death),
	}

	// Contact details belong to the living, and outside Full only to adults.
	// IsMinor treats a missing birth date as a minor, which fails closed (§5.7).
	reachable := !memorial && !deceased && (f.tier.full() || !p.IsMinor(f.asOf))
	if !reachable {
		return out
	}

	if f.tier.admitsPhone() {
		out.Phone = withhold(p.Phone, p.Hidden.Phone)
	}

	if f.tier.admitsEmail() {
		out.Email = withhold(p.Email, p.Hidden.Email)
	}

	return out
}

// date renders d Whole when it concerns only the dead or the tier is Full, and
// Truncated otherwise: a date is truncated while the person it concerns is
// living (§5.3).
func (f filter) date(d rolo.Date, concernsOnlyTheDead bool) string {
	if concernsOnlyTheDead || f.tier.full() {
		return wholeDate(d)
	}

	return truncatedDate(d)
}

// withhold replaces a value the Editor marked hidden with Private.
//
// An empty value stays empty even when hidden: the marker says "we have this
// and are not sharing it", which would be untrue of a field never recorded or
// of a year-only date that truncation has already emptied.
func withhold(v string, hidden bool) string {
	if v == "" || !hidden {
		return v
	}

	return Private
}
