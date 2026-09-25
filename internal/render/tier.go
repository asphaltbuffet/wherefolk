package render

// Tier is the audience a Directory is built for, named for what the recipient
// will do with it rather than by a sensitivity number (§5.2, ADR-0003).
//
// Tiers are progressive — each admits everything the one before it does — so
// the admission rules below compare against a threshold. The zero Tier is not
// Mail: it admits nothing and truncates every living person's dates, so a
// caller that forgets to choose a tier fails closed.
type Tier int

// The four Directory tiers, least to most revealing. Proof (§5.6) is a
// different kind of export and is not a Tier.
const (
	Mail Tier = iota + 1
	Call
	Digital
	Full
)

// String is the tier's name as the footer prints it.
func (t Tier) String() string {
	switch t {
	case Mail:
		return "Mail"
	case Call:
		return "Call"
	case Digital:
		return "Digital"
	case Full:
		return "Full"
	default:
		return ""
	}
}

// admitsPhone reports whether the tier prints phone numbers at all. Outside
// Full, a minor's is suppressed regardless.
func (t Tier) admitsPhone() bool { return t >= Call }

// admitsEmail reports whether the tier prints email addresses at all. Outside
// Full, a minor's is suppressed regardless.
func (t Tier) admitsEmail() bool { return t >= Digital }

// full reports whether this is the inner-circle tier: every living person's
// dates Whole, minors' contact details included, DO NOT DISTRIBUTE on every
// page.
func (t Tier) full() bool { return t == Full }
