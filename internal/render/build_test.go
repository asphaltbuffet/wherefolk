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
				labels := make([]string, 0, len(d.Households))
				for _, h := range d.Households {
					labels = append(labels, h.Label)
				}
				assert.Equal(t, []string{"Harold/June", "Robert/Susan", "Daniel/Claire", "Patricia"}, labels)
			},
		},
		{
			name: "the generation date, tier and restriction are stamped",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				assert.Equal(t, "September 24, 2026", d.GeneratedAt, "exports must say when they were made (§5.7)")
				assert.Equal(t, "Full", d.Tier)
				assert.True(t, d.Restricted, "Full carries DO NOT DISTRIBUTE (§5.2)")
			},
		},
		{
			name: "a memorial household keeps its names and whole dates",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				h := d.Households[0]
				assert.True(t, h.Memorial)
				assert.Equal(t, "May 23, 1953", h.Anniversary)
				require.Len(t, h.Adults, 2)
				assert.Equal(t, "Harold Langford", h.Adults[0].Name)
				assert.Equal(t, "February 14, 1928", h.Adults[0].Birth)
				assert.Equal(t, "September 30, 2011", h.Adults[0].Death)
			},
		},
		{
			name: "a nickname renders in the display name",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				require.Len(t, d.Households[3].Adults, 1)
				assert.Equal(t, `Patricia "Pat" Novak`, d.Households[3].Adults[0].Name)
			},
		},
		{
			name: "full prints living people's dates whole and their contact details",
			checkFunc: func(t *testing.T, d render.Directory) {
				t.Helper()
				robert := d.Households[1].Adults[0]
				assert.Equal(t, "March 12, 1965", robert.Birth)
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
			want: render.Person{Name: "Robert Langford", Birth: "March 12"},
		},
		{
			name: "call: adds phone",
			tier: render.Call,
			want: render.Person{Name: "Robert Langford", Birth: "March 12", Phone: "555-201-0001"},
		},
		{
			name: "digital: adds email",
			tier: render.Digital,
			want: render.Person{
				Name: "Robert Langford", Birth: "March 12",
				Phone: "555-201-0001", Email: "robert@example.com",
			},
		},
		{
			name: "full: whole birth date",
			tier: render.Full,
			want: render.Person{
				Name: "Robert Langford", Birth: "March 12, 1965",
				Phone: "555-201-0001", Email: "robert@example.com",
			},
		},
		{
			name:   "minor outside full: name and truncated birthday, no contact details",
			tier:   render.Digital,
			mutate: func(p *rolo.Person) { p.Birth = minorBirth },
			want:   render.Person{Name: "Robert Langford", Birth: "April 30"},
		},
		{
			name:   "minor in full: everything",
			tier:   render.Full,
			mutate: func(p *rolo.Person) { p.Birth = minorBirth },
			want: render.Person{
				Name: "Robert Langford", Birth: "April 30, 2021",
				Phone: "555-201-0001", Email: "robert@example.com",
			},
		},
		{
			name:   "an eighteenth birthday on the export date admits contact details",
			tier:   render.Call,
			mutate: func(p *rolo.Person) { p.Birth = turns18 },
			want:   render.Person{Name: "Robert Langford", Birth: "September 24", Phone: "555-201-0001"},
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
			want:   render.Person{Name: "Robert Langford", Birth: "March 12, 1965", Death: "January 8, 2022"},
		},
		{
			name:   "deceased: contact details suppressed even in full",
			tier:   render.Full,
			mutate: func(p *rolo.Person) { p.Death = deathDate },
			want:   render.Person{Name: "Robert Langford", Birth: "March 12, 1965", Death: "January 8, 2022"},
		},
		{
			name:   "withheld phone where the tier prints phones",
			tier:   render.Digital,
			mutate: func(p *rolo.Person) { p.Hidden.Phone = true },
			want: render.Person{
				Name: "Robert Langford", Birth: "March 12",
				Phone: render.Private, Email: "robert@example.com",
			},
		},
		{
			name:   "withheld phone in full is still private",
			tier:   render.Full,
			mutate: func(p *rolo.Person) { p.Hidden.Phone = true },
			want: render.Person{
				Name: "Robert Langford", Birth: "March 12, 1965",
				Phone: render.Private, Email: "robert@example.com",
			},
		},
		{
			name:   "suppression beats withholding: hidden phone in mail prints nothing",
			tier:   render.Mail,
			mutate: func(p *rolo.Person) { p.Hidden.Phone = true },
			want:   render.Person{Name: "Robert Langford", Birth: "March 12"},
		},
		{
			name: "suppression beats withholding: a minor's hidden phone outside full",
			tier: render.Call,
			mutate: func(p *rolo.Person) {
				p.Birth = minorBirth
				p.Hidden.Phone = true
			},
			want: render.Person{Name: "Robert Langford", Birth: "April 30"},
		},
		{
			name:   "withheld email",
			tier:   render.Digital,
			mutate: func(p *rolo.Person) { p.Hidden.Email = true },
			want: render.Person{
				Name: "Robert Langford", Birth: "March 12",
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
			want: render.Person{Name: "Robert Langford", Birth: render.Private, Death: "January 8, 2022"},
		},
		{
			name: "withheld but never recorded prints nothing",
			tier: render.Full,
			mutate: func(p *rolo.Person) {
				p.Phone = ""
				p.Hidden.Phone = true
			},
			want: render.Person{Name: "Robert Langford", Birth: "March 12, 1965", Email: "robert@example.com"},
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
			want: render.Person{Name: "Robert Langford", Birth: "March 12"},
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
				assert.Equal(t, "June 15", h.Anniversary)
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
					t, "June 15", h.Anniversary,
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
				assert.Equal(t, "June 15, 1991", h.Anniversary)
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
				assert.Equal(t, "June 15, 1991", h.Anniversary)
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
				assert.Equal(t, "March 12, 1965", h.Dependents[0].Birth)
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
			assert.Equal(t, "September 24, 2026", d.GeneratedAt)
			assert.Equal(t, tt.wantTier, d.Tier)
			assert.Equal(t, tt.wantRestricted, d.Restricted)
		})
	}
}
