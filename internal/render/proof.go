package render

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// A Proof Sheet is one Household's own entry, sent to them to correct
// (CONTEXT.md, Proof Sheet; ADR-0005). BuildProof is to it what Build is to a
// Directory: the one place every rule about what the sheet shows is applied,
// while values are still rolo types.
//
// The recipient is the data subject, so nothing is suppressed or truncated:
// dates are Whole and withheld values are shown, marked. A field or check with
// an empty ID is not on the sheet.

var (
	// ErrNoSuchHousehold reports an ID that is not in the tree.
	ErrNoSuchHousehold = errors.New("render: no such household")
	// ErrNoLivingMember reports a Household nobody could receive a sheet for.
	ErrNoLivingMember = errors.New("render: household has no living member")
)

const (
	// proofAddressLines is how many address lines a sheet offers at least, so a
	// recipient who has moved has room to write the new one.
	proofAddressLines = 4
	// proofMissing is how many blank person blocks "Anyone missing?" offers.
	proofMissing = 2
)

// ProofField is one pre-filled form field. Withheld prints the marker beside it.
type ProofField struct {
	ID       string
	Value    string
	Withheld bool
}

// ProofCheck is one checkbox, ticked or not.
type ProofCheck struct {
	ID      string
	Checked bool
}

// ProofPerson is one person's block on a sheet. A deceased person carries names
// and dates only; a blank "Anyone missing?" block carries a Relationship field
// and no Remove box.
type ProofPerson struct {
	Deceased bool

	Given, Surname, Aka, BirthName ProofField
	Birth, Death                   ProofField
	Phone, Email                   ProofField
	Relationship                   ProofField

	KeepBirth, KeepPhone, KeepEmail ProofCheck
	Remove                          ProofCheck
}

// ProofSheet is one Household's Proof Sheet as strings and field IDs.
type ProofSheet struct {
	Title       string
	GeneratedAt string
	Reply       string

	HouseholdID string
	Name        string

	Address         []ProofField
	AddressWithheld bool
	// SharedWith is the Household Name of the Household this one's Address
	// refers to; empty unless it is a Shared Address.
	SharedWith  string
	KeepAddress ProofCheck
	Moved       ProofCheck

	Anniversary ProofField

	Adults, Dependents, Missing []ProofPerson

	AllCorrect ProofCheck
	Notes      ProofField
}

// BuildProof renders the Proof Sheet for the Household id.
func BuildProof(t *rolo.Tree, id rolo.HouseholdID, asOf time.Time, title, reply string) (ProofSheet, error) {
	h, ok := t.Get(id)
	if !ok {
		return ProofSheet{}, ErrNoSuchHousehold
	}

	if !h.HasLivingMember() {
		return ProofSheet{}, ErrNoLivingMember
	}

	if strings.TrimSpace(title) == "" {
		title = DefaultTitle
	}

	lines, withheld, shared := proofAddress(t, h)

	s := ProofSheet{
		Title:           title,
		GeneratedAt:     asOf.Format(dateStamp),
		Reply:           reply,
		HouseholdID:     string(h.ID),
		Name:            h.Name(),
		Address:         addressFields(lines),
		AddressWithheld: withheld,
		KeepAddress:     ProofCheck{ID: "address-keep", Checked: withheld},
		Adults:          proofPeople(h.Adults),
		Dependents:      proofPeople(h.Dependents),
		AllCorrect:      ProofCheck{ID: "all-correct"},
		Notes:           ProofField{ID: "notes"},
	}

	if h.SharesAddress() {
		s.SharedWith = shared
		s.Moved = ProofCheck{ID: "address-moved"}
	}

	if len(h.Adults) > 1 || !h.Anniversary.IsZero() {
		s.Anniversary = ProofField{ID: "anniversary", Value: wholeDate(h.Anniversary)}
	}

	for i := range proofMissing {
		s.Missing = append(s.Missing, missingPerson("missing"+strconv.Itoa(i+1)))
	}

	return s, nil
}

// proofAddress follows h's Address to the lines it stands for, ignoring every
// withheld flag on the way (ADR-0014), and reports whether any link was
// withheld and the Household Name of the first referenced Household.
//
// seen guards a cycle of Shared Addresses, which BuildTree does not reject.
func proofAddress(t *rolo.Tree, h rolo.Household) ([]string, bool, string) {
	var (
		withheld bool
		shared   string
	)

	seen := map[rolo.HouseholdID]bool{}
	cur := h

	for {
		withheld = withheld || cur.AddressHidden()

		if !cur.SharesAddress() {
			return cur.Address.Lines, withheld, shared
		}

		if seen[cur.ID] {
			return nil, withheld, shared
		}

		seen[cur.ID] = true

		target, ok := t.Get(cur.Address.SharedWith)
		if shared == "" {
			shared = string(cur.Address.SharedWith)
			if ok {
				shared = target.Name()
			}
		}

		if !ok {
			return nil, withheld, shared
		}

		cur = target
	}
}

// addressFields offers each stored line plus blanks up to proofAddressLines.
func addressFields(lines []string) []ProofField {
	out := make([]ProofField, max(len(lines), proofAddressLines))
	for i := range out {
		out[i].ID = "address-" + strconv.Itoa(i+1)
		if i < len(lines) {
			out[i].Value = lines[i]
		}
	}

	return out
}

func proofPeople(ps []rolo.Person) []ProofPerson {
	out := make([]ProofPerson, 0, len(ps))
	for _, p := range ps {
		out = append(out, proofPerson(p))
	}

	return out
}

// proofPerson renders one stored person. The withheld marker appears only on a
// value that exists, as in the Directory (CONTEXT.md, Withheld field); the box
// reflects the stored flag either way.
func proofPerson(p rolo.Person) ProofPerson {
	k := string(p.ID)

	out := ProofPerson{
		Deceased:  p.IsDeceased(),
		Given:     ProofField{ID: k + "-given", Value: p.Given},
		Surname:   ProofField{ID: k + "-surname", Value: p.Surname},
		Aka:       ProofField{ID: k + "-aka", Value: p.Aka},
		BirthName: ProofField{ID: k + "-birth-name", Value: p.BirthName},
		Birth: ProofField{
			ID: k + "-birth", Value: wholeDate(p.Birth),
			Withheld: p.Hidden.Birth && !p.Birth.IsZero(),
		},
	}

	if out.Deceased {
		out.Death = ProofField{ID: k + "-death", Value: wholeDate(p.Death)}
		return out
	}

	out.Phone = ProofField{ID: k + "-phone", Value: p.Phone, Withheld: p.Hidden.Phone && p.Phone != ""}
	out.Email = ProofField{ID: k + "-email", Value: p.Email, Withheld: p.Hidden.Email && p.Email != ""}
	out.KeepBirth = ProofCheck{ID: k + "-keep-birth", Checked: p.Hidden.Birth}
	out.KeepPhone = ProofCheck{ID: k + "-keep-phone", Checked: p.Hidden.Phone}
	out.KeepEmail = ProofCheck{ID: k + "-keep-email", Checked: p.Hidden.Email}
	out.Remove = ProofCheck{ID: k + "-remove"}

	return out
}

// missingPerson is a blank block for someone the Directory does not hold yet.
func missingPerson(k string) ProofPerson {
	return ProofPerson{
		Given:        ProofField{ID: k + "-given"},
		Surname:      ProofField{ID: k + "-surname"},
		Aka:          ProofField{ID: k + "-aka"},
		BirthName:    ProofField{ID: k + "-birth-name"},
		Birth:        ProofField{ID: k + "-birth"},
		Phone:        ProofField{ID: k + "-phone"},
		Email:        ProofField{ID: k + "-email"},
		Relationship: ProofField{ID: k + "-relationship"},
		KeepBirth:    ProofCheck{ID: k + "-keep-birth"},
		KeepPhone:    ProofCheck{ID: k + "-keep-phone"},
		KeepEmail:    ProofCheck{ID: k + "-keep-email"},
	}
}

// FormValues is every field on the sheet by ID: text values, and checkbox
// states. The renderer checks the template placed exactly these.
func (s ProofSheet) FormValues() (map[string]string, map[string]bool) {
	texts := map[string]string{}
	checks := map[string]bool{}

	addText := func(fs ...ProofField) {
		for _, f := range fs {
			if f.ID != "" {
				texts[f.ID] = f.Value
			}
		}
	}

	addCheck := func(cs ...ProofCheck) {
		for _, c := range cs {
			if c.ID != "" {
				checks[c.ID] = c.Checked
			}
		}
	}

	addText(s.Address...)
	addText(s.Anniversary, s.Notes)
	addCheck(s.KeepAddress, s.Moved, s.AllCorrect)

	for _, group := range [][]ProofPerson{s.Adults, s.Dependents, s.Missing} {
		for _, p := range group {
			addText(p.Given, p.Surname, p.Aka, p.BirthName, p.Birth, p.Death, p.Phone, p.Email, p.Relationship)
			addCheck(p.KeepBirth, p.KeepPhone, p.KeepEmail, p.Remove)
		}
	}

	return texts, checks
}
