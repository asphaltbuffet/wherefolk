package web

import (
	"net/url"
	"strings"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// toggleURL builds the link that expands or collapses one node.
//
// Every value goes through [url.Values.Encode], so an ID carrying "&", "=", or a
// space cannot break the query apart. Building this here rather than in the
// template is deliberate: html/template percent-encodes into href, which it
// recognises as a URL attribute, but hx-get is an attribute it knows nothing
// about and would receive HTML escaping only.
//
// Collapsing is expressed as "everything currently open, minus this one" so the
// link is a plain URL with no state of its own.
func toggleURL(selected rolo.HouseholdID, openList string, id rolo.HouseholdID, isOpen bool) string {
	q := url.Values{}

	if selected != "" {
		q.Set("selected", string(selected))
	}

	switch {
	case isOpen:
		q.Set("open", openList)
		q.Set("close", string(id))
	case openList == "":
		q.Set("open", string(id))
	default:
		q.Set("open", openList+","+string(id))
	}

	return "/tree?" + q.Encode()
}

// treeNode is one row in the tree pane.
//
// Children is populated only when Open, so the rendered tree contains exactly
// what is visible. That keeps a deep family's first paint small and means the
// template needs no visibility logic of its own — if a node has children in the
// view model, they are on screen.
//
// HasChildren is therefore separate from len(Children): a collapsed node still
// needs a disclosure triangle, and the template cannot infer one from the other.
type treeNode struct {
	ID          rolo.HouseholdID
	Label       string
	Memorial    bool
	Selected    bool
	Open        bool
	HasChildren bool
	Children    []treeNode

	// Pinned marks a node the Editor cannot collapse: the selection and its
	// ancestors, which treeView re-opens after any close request so the tree
	// can never hide the Household the detail pane is showing. The template
	// renders these without a toggle control, because advertising a Collapse
	// affordance that cannot collapse anything is worse than showing none.
	Pinned bool

	// ToggleURL expands or collapses this node. It is built here, with
	// url.Values, rather than concatenated in the template: html/template
	// percent-encodes into href because it recognises it as a URL attribute,
	// but hx-get is an attribute it knows nothing about, so an ID containing
	// "&" or a space would be HTML-escaped only and would then break the query
	// apart in the browser. IDs are not guaranteed safe — store.Load accepts a
	// hand-repaired document exactly as written, and validation in this project
	// observes rather than rejects.
	//
	// Empty when the node is Pinned or childless, in which case the template
	// renders no control at all.
	ToggleURL string
}

// treeNodes builds the visible tree. selected is the Household the detail pane
// is showing, which may be empty; open is the set of Households whose children
// are expanded.
//
// It renders the open set it is given and does not compute one. Deciding what
// is open — in particular the rule that a selection's ancestors are always
// open, so the tree can never hide the Household the detail pane is showing —
// belongs to openSet and treeView. Duplicating that rule here would put it in
// two places that could drift, and would silently override treeView's
// deliberate ordering, which applies a close request before re-adding the
// selection's chain.
//
// Callers hold at least a read lock.
func (s *Server) treeNodes(selected rolo.HouseholdID, open map[rolo.HouseholdID]bool) []treeNode {
	// The selection's chain is pinned: treeView re-opens it after any close, so
	// a toggle on one of these nodes would render identically to not clicking
	// it at all.
	pinned := make(map[rolo.HouseholdID]bool)
	for _, id := range s.selectionChain(selected) {
		pinned[id] = true
	}

	openList := s.joinIDsOrdered(open)

	var build func(households []rolo.Household) []treeNode

	build = func(households []rolo.Household) []treeNode {
		nodes := make([]treeNode, 0, len(households))

		for _, h := range households {
			children := s.tree.Children(h.ID)

			node := treeNode{
				ID:          h.ID,
				Label:       h.Label(),
				Memorial:    h.IsMemorial(),
				Selected:    h.ID == selected,
				Open:        open[h.ID],
				HasChildren: len(children) > 0,
				Pinned:      pinned[h.ID],
			}

			if node.HasChildren && !node.Pinned {
				node.ToggleURL = toggleURL(selected, openList, h.ID, node.Open)
			}

			if node.Open {
				node.Children = build(children)
			}

			nodes = append(nodes, node)
		}

		return nodes
	}

	return build(s.tree.Roots())
}

// selectionChain returns the Households that must be open for selected to be
// visible: itself and every ancestor. The selection itself is included so its
// children are visible — an Editor who has navigated to a Household is usually
// on their way further down.
//
// An unknown or empty selection yields nothing, which leaves the tree rendering
// normally beside whatever the caller puts in the detail pane.
//
// Callers hold at least a read lock.
func (s *Server) selectionChain(selected rolo.HouseholdID) []rolo.HouseholdID {
	if selected == "" {
		return nil
	}

	chain, err := s.tree.Path(selected)
	if err != nil {
		return nil
	}

	ids := make([]rolo.HouseholdID, 0, len(chain))
	for _, h := range chain {
		ids = append(ids, h.ID)
	}

	return ids
}

// openSet resolves which Households are expanded, given an explicit set and a
// selection.
//
// Every ancestor of the selection is always open, regardless of what the request
// asked for: a selected Household the Editor cannot see in the tree would break
// §4.3's promise that search relocates them *within* their mental model rather
// than bypassing it. The explicit set from the query string is layered under it,
// so collapsing a Branch the Editor is not standing in still works.
//
// See treeView for the variant that also honours a close request.
//
// Callers hold at least a read lock.
func (s *Server) openSet(selected rolo.HouseholdID, raw string) map[rolo.HouseholdID]bool {
	open := make(map[rolo.HouseholdID]bool)

	for _, id := range parseIDs(raw) {
		if _, ok := s.tree.Get(id); ok {
			open[id] = true
		}
	}

	for _, id := range s.selectionChain(selected) {
		open[id] = true
	}

	return open
}

// parseIDs splits a comma-separated list of Household IDs, discarding empties.
// It does no validation: openSet checks each against the tree, because an ID
// from a stale bookmark is an ordinary occurrence and not an error.
func parseIDs(raw string) []rolo.HouseholdID {
	if raw == "" {
		return nil
	}

	var ids []rolo.HouseholdID

	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		ids = append(ids, rolo.HouseholdID(part))
	}

	return ids
}

// joinIDsOrdered renders an open set as the comma-separated string the query
// string carries. The order is the tree's own depth-first order rather than map
// order, so the same expansion always produces the same URL and the Editor's
// history does not fill with URLs that differ only by shuffling.
func (s *Server) joinIDsOrdered(open map[rolo.HouseholdID]bool) string {
	var ids []string

	_ = s.tree.Walk(func(h rolo.Household, _ int) error {
		if open[h.ID] {
			ids = append(ids, string(h.ID))
		}
		return nil
	})

	return strings.Join(ids, ",")
}

// Private is what a withheld field renders as. §5.5 fixes the literal string in
// preference to a lock glyph: it survives any font stack, needs no legend, and
// reads correctly aloud to a screen reader.
const Private = "[private]"

// crumb is one step in the Path breadcrumb. Every crumb is navigable, because
// walking back up the Path is how the Editor gets from a cousin to an uncle.
type crumb struct {
	ID    rolo.HouseholdID
	Label string
}

// personView is one Person as the detail pane shows them. Every field is a
// rendered string rather than a domain value: a withheld phone number must not
// reach the template at all, so the substitution happens here where it is
// testable without parsing HTML.
type personView struct {
	Name     string
	Birth    string
	Death    string
	Phone    string
	Email    string
	Deceased bool
}

// householdView is the detail pane. It holds strings, not rolo types, for the
// same reason personView does: withheld values are replaced before rendering,
// not hidden by the template.
type householdView struct {
	ID       rolo.HouseholdID
	Title    string
	Crumbs   []crumb
	Memorial bool

	AddressLines   []string
	AddressPrivate bool
	// AddressNote is the single-line form of the address row — the [private]
	// marker or a Shared Address back-reference. It exists so the template
	// interpolates one value instead of hardcoding the marker text, which would
	// put a second copy of Private where drift is least likely to be noticed.
	// Empty when the Household has ordinary address lines, or none at all.
	AddressNote string
	// SharedWith is the label of the Household whose Address this one uses,
	// empty unless this is a Shared Address. §3 makes it a reference rather than
	// a copy, so the pane shows where the address comes from.
	SharedWith string

	Anniversary string

	Adults     []personView
	Dependents []personView
}

// directoryView is the whole two-pane page. Household is nil when nothing is
// selected — the Editor's first visit — and NotFound distinguishes that from a
// selection that does not exist, which needs an explanation rather than an
// empty pane.
type directoryView struct {
	Tree      treeView
	Household *householdView
	NotFound  bool

	// Results is the search result list. It is empty on first load and after a
	// navigation, because the tree — not the last search — is where the Editor
	// is. The fragment is rendered inline anyway so htmx has a target to swap.
	Results resultsView
}

// householdView builds the detail pane for one Household, reporting false if it
// is not in the tree. Callers hold at least a read lock.
func (s *Server) householdView(id rolo.HouseholdID) (householdView, bool) {
	h, ok := s.tree.Get(id)
	if !ok {
		return householdView{}, false
	}

	chain, err := s.tree.Path(id)
	if err != nil {
		// Unreachable: Get and Path fail on exactly the same condition.
		return householdView{}, false
	}

	crumbs := make([]crumb, 0, len(chain))
	for _, ancestor := range chain {
		crumbs = append(crumbs, crumb{ID: ancestor.ID, Label: ancestor.Label()})
	}

	view := householdView{
		ID:          id,
		Title:       householdTitle(h),
		Crumbs:      crumbs,
		Memorial:    h.IsMemorial(),
		Anniversary: h.Anniversary.String(),
		Adults:      peopleViews(h.Adults),
		Dependents:  peopleViews(h.Dependents),
	}

	// Order matters: Hidden is tested first, so a Household that both withholds
	// its address and shares its parent's renders [private] rather than naming
	// whose address it uses — which would itself disclose the withheld fact.
	switch {
	case h.AddressHidden():
		// The lines are deliberately not copied into the view: a withheld value
		// that never reaches the template cannot leak through a future change
		// to the markup. AddressNote carries the marker so the Private constant
		// stays the single source of that string.
		view.AddressPrivate = true
		view.AddressNote = Private
	case h.SharesAddress():
		// Unreachable with a loaded document: BuildTree rejects a dangling
		// SharedWith, and web.New surfaces that as a startup error, so a broken
		// reference never reaches a request. The check keeps the zero value
		// meaningful for a Server built directly in a test.
		if parent, found := s.tree.Get(h.Address.SharedWith); found {
			view.SharedWith = parent.Label()
			view.AddressNote = "Same address as " + parent.Label()
		}
	default:
		view.AddressLines = h.Address.Lines
	}

	return view, true
}

// householdTitle renders the Household's heading — the adults' display names
// joined by an ampersand, which is how §4.1's mockup heads the detail pane. It
// differs from Label(), which uses given names only and is the tree's compact
// form.
func householdTitle(h rolo.Household) string {
	names := make([]string, 0, len(h.Adults))
	for _, a := range h.Adults {
		names = append(names, a.DisplayName())
	}

	return strings.Join(names, " & ")
}

// peopleViews renders a group of Persons, substituting Private for every field
// the Editor has withheld and blanking a deceased person's contact details.
func peopleViews(people []rolo.Person) []personView {
	views := make([]personView, 0, len(people))

	for _, p := range people {
		view := personView{
			Name:     p.DisplayName(),
			Birth:    hide(p.Birth.String(), p.Hidden.Birth),
			Death:    p.Death.String(),
			Phone:    hide(p.Phone, p.Hidden.Phone),
			Email:    hide(p.Email, p.Hidden.Email),
			Deceased: p.IsDeceased(),
		}

		// §5.4: a Memorial Household is a names-and-dates reference with no
		// contact details. The rule is per-Person, not per-Household, because
		// §5.4's reason for it is that there is nobody left to own them — and a
		// Memorial Household can still list a living Dependent, whose number
		// the Editor very much needs. Gating on the Household would hide it.
		//
		// Blanked rather than marked Private, because the two absences mean
		// different things (§5.5): [private] announces that a value is held and
		// withheld, which on a dead relative's phone number would advertise
		// exactly what suppression exists to omit. There is nothing here to act
		// on, so nothing is shown.
		if p.IsDeceased() {
			view.Phone = ""
			view.Email = ""
		}

		views = append(views, view)
	}

	return views
}

// hide replaces a withheld value with Private, and leaves an absent one absent.
//
// Absence and withholding are different facts and must render differently: a
// field nobody has recorded shows nothing, while one the Editor withheld shows
// [private] so that nobody helpfully re-collects it next year (§5.5). A death
// date is never withheld — the flags cover phone, email, and birth only.
func hide(value string, hidden bool) string {
	if value == "" {
		return ""
	}

	if hidden {
		return Private
	}

	return value
}
