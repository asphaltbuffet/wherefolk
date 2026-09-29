package render_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

const examplePath = "../../testdata/directory.json"

// asOf is the export date every test builds at. Ages below are relative to it.
var asOf = time.Date(2026, time.September, 24, 0, 0, 0, 0, time.UTC)

// exampleDirectory builds the Full-tier render model over the canonical
// example. Full is used so the markup and compile tests see every value.
func exampleDirectory(t *testing.T) render.Directory {
	t.Helper()

	doc, err := store.Load(examplePath)
	require.NoError(t, err)

	tree, err := doc.Tree()
	require.NoError(t, err)

	return render.Build(tree, render.Full, asOf)
}

// build runs Build over a synthetic set of Households.
func build(t *testing.T, tier render.Tier, hs ...rolo.Household) render.Directory {
	t.Helper()

	tree, err := rolo.BuildTree(hs)
	require.NoError(t, err)

	return render.Build(tree, tier, asOf)
}

// Fixture dates, relative to asOf (2026-09-24).
var (
	adultBirth  = rolo.Date{Year: 1965, Month: 3, Day: 12}
	minorBirth  = rolo.Date{Year: 2021, Month: 4, Day: 30}
	turns18     = rolo.Date{Year: 2008, Month: 9, Day: 24} // eighteenth birthday is asOf itself
	deathDate   = rolo.Date{Year: 2022, Month: 1, Day: 8}
	yearOnly    = rolo.Date{Year: 1938}
	anniversary = rolo.Date{Year: 1991, Month: 6, Day: 15}
)

// anchor is a living adult who keeps a fixture Household from being Memorial,
// so the person under test can sit beside them as a Dependent.
func anchor() rolo.Person {
	return rolo.Person{ID: "p_anch01", Given: "Anchor", Surname: "Test", Birth: adultBirth}
}

// subject is a fully populated living adult; rows mutate a copy of it.
func subject() rolo.Person {
	return rolo.Person{
		ID:      "p_subj01",
		Given:   "Robert",
		Surname: "Langford",
		Birth:   adultBirth,
		Phone:   "555-201-0001",
		Email:   "robert@example.com",
	}
}

func TestBuildExampleDirectory(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, d render.Directory)
	}{
		{
			name: "every household appears, flat",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				assert.Len(t, d.Households, 4, "a Household never nests inside its parent (ADR-0002)")
			},
		},
		{
			name: "households follow a depth-first walk",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				names := make([]string, 0, len(d.Households))
				for _, h := range d.Households {
					names = append(names, h.Name)
				}
				assert.Equal(t, []string{
					"Harold & June (Whitfield) Langford",
					"Robert & Susan (Marsh) Langford",
					"Daniel & Claire (Ortega) Langford",
					"Patricia Novak",
				}, names)
			},
		},
		{
			name: "the generation date, tier and restriction are stamped",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				assert.Equal(t, "Sep 24, 2026", d.GeneratedAt, "exports must say when they were made (§5.7)")
				assert.Equal(t, "Full", d.Tier)
				assert.True(t, d.Restricted, "Full carries DO NOT DISTRIBUTE (§5.2)")
			},
		},
		{
			name: "a memorial household keeps its names and dates",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				h := d.Households[0]
				assert.True(t, h.Memorial)
				assert.Equal(t, "May 23, 1953", h.Anniversary)
				require.Len(t, h.Adults, 2)
				assert.Equal(t, "Harold", h.Adults[0].Name)
				assert.Equal(t, "Feb 14, 1928", h.Adults[0].Birth)
				assert.Equal(t, "Sep 30, 2011", h.Adults[0].Death)
			},
		},
		{
			name: "a nickname renders on the row, not in the Household Name",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				require.Len(t, d.Households[3].Adults, 1)
				assert.Equal(t, `Patricia "Pat"`, d.Households[3].Adults[0].Name)
				assert.Equal(t, "Patricia Novak", d.Households[3].Name)
			},
		},
		{
			name: "a shared address points at the target's Household Name",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				assert.Equal(t, "Robert & Susan (Marsh) Langford", d.Households[2].SharedWith)
			},
		},
		{
			name: "full prints living people's dates whole and their contact details",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				robert := d.Households[1].Adults[0]
				assert.Equal(t, "Mar 12, 1965", robert.Birth)
				assert.Equal(t, "555-201-0001", robert.Phone)
				assert.Equal(t, "robert.langford@example.com", robert.Email)
			},
		},
	}

	d := exampleDirectory(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, d)
		})
	}
}

// TestBuildPerson places the person under test as a Dependent beside a living
// adult, so the Household is never Memorial and only person-level rules act.
func TestBuildPerson(t *testing.T) {
	tests := []struct {
		name   string
		tier   render.Tier
		mutate func(p *rolo.Person)
		want   render.Person
	}{
		{
			name: "mail: truncated birth, no phone, no email",
			tier: render.Mail,
			want: render.Person{Name: "Robert Langford", Birth: "Mar 12"},
		},
		{
			name: "call: adds phone",
			tier: render.Call,
			want: render.Person{Name: "Robert Langford", Birth: "Mar 12", Phone: "555-201-0001"},
		},
		{
			name: "digital: adds email",
			tier: render.Digital,
			want: render.Person{
				Name: "Robert Langford", Birth: "Mar 12",
				Phone: "555-201-0001", Email: "robert@example.com",
			},
		},
		{
			name: "full: whole birth date",
			tier: render.Full,
			want: render.Person{
				Name: "Robert Langford", Birth: "Mar 12, 1965",
				Phone: "555-201-0001", Email: "robert@example.com",
			},
		},
		{
			name:   "minor outside full: name and truncated birthday, no contact details",
			tier:   render.Digital,
			mutate: func(p *rolo.Person) { p.Birth = minorBirth },
			want:   render.Person{Name: "Robert Langford", Birth: "Apr 30"},
		},
		{
			name:   "minor in full: everything",
			tier:   render.Full,
			mutate: func(p *rolo.Person) { p.Birth = minorBirth },
			want: render.Person{
				Name: "Robert Langford", Birth: "Apr 30, 2021",
				Phone: "555-201-0001", Email: "robert@example.com",
			},
		},
		{
			name:   "an eighteenth birthday on the export date admits contact details",
			tier:   render.Call,
			mutate: func(p *rolo.Person) { p.Birth = turns18 },
			want:   render.Person{Name: "Robert Langford", Birth: "Sep 24", Phone: "555-201-0001"},
		},
		{
			name:   "no birth date fails closed as a minor",
			tier:   render.Digital,
			mutate: func(p *rolo.Person) { p.Birth = rolo.Date{} },
			want:   render.Person{Name: "Robert Langford"},
		},
		{
			name:   "year-only birth date truncates to nothing",
			tier:   render.Mail,
			mutate: func(p *rolo.Person) { p.Birth = yearOnly },
			want:   render.Person{Name: "Robert Langford"},
		},
		{
			name:   "deceased: whole dates in mail",
			tier:   render.Mail,
			mutate: func(p *rolo.Person) { p.Death = deathDate },
			want:   render.Person{Name: "Robert Langford", Birth: "Mar 12, 1965", Death: "Jan 8, 2022"},
		},
		{
			name:   "deceased: contact details suppressed even in full",
			tier:   render.Full,
			mutate: func(p *rolo.Person) { p.Death = deathDate },
			want:   render.Person{Name: "Robert Langford", Birth: "Mar 12, 1965", Death: "Jan 8, 2022"},
		},
		{
			name:   "withheld phone where the tier prints phones",
			tier:   render.Digital,
			mutate: func(p *rolo.Person) { p.Hidden.Phone = true },
			want: render.Person{
				Name: "Robert Langford", Birth: "Mar 12",
				Phone: render.Private, Email: "robert@example.com",
			},
		},
		{
			name:   "withheld phone in full is still private",
			tier:   render.Full,
			mutate: func(p *rolo.Person) { p.Hidden.Phone = true },
			want: render.Person{
				Name: "Robert Langford", Birth: "Mar 12, 1965",
				Phone: render.Private, Email: "robert@example.com",
			},
		},
		{
			name:   "suppression beats withholding: hidden phone in mail prints nothing",
			tier:   render.Mail,
			mutate: func(p *rolo.Person) { p.Hidden.Phone = true },
			want:   render.Person{Name: "Robert Langford", Birth: "Mar 12"},
		},
		{
			name: "suppression beats withholding: a minor's hidden phone outside full",
			tier: render.Call,
			mutate: func(p *rolo.Person) {
				p.Birth = minorBirth
				p.Hidden.Phone = true
			},
			want: render.Person{Name: "Robert Langford", Birth: "Apr 30"},
		},
		{
			name:   "withheld email",
			tier:   render.Digital,
			mutate: func(p *rolo.Person) { p.Hidden.Email = true },
			want: render.Person{
				Name: "Robert Langford", Birth: "Mar 12",
				Phone: "555-201-0001", Email: render.Private,
			},
		},
		{
			name:   "withheld birth date of a living person",
			tier:   render.Mail,
			mutate: func(p *rolo.Person) { p.Hidden.Birth = true },
			want:   render.Person{Name: "Robert Langford", Birth: render.Private},
		},
		{
			name: "withheld birth date of a deceased person",
			tier: render.Mail,
			mutate: func(p *rolo.Person) {
				p.Death = deathDate
				p.Hidden.Birth = true
			},
			want: render.Person{Name: "Robert Langford", Birth: render.Private, Death: "Jan 8, 2022"},
		},
		{
			name: "withheld but never recorded prints nothing",
			tier: render.Full,
			mutate: func(p *rolo.Person) {
				p.Phone = ""
				p.Hidden.Phone = true
			},
			want: render.Person{Name: "Robert Langford", Birth: "Mar 12, 1965", Email: "robert@example.com"},
		},
		{
			name: "withheld year-only birth outside full prints nothing: truncation left nothing to withhold",
			tier: render.Mail,
			mutate: func(p *rolo.Person) {
				p.Birth = yearOnly
				p.Hidden.Birth = true
			},
			want: render.Person{Name: "Robert Langford"},
		},
		{
			name: "withheld year-only birth in full is private",
			tier: render.Full,
			mutate: func(p *rolo.Person) {
				p.Birth = yearOnly
				p.Hidden.Birth = true
			},
			want: render.Person{
				Name: "Robert Langford", Birth: render.Private,
				Phone: "555-201-0001", Email: "robert@example.com",
			},
		},
		{
			name: "the zero tier fails closed",
			tier: render.Tier(0),
			want: render.Person{Name: "Robert Langford", Birth: "Mar 12"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := subject()
			if tt.mutate != nil {
				tt.mutate(&p)
			}

			d := build(t, tt.tier, rolo.Household{
				ID:         "h_test01",
				Adults:     []rolo.Person{anchor()},
				Dependents: []rolo.Person{p},
			})

			require.Len(t, d.Households, 1)
			require.Len(t, d.Households[0].Dependents, 1)
			assert.Equal(t, tt.want, d.Households[0].Dependents[0])
		})
	}
}

func TestBuildHousehold(t *testing.T) {
	dead := func(p rolo.Person) rolo.Person {
		p.Death = deathDate
		return p
	}
	spouse := rolo.Person{ID: "p_spou01", Given: "Susan", Surname: "Langford", Birth: adultBirth}

	tests := []struct {
		name      string
		tier      render.Tier
		household rolo.Household
		checkFunc func(t *testing.T, h render.Household)
	}{
		{
			name: "anniversary truncated while both adults live",
			tier: render.Mail,
			household: rolo.Household{
				ID: "h_test01", Adults: []rolo.Person{subject(), spouse}, Anniversary: anniversary,
			},
			checkFunc: func(t *testing.T, h render.Household) {
				t.Helper()
				assert.Equal(t, "Jun 15", h.Anniversary)
			},
		},
		{
			name: "anniversary truncated while either adult lives",
			tier: render.Digital,
			household: rolo.Household{
				ID: "h_test01", Adults: []rolo.Person{dead(subject()), spouse}, Anniversary: anniversary,
			},
			checkFunc: func(t *testing.T, h render.Household) {
				t.Helper()
				assert.False(t, h.Memorial)
				assert.Equal(
					t, "Jun 15", h.Anniversary,
					"a surviving spouse's wedding date is a security question (§5.3)",
				)
			},
		},
		{
			name: "anniversary whole once both adults have died",
			tier: render.Mail,
			household: rolo.Household{
				ID: "h_test01", Adults: []rolo.Person{dead(subject()), dead(spouse)}, Anniversary: anniversary,
			},
			checkFunc: func(t *testing.T, h render.Household) {
				t.Helper()
				assert.True(t, h.Memorial)
				assert.Equal(t, "Jun 15, 1991", h.Anniversary)
			},
		},
		{
			name: "anniversary whole in full",
			tier: render.Full,
			household: rolo.Household{
				ID: "h_test01", Adults: []rolo.Person{subject(), spouse}, Anniversary: anniversary,
			},
			checkFunc: func(t *testing.T, h render.Household) {
				t.Helper()
				assert.Equal(t, "Jun 15, 1991", h.Anniversary)
			},
		},
		{
			name: "a memorial household publishes no contact details, even for a living dependent",
			tier: render.Full,
			household: rolo.Household{
				ID:         "h_test01",
				Adults:     []rolo.Person{dead(anchor())},
				Dependents: []rolo.Person{subject()},
			},
			checkFunc: func(t *testing.T, h render.Household) {
				t.Helper()
				require.True(t, h.Memorial)
				require.Len(t, h.Dependents, 1)
				assert.Empty(t, h.Dependents[0].Phone, "§5.4: names and dates only")
				assert.Empty(t, h.Dependents[0].Email)
				assert.Equal(t, "Mar 12, 1965", h.Dependents[0].Birth)
			},
		},
		{
			name: "a living dependent in a memorial household keeps a truncated birth date outside full",
			tier: render.Mail,
			household: rolo.Household{
				ID:         "h_test01",
				Adults:     []rolo.Person{dead(anchor())},
				Dependents: []rolo.Person{subject()},
			},
			checkFunc: func(t *testing.T, h render.Household) {
				t.Helper()
				require.True(t, h.Memorial)
				require.Len(t, h.Dependents, 1)
				assert.Equal(t, "Mar 12", h.Dependents[0].Birth, "§5.4 is not \"all dates Whole\"")
				assert.Empty(t, h.Dependents[0].Phone)
				assert.Empty(t, h.Dependents[0].Email)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := build(t, tt.tier, tt.household)
			require.Len(t, d.Households, 1)
			tt.checkFunc(t, d.Households[0])
		})
	}
}

func TestBuildStamp(t *testing.T) {
	tests := []struct {
		name           string
		tier           render.Tier
		wantTier       string
		wantRestricted bool
	}{
		{name: "mail", tier: render.Mail, wantTier: "Mail"},
		{name: "call", tier: render.Call, wantTier: "Call"},
		{name: "digital", tier: render.Digital, wantTier: "Digital"},
		{name: "full is restricted", tier: render.Full, wantTier: "Full", wantRestricted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := build(t, tt.tier, rolo.Household{ID: "h_test01", Adults: []rolo.Person{anchor()}})
			assert.Equal(t, "Sep 24, 2026", d.GeneratedAt)
			assert.Equal(t, tt.wantTier, d.Tier)
			assert.Equal(t, tt.wantRestricted, d.Restricted)
		})
	}
}

func TestBuildAddress(t *testing.T) {
	elm := []string{"42 Elm Street", "Springfield, IL 62701"}

	living := func(id rolo.PersonID, given string) rolo.Person {
		return rolo.Person{ID: id, Given: given, Surname: "Test", Birth: adultBirth}
	}
	dead := func(id rolo.PersonID, given string) rolo.Person {
		p := living(id, given)
		p.Death = deathDate
		return p
	}

	// parent is the Household whose address a child shares. Rows vary it.
	parent := func(addr rolo.Address, adult rolo.Person) rolo.Household {
		return rolo.Household{ID: "h_par001", Adults: []rolo.Person{adult}, Address: addr}
	}
	// child shares the parent's address unless a row overrides Address.
	child := func(addr rolo.Address) rolo.Household {
		return rolo.Household{
			ID: "h_kid001", Parent: "h_par001",
			Adults:  []rolo.Person{living("p_kid001", "Daniel")},
			Address: addr,
		}
	}
	// grandchild shares the child's address unless a row overrides Address.
	grandchild := func(addr rolo.Address) rolo.Household {
		return rolo.Household{
			ID: "h_gkid01", Parent: "h_kid001",
			Adults:  []rolo.Person{living("p_gkid01", "Emma")},
			Address: addr,
		}
	}
	sharesParent := rolo.Address{SharedWith: "h_par001"}
	sharesChild := rolo.Address{SharedWith: "h_kid001"}

	tests := []struct {
		name           string
		tier           render.Tier
		households     []rolo.Household
		index          int // which rendered Household to check
		wantLines      []string
		wantSharedWith string
	}{
		{
			name:       "own address keeps its lines",
			tier:       render.Full,
			households: []rolo.Household{parent(rolo.Address{Lines: elm}, living("p_par001", "Robert"))},
			index:      0,
			wantLines:  elm,
		},
		{
			name:       "withheld own address is private",
			tier:       render.Full,
			households: []rolo.Household{parent(rolo.Address{Lines: elm, Hidden: true}, living("p_par001", "Robert"))},
			index:      0,
			wantLines:  []string{render.Private},
		},
		{
			name:       "withheld but empty address prints nothing",
			tier:       render.Full,
			households: []rolo.Household{parent(rolo.Address{Hidden: true}, living("p_par001", "Robert"))},
			index:      0,
		},
		{
			name:       "a memorial household prints no address (§5.4)",
			tier:       render.Full,
			households: []rolo.Household{parent(rolo.Address{Lines: elm}, dead("p_par001", "Harold"))},
			index:      0,
		},
		{
			name: "a shared address is a back-reference to a live target's Household Name",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm}, living("p_par001", "Robert")),
				child(sharesParent),
			},
			index:          1,
			wantSharedWith: "Robert Test",
		},
		{
			name: "a shared address whose target is withheld is private, not a pointer to a marker",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm, Hidden: true}, living("p_par001", "Robert")),
				child(sharesParent),
			},
			index:     1,
			wantLines: []string{render.Private},
		},
		{
			name: "a sharer that withholds its own address is private",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm}, living("p_par001", "Robert")),
				child(rolo.Address{SharedWith: "h_par001", Hidden: true}),
			},
			index:     1,
			wantLines: []string{render.Private},
		},
		{
			name: "a shared address whose target is memorial is resolved into lines",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm}, dead("p_par001", "Harold")),
				child(sharesParent),
			},
			index:     1,
			wantLines: elm,
		},
		{
			name: "a shared address whose memorial target is withheld is private",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm, Hidden: true}, dead("p_par001", "Harold")),
				child(sharesParent),
			},
			index:     1,
			wantLines: []string{render.Private},
		},
		{
			name: "a shared address whose target has no address prints nothing",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{}, living("p_par001", "Robert")),
				child(sharesParent),
			},
			index: 1,
		},
		{
			name: "a withheld sharer whose target has no address prints nothing",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{}, living("p_par001", "Robert")),
				child(rolo.Address{SharedWith: "h_par001", Hidden: true}),
			},
			index: 1,
		},
		{
			name: "a memorial sharer prints nothing",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm}, living("p_par001", "Robert")),
				{
					ID: "h_kid001", Parent: "h_par001",
					Adults:  []rolo.Person{dead("p_kid001", "Daniel")},
					Address: sharesParent,
				},
			},
			index: 1,
		},
		{
			name: "a cycle of shared addresses prints nothing rather than looping",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{SharedWith: "h_kid001"}, living("p_par001", "Robert")),
				child(sharesParent),
			},
			index: 1,
		},
		{
			name: "a chain of shared addresses prints the lines rather than a pointer to a pointer",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm}, living("p_par001", "Robert")),
				child(sharesParent),
				grandchild(sharesChild),
			},
			index:     2,
			wantLines: elm,
		},
		{
			name: "a chain of shared addresses still back-references at the first hop",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm}, living("p_par001", "Robert")),
				child(sharesParent),
				grandchild(sharesChild),
			},
			index:          1,
			wantSharedWith: "Robert Test",
		},
		{
			name: "a chain ending at a withheld address is private",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm, Hidden: true}, living("p_par001", "Robert")),
				child(sharesParent),
				grandchild(sharesChild),
			},
			index:     2,
			wantLines: []string{render.Private},
		},
		{
			name: "a memorial target that itself shares resolves to the lines at the end",
			tier: render.Full,
			households: []rolo.Household{
				parent(rolo.Address{Lines: elm}, living("p_par001", "Robert")),
				{
					ID: "h_kid001", Parent: "h_par001",
					Adults:  []rolo.Person{dead("p_kid001", "Daniel")},
					Address: sharesParent,
				},
				grandchild(sharesChild),
			},
			index:     2,
			wantLines: elm,
		},
		{
			name: "a self-share prints nothing",
			tier: render.Full,
			households: []rolo.Household{
				{
					ID:      "h_par001",
					Adults:  []rolo.Person{living("p_par001", "Robert")},
					Address: rolo.Address{SharedWith: "h_par001"},
				},
			},
			index: 0,
		},
		{
			name:       "addresses print in every tier",
			tier:       render.Mail,
			households: []rolo.Household{parent(rolo.Address{Lines: elm}, living("p_par001", "Robert"))},
			index:      0,
			wantLines:  elm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := build(t, tt.tier, tt.households...)
			require.Len(t, d.Households, len(tt.households))

			h := d.Households[tt.index]
			assert.Equal(t, tt.wantLines, h.AddressLines, "address lines")
			assert.Equal(t, tt.wantSharedWith, h.SharedWith, "back-reference")
		})
	}
}

// TestBuildNames covers the Household Name and the row names beneath it
// (CONTEXT.md, Household Name). Every row is Full so no tier rule interferes.
func TestBuildNames(t *testing.T) {
	adult := func(id rolo.PersonID, given, surname, birthName string) rolo.Person {
		return rolo.Person{ID: id, Given: given, Surname: surname, BirthName: birthName, Birth: adultBirth}
	}
	daryl := adult("p_dary01", "Daryl", "Yoder", "")
	dawn := adult("p_dawn01", "Dawn", "Yoder", "Mitchell")

	tests := []struct {
		name           string
		household      rolo.Household
		wantName       string
		wantAdults     []string
		wantDependents []string
	}{
		{
			name: "a shared surname prints once, with a birth name in parentheses",
			household: rolo.Household{
				ID: "h_yode01", Adults: []rolo.Person{daryl, dawn},
			},
			wantName:   "Daryl & Dawn (Mitchell) Yoder",
			wantAdults: []string{"Daryl", "Dawn"},
		},
		{
			name: "a birth name equal to the surname is not shown",
			household: rolo.Household{
				ID: "h_yode01", Adults: []rolo.Person{daryl, adult("p_dawn01", "Dawn", "Yoder", "Yoder")},
			},
			wantName:   "Daryl & Dawn Yoder",
			wantAdults: []string{"Daryl", "Dawn"},
		},
		{
			name: "every adult born under another surname carries it",
			household: rolo.Household{
				ID: "h_yode01", Adults: []rolo.Person{adult("p_dary01", "Daryl", "Yoder", "Smith"), dawn},
			},
			wantName:   "Daryl (Smith) & Dawn (Mitchell) Yoder",
			wantAdults: []string{"Daryl", "Dawn"},
		},
		{
			name: "different surnames name each adult in full, without birth names",
			household: rolo.Household{
				ID: "h_mixd01",
				Adults: []rolo.Person{
					adult("p_chri01", "Chris", "Yoder", ""),
					adult("p_samp01", "Sam", "Patel", "Jones"),
				},
			},
			wantName:   "Chris Yoder & Sam Patel",
			wantAdults: []string{"Chris", "Sam"},
		},
		{
			name: "a single adult prints their own name without a birth name",
			household: rolo.Household{
				ID: "h_sing01", Adults: []rolo.Person{adult("p_sing01", "Dawn", "Yoder", "Mitchell")},
			},
			wantName:   "Dawn Yoder",
			wantAdults: []string{"Dawn"},
		},
		{
			name: "a nickname is on the row, never in the Household Name",
			household: rolo.Household{
				ID: "h_nova01",
				Adults: []rolo.Person{{
					ID: "p_patn01", Given: "Patricia", Surname: "Novak", Aka: "Pat", Birth: adultBirth,
				}},
			},
			wantName:   "Patricia Novak",
			wantAdults: []string{`Patricia "Pat"`},
		},
		{
			name: "a dependent drops a carried surname and keeps any other",
			household: rolo.Household{
				ID: "h_yode01", Adults: []rolo.Person{daryl, dawn},
				Dependents: []rolo.Person{
					{ID: "p_kyle01", Given: "Kyle", Surname: "Yoder", Birth: minorBirth},
					{ID: "p_jord01", Given: "Jordan", Surname: "Mitchell", Birth: minorBirth},
				},
			},
			wantName:       "Daryl & Dawn (Mitchell) Yoder",
			wantAdults:     []string{"Daryl", "Dawn"},
			wantDependents: []string{"Kyle", "Jordan Mitchell"},
		},
		{
			name: "in a two-surname household either surname is carried",
			household: rolo.Household{
				ID: "h_mixd01",
				Adults: []rolo.Person{
					adult("p_chri01", "Chris", "Yoder", ""),
					adult("p_samp01", "Sam", "Patel", ""),
				},
				Dependents: []rolo.Person{
					{ID: "p_rile01", Given: "Riley", Surname: "Patel", Birth: minorBirth},
					{ID: "p_rile02", Given: "Rowan", Surname: "Patel-Yoder", Birth: minorBirth},
				},
			},
			wantName:       "Chris Yoder & Sam Patel",
			wantAdults:     []string{"Chris", "Sam"},
			wantDependents: []string{"Riley", "Rowan Patel-Yoder"},
		},
		{
			name: "an adult with an empty Given in a shared-surname household is skipped, not a bare parenthetical",
			household: rolo.Household{
				ID: "h_yode01", Adults: []rolo.Person{daryl, adult("p_dawn01", "", "Yoder", "Mitchell")},
			},
			wantName:   "Daryl Yoder",
			wantAdults: []string{"Daryl", "Yoder"},
		},
		{
			name: "a dependent with an empty Given keeps a carried surname rather than printing blank",
			household: rolo.Household{
				ID: "h_yode01", Adults: []rolo.Person{daryl, dawn},
				Dependents: []rolo.Person{
					{ID: "p_kid001", Given: "", Surname: "Yoder", Birth: minorBirth},
				},
			},
			wantName:       "Daryl & Dawn (Mitchell) Yoder",
			wantAdults:     []string{"Daryl", "Dawn"},
			wantDependents: []string{"Yoder"},
		},
		{
			name: "a memorial household is named by the same rules",
			household: rolo.Household{
				ID: "h_meml01",
				Adults: []rolo.Person{
					{ID: "p_hara01", Given: "Harold", Surname: "Langford", Birth: adultBirth, Death: deathDate},
					{
						ID:        "p_june01",
						Given:     "June",
						Surname:   "Langford",
						BirthName: "Whitfield",
						Birth:     adultBirth,
						Death:     deathDate,
					},
				},
			},
			wantName:   "Harold & June (Whitfield) Langford",
			wantAdults: []string{"Harold", "June"},
		},
	}

	names := func(ps []render.Person) []string {
		var out []string
		for _, p := range ps {
			out = append(out, p.Name)
		}
		return out
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := build(t, render.Full, tt.household)

			require.Len(t, d.Households, 1)
			h := d.Households[0]
			assert.Equal(t, tt.wantName, h.Name, "Household Name")
			assert.Equal(t, tt.wantAdults, names(h.Adults), "adult rows")
			assert.Equal(t, tt.wantDependents, names(h.Dependents), "dependent rows")
		})
	}
}
