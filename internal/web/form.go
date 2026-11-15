package web

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// newAdultSlotKey and newDependentSlotKey are declared in submission.go, which
// is the file that has to recognise the names coming back off the wire. The
// form offers at most one blank slot of each kind per save: §2.1 permits
// hand-written JavaScript only for tree expand/collapse, so there is no "add
// another row" control, and one addition of each kind per save is enough — the
// re-rendered form offers fresh slots immediately.
//
// Do not redeclare them here.

// personFormView is one Person as the form renders them. Every field is a
// string, including the dates, because a refused submit must re-render exactly
// what the Editor typed — which by definition did not parse.
//
// Errors is keyed by field name and holds input that could not be stored. It is
// distinct from a Finding: a Finding observes a value that *is* stored and
// never blocks a save (§4.5).
type personFormView struct {
	Key string

	Given     string
	Surname   string
	BirthName string
	Aka       string
	Phone     string
	Email     string
	Birth     string
	Death     string

	HiddenPhone bool
	HiddenEmail bool
	HiddenBirth bool

	Deceased bool
	IsAdult  bool
	IsNew    bool

	// Promote and Remove are the structural checkboxes. They are false on a
	// GET and carried from the submission on a refusal, so a pending change
	// the Editor ticked is still ticked when the form comes back.
	Promote bool
	Remove  bool

	// Lifespan is the "1963–1981" a memorial stamp shows; empty when living.
	Lifespan string

	// Findings are the observations about this person, shown on their card.
	Findings []findingView

	Errors map[string]string
}

// findingView is one validation Finding as the form shows it. The message is
// rendered verbatim: §4.5 requires it, because the message was written for the
// Editor rather than for a log.
type findingView struct {
	Person   rolo.PersonID
	Field    string
	Message  string
	Severity string
}

// householdFormView is the editable detail pane. It is the single input to the
// form template, built either from the document on a GET or from the Editor's
// own submission on a refused POST — one shape, so the two paths cannot drift
// into rendering different markup.
type householdFormView struct {
	ID     rolo.HouseholdID
	Title  string
	Crumbs []crumb

	// Summary is the one line under the heading: who lives here and since
	// when. Empty when there is nothing to say, and on the refusal path,
	// which must not consult the document.
	Summary string

	Adults     []personFormView
	Dependents []personFormView

	// NewSlot is the blank adult slot, nil when the Household already holds two.
	NewSlot *personFormView
	// NewDependent is the blank Dependent slot, always offered.
	NewDependent personFormView

	AddressText   string
	AddressHidden bool
	// SharedWith is the label of the Household whose Address this one uses,
	// empty unless this is a Shared Address. §3 makes it a reference rather
	// than a copy, so the pane says where the address comes from. It is filled
	// in by householdView, which has the tree to resolve the reference against;
	// the refusal path leaves it empty because it must not consult the
	// document.
	SharedWith  string
	Anniversary string

	// Open and Pane ride through the form as hidden inputs so that a save, or a
	// refusal, puts the tree back exactly as the Editor left it. ADR-0008 keeps
	// this state in the URL; a form post is the one place it travels in a body.
	Open string
	Pane string

	// Errors holds Household-level input that could not be stored, keyed by
	// field. The empty key carries a whole-submission refusal.
	Errors map[string]string
	// Findings are the observations not filed on a person's card: those about
	// the Household itself, and any whose person has no card here.
	Findings []findingView
}

// formViewFromHousehold builds the form from what the document holds. This is
// the GET path.
func formViewFromHousehold(
	id rolo.HouseholdID,
	h rolo.Household,
	findings []rolo.Finding,
) householdFormView {
	view := householdFormView{
		ID:            id,
		AddressText:   strings.Join(h.Address.Lines, "\n"),
		AddressHidden: h.Address.Hidden,
		Anniversary:   h.Anniversary.String(),
		Adults:        personFormViews(h.Adults, true),
		Dependents:    personFormViews(h.Dependents, false),
		NewDependent:  personFormView{Key: newDependentSlotKey, IsNew: true},
		Errors:        map[string]string{},
		Summary:       householdSummary(h),
	}

	// The blank adult slot only exists while there is room for one. Offering a
	// third would invite a submission the store cannot load.
	//
	// The Dependent slot is always offered, and carries its own name: a
	// Household with one adult must still be able to gain a Dependent, or a
	// widowed parent could not record a grandchild in their care without first
	// inventing a spouse.
	onCards := make(map[rolo.PersonID]bool, len(h.Adults)+len(h.Dependents))
	for _, p := range h.Adults {
		onCards[p.ID] = true
	}

	for _, p := range h.Dependents {
		onCards[p.ID] = true
	}

	household, byPerson := splitFindings(findings, onCards)
	view.Findings = household

	for i := range view.Adults {
		view.Adults[i].Findings = byPerson[rolo.PersonID(view.Adults[i].Key)]
	}

	for i := range view.Dependents {
		view.Dependents[i].Findings = byPerson[rolo.PersonID(view.Dependents[i].Key)]
	}

	if len(h.Adults) < maxAdults {
		view.NewSlot = &personFormView{Key: newAdultSlotKey, IsNew: true, IsAdult: true}
	}

	return view
}

// formViewFromSubmission rebuilds the form from what the Editor typed. This is
// the refusal path, and it deliberately does not consult the document: the
// point is to give back the submission unchanged, with an explanation attached.
func formViewFromSubmission(
	id rolo.HouseholdID,
	sub submission,
	errs []fieldError,
) householdFormView {
	byPerson, householdErrs := groupFieldErrors(errs)

	view := householdFormView{
		ID:            id,
		AddressText:   strings.Join(sub.AddressLines, "\n"),
		AddressHidden: sub.AddressHidden,
		Anniversary:   sub.AnniversaryRaw,
		Open:          sub.Open,
		Pane:          sub.Pane,
		// Findings observe values that are stored, and the refusal path never
		// consults the document — so there is nothing to observe here, unlike
		// SharedWith above, which is left empty for the same reason.
		Errors: householdErrs,
	}

	haveNewAdult, haveNewDependent := false, false

	for _, p := range sub.People {
		form := personFormView{
			Key:         string(p.ID),
			Given:       p.Given,
			Surname:     p.Surname,
			BirthName:   p.BirthName,
			Aka:         p.Aka,
			Phone:       p.Phone,
			Email:       p.Email,
			Birth:       p.BirthRaw,
			Death:       p.DeathRaw,
			HiddenPhone: p.Hidden.Phone,
			HiddenEmail: p.Hidden.Email,
			HiddenBirth: p.Hidden.Birth,
			IsNew:       p.New,
			IsAdult:     p.WasAdult,
			Promote:     p.Promote,
			Remove:      p.Remove,
			Errors:      byPerson[p.ID],
		}

		if p.New {
			// The slot keeps the name it arrived under, so a re-rendered
			// Dependent addition is still a Dependent addition on the retry.
			form.Key = newAdultSlotKey
			form.IsAdult = true

			if p.AsDependent {
				form.Key = newDependentSlotKey
				form.IsAdult = false
				haveNewDependent = true
			} else {
				haveNewAdult = true
			}
		}

		// The submission does not say which group an existing person came from
		// for rendering position, and the refusal path must not consult the
		// document to find out — so everyone renders in the adult list. A
		// refused submit is a corrective moment, not a navigation one, and the
		// grouping returns on the next good save. IsAdult, however, is carried
		// through from the submission itself (see personSubmission.WasAdult),
		// so the promote control still shows only for Dependents.
		view.Adults = append(view.Adults, form)
	}

	// The blank new-person slots are offered unless the submission already
	// carries that slot's person: that person is already re-rendered above
	// with their typing intact, and a second slot with the same field names
	// would collide. Both slots carry the same hazard, so both are guarded.
	if !haveNewAdult {
		view.NewSlot = &personFormView{Key: newAdultSlotKey, IsNew: true, IsAdult: true}
	}

	if !haveNewDependent {
		view.NewDependent = personFormView{Key: newDependentSlotKey, IsNew: true}
	}

	return view
}

// groupFieldErrors sorts refused input into the per-person and Household-level
// maps the form view carries.
func groupFieldErrors(errs []fieldError) (map[rolo.PersonID]map[string]string, map[string]string) {
	byPerson := make(map[rolo.PersonID]map[string]string)
	householdErrs := map[string]string{}

	for _, e := range errs {
		if e.Person == "" {
			householdErrs[e.Field] = e.Message
			continue
		}

		if byPerson[e.Person] == nil {
			byPerson[e.Person] = map[string]string{}
		}

		byPerson[e.Person][e.Field] = e.Message
	}

	return byPerson, householdErrs
}

// personFormViews renders a group of people as form rows.
func personFormViews(people []rolo.Person, adults bool) []personFormView {
	views := make([]personFormView, 0, len(people))

	for _, p := range people {
		views = append(views, personFormView{
			Key:       string(p.ID),
			Given:     p.Given,
			Surname:   p.Surname,
			BirthName: p.BirthName,
			Aka:       p.Aka,
			// Every stored value is rendered: ADR-0010 keeps masking out of
			// this pane entirely.
			Phone:       p.Phone,
			Email:       p.Email,
			Birth:       p.Birth.String(),
			Death:       p.Death.String(),
			HiddenPhone: p.Hidden.Phone,
			HiddenEmail: p.Hidden.Email,
			HiddenBirth: p.Hidden.Birth,
			Deceased:    p.IsDeceased(),
			Lifespan:    lifespan(p),
			IsAdult:     adults,
			Errors:      map[string]string{},
		})
	}

	return views
}

// findingViews renders validation findings for display, verbatim.
func findingViews(findings []rolo.Finding) []findingView {
	views := make([]findingView, 0, len(findings))

	for _, f := range findings {
		views = append(views, findingView{
			Person:   f.Person,
			Field:    f.Field,
			Message:  f.Message,
			Severity: f.Severity.String(),
		})
	}

	return views
}

// splitFindings files each Finding with the person it is about, so the form
// shows it on that person's card. A Finding about the Household, or about a
// person with no card here, stays with the Household: a Finding is never
// dropped for want of somewhere to show it.
func splitFindings(
	findings []rolo.Finding,
	onCards map[rolo.PersonID]bool,
) ([]findingView, map[rolo.PersonID][]findingView) {
	var household []findingView

	byPerson := map[rolo.PersonID][]findingView{}

	for _, f := range findingViews(findings) {
		if f.Person == "" || !onCards[f.Person] {
			household = append(household, f)
			continue
		}

		byPerson[f.Person] = append(byPerson[f.Person], f)
	}

	return household, byPerson
}

// countWords spells small counts the way a person would say them. A
// Household with more than ten of anything is rare enough that digits read
// fine.
var countWords = []string{"no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}

func countPhrase(n int, singular, plural string) string {
	word := strconv.Itoa(n)
	if n < len(countWords) {
		word = countWords[n]
	}

	if n == 1 {
		return word + " " + singular
	}

	return word + " " + plural
}

// householdSummary is the line under a Household's heading, e.g. "Two adults,
// one dependent · Married June 14, 1962". Each part starts with a capital
// because the middle dot separates them like sentences.
func householdSummary(h rolo.Household) string {
	var counts []string
	if n := len(h.Adults); n > 0 {
		counts = append(counts, countPhrase(n, "adult", "adults"))
	}

	if n := len(h.Dependents); n > 0 {
		counts = append(counts, countPhrase(n, "dependent", "dependents"))
	}

	var parts []string
	if len(counts) > 0 {
		parts = append(parts, capitalize(strings.Join(counts, ", ")))
	}

	if d := longDate(h.Anniversary); d != "" {
		parts = append(parts, "Married "+d)
	}

	return strings.Join(parts, " · ")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

// longDate writes a date as a letter would: "June 14, 1962", "June 1962" or
// "1962". rolo.Date.String stays ISO because the form and the store read it.
func longDate(d rolo.Date) string {
	switch {
	case !d.HasYear():
		return ""
	case d.Month == 0:
		return strconv.Itoa(d.Year)
	case d.Day == 0:
		return fmt.Sprintf("%s %d", time.Month(d.Month), d.Year)
	default:
		return fmt.Sprintf("%s %d, %d", time.Month(d.Month), d.Day, d.Year)
	}
}

// lifespan is the span a memorial stamp shows. Only years: the stamp is a
// reminder, and the full dates are in the fields beneath it.
func lifespan(p rolo.Person) string {
	switch {
	case !p.Death.HasYear():
		return ""
	case !p.Birth.HasYear():
		return "died " + strconv.Itoa(p.Death.Year)
	default:
		return fmt.Sprintf("%d–%d", p.Birth.Year, p.Death.Year)
	}
}
