package web

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// Form field names are ID-keyed: person.{personID}.{field}. The Household's own
// fields carry no person segment. A new person uses a slot name in place of an
// ID — person.new1.given — and is given a real identity only when the
// submission is applied.
const (
	personPrefix   = "person."
	newSlotPrefix  = "new"
	addressLines   = "address.lines"
	addressHidden  = "address.hidden"
	anniversaryKey = "anniversary"
	openKey        = "open"
	paneKey        = "pane"
)

// checkedValue is what a ticked checkbox submits. Every checkbox is preceded by
// a hidden input carrying uncheckedValue, because a browser submits nothing at
// all for an unticked box — without the pair, "unticked" and "not in this form"
// would be the same absence and a hidden flag could never be turned off.
const (
	checkedValue   = "on"
	uncheckedValue = "off"
)

// personSubmission is one person as the form submitted them. Dates carry both
// the parsed value and the raw text: a value that would not parse must survive
// into the re-rendered form so the Editor sees what they typed.
type personSubmission struct {
	ID  rolo.PersonID
	New bool

	Given     string
	Surname   string
	BirthName string
	Aka       string
	Phone     string
	Email     string

	Birth    rolo.Date
	BirthRaw string
	Death    rolo.Date
	DeathRaw string

	Hidden rolo.HiddenFields

	Remove  bool
	Promote bool
}

// submission is one Household's form, parsed but not yet applied.
type submission struct {
	People []personSubmission

	AddressLines   []string
	AddressHidden  bool
	Anniversary    rolo.Date
	AnniversaryRaw string

	// Open and Pane carry the tree's state through the round-trip so that a
	// save — or a refusal — puts the Editor's expanded Branches back exactly as
	// they were. ADR-0008 keeps navigation state in the URL, and a form post is
	// the one place it has to travel through a body to get there.
	Open string
	Pane string
}

// fieldError is input that cannot be stored at all — a date that yields no
// Date. It is not a Finding: a Finding observes a value that *is* stored, and
// there is nothing here to observe. The submit is refused and the raw value is
// re-rendered. See §4.5.
type fieldError struct {
	Person  rolo.PersonID
	Field   string
	Value   string
	Message string
}

// parseSubmission reads a posted form against the Household being edited.
//
// Keys naming a person who is not in that Household are ignored rather than
// rejected: the Editor may have had the page open while the document changed
// underneath them, which is an ordinary occurrence and not an attack. A person
// in the Household but absent from the form keeps their stored values, so a
// stale form cannot silently blank someone.
func parseSubmission(form url.Values, h rolo.Household) (submission, []fieldError) {
	var (
		sub  submission
		errs []fieldError
	)

	sub.Open = form.Get(openKey)
	sub.Pane = form.Get(paneKey)

	sub.AddressLines = splitLines(form.Get(addressLines))
	sub.AddressHidden = checkbox(form[addressHidden])

	sub.AnniversaryRaw = strings.TrimSpace(form.Get(anniversaryKey))

	anniversary, err := rolo.ParseDate(sub.AnniversaryRaw)
	if err != nil {
		errs = append(errs, fieldError{
			Field:   "anniversary",
			Value:   sub.AnniversaryRaw,
			Message: dateMessage(sub.AnniversaryRaw),
		})
	} else {
		sub.Anniversary = anniversary
	}

	known := make(map[rolo.PersonID]bool)
	for _, p := range h.Adults {
		known[p.ID] = true
	}
	for _, p := range h.Dependents {
		known[p.ID] = true
	}

	for _, key := range personKeys(form) {
		isNew := strings.HasPrefix(key, newSlotPrefix)
		if !isNew && !known[rolo.PersonID(key)] {
			continue
		}

		person, personErrs := parsePerson(form, key, isNew)

		// A slot the Editor left untouched is not an addition. Given is the
		// test because a person with no given name has no name at all — every
		// other field is optional.
		if isNew && person.Given == "" {
			continue
		}

		sub.People = append(sub.People, person)
		errs = append(errs, personErrs...)
	}

	return sub, errs
}

// parsePerson reads one person.{key}.* group. key is a PersonID for an existing
// person and a slot name for a new one.
func parsePerson(form url.Values, key string, isNew bool) (personSubmission, []fieldError) {
	field := func(name string) string {
		return strings.TrimSpace(form.Get(personPrefix + key + "." + name))
	}

	person := personSubmission{
		New:       isNew,
		Given:     field("given"),
		Surname:   field("surname"),
		BirthName: field("birth_name"),
		Aka:       field("aka"),
		Phone:     field("phone"),
		Email:     field("email"),
		BirthRaw:  field("birth"),
		DeathRaw:  field("death"),
		Hidden: rolo.HiddenFields{
			Phone: checkbox(form[personPrefix+key+".hidden.phone"]),
			Email: checkbox(form[personPrefix+key+".hidden.email"]),
			Birth: checkbox(form[personPrefix+key+".hidden.birth"]),
		},
		Remove:  checkbox(form[personPrefix+key+".remove"]),
		Promote: checkbox(form[personPrefix+key+".promote"]),
	}

	if !isNew {
		person.ID = rolo.PersonID(key)
	}

	var errs []fieldError

	birth, err := rolo.ParseDate(person.BirthRaw)
	if err != nil {
		errs = append(errs, fieldError{
			Person:  person.ID,
			Field:   "birth",
			Value:   person.BirthRaw,
			Message: dateMessage(person.BirthRaw),
		})
	} else {
		person.Birth = birth
	}

	death, err := rolo.ParseDate(person.DeathRaw)
	if err != nil {
		errs = append(errs, fieldError{
			Person:  person.ID,
			Field:   "death",
			Value:   person.DeathRaw,
			Message: dateMessage(person.DeathRaw),
		})
	} else {
		person.Death = death
	}

	return person, errs
}

// dateMessage explains an unreadable date in the Editor's own terms. §4.5 fixes
// the register: plain English, no library internals, and a hint at what to
// change.
func dateMessage(value string) string {
	return fmt.Sprintf(
		"%q is not a date this can read. Use a year (1998), a year and month (1998-06), "+
			"or a full date (1998-06-14). Leave it empty if it is not known.",
		value,
	)
}

// personKeys returns the person segments present in the form, in a stable
// order. Sorting keeps a submission's people in the same order across requests,
// which matters because the order decides which minted ID lands on whom.
func personKeys(form url.Values) []string {
	seen := make(map[string]bool)

	for name := range form {
		if !strings.HasPrefix(name, personPrefix) {
			continue
		}

		rest := strings.TrimPrefix(name, personPrefix)

		key, _, found := strings.Cut(rest, ".")
		if !found || key == "" {
			continue
		}

		seen[key] = true
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}

	// Existing people sort before new slots, each group ordered
	// lexicographically, so an existing person's position never shifts
	// depending on how many new-person slots the form also carried.
	sort.Slice(keys, func(i, j int) bool {
		iNew := strings.HasPrefix(keys[i], newSlotPrefix)
		jNew := strings.HasPrefix(keys[j], newSlotPrefix)
		if iNew != jNew {
			return !iNew
		}
		return keys[i] < keys[j]
	})

	return keys
}

// checkbox resolves a checkbox's paired inputs. The hidden "off" is always
// submitted; a ticked box adds "on" after it, so the last value wins.
func checkbox(values []string) bool {
	return len(values) > 0 && values[len(values)-1] == checkedValue
}

// splitLines turns a textarea's contents into address lines, dropping blanks.
// Normalisation trims them again later; doing it here keeps a submission
// comparable in tests without a normalisation pass.
func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	var lines []string

	for line := range strings.SplitSeq(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	return lines
}
