package render_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

var proofAsOf = time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)

func proofTree(t *testing.T, hs ...rolo.Household) *rolo.Tree {
	t.Helper()

	tree, err := rolo.BuildTree(hs)
	require.NoError(t, err)

	return tree
}

func yoders() rolo.Household {
	return rolo.Household{
		ID: "h_yod",
		Adults: []rolo.Person{
			{
				ID: "p_dar", Given: "Daryl", Surname: "Yoder", Birth: rolo.Date{Year: 1965, Month: 3, Day: 12},
				Phone: "(555) 201-4433", Hidden: rolo.HiddenFields{Phone: true},
			},
			{
				ID: "p_daw", Given: "Dawn", Surname: "Yoder", BirthName: "Mitchell", Email: "dawn@example.com",
				Hidden: rolo.HiddenFields{Birth: true},
			},
		},
		Dependents: []rolo.Person{
			{
				ID: "p_tom", Given: "Thomas", Surname: "Yoder", Birth: rolo.Date{Year: 1990},
				Death: rolo.Date{Year: 2001, Month: 6, Day: 3}, Phone: "(555) 000-0000",
			},
		},
		Anniversary: rolo.Date{Year: 1988, Month: 6},
		Address:     rolo.Address{Lines: []string{"12 Elm St", "Springfield, IL 62704"}},
	}
}

func person(t *testing.T, ps []render.ProofPerson, idPrefix string) render.ProofPerson {
	t.Helper()

	for _, p := range ps {
		if p.Given.ID == idPrefix+"-given" {
			return p
		}
	}

	t.Fatalf("no person %s", idPrefix)

	return render.ProofPerson{}
}

func TestBuildProof(t *testing.T) {
	memorial := rolo.Household{ID: "h_old", Adults: []rolo.Person{
		{ID: "p_old", Given: "Aden", Surname: "Yoder", Death: rolo.Date{Year: 1990}},
	}}
	parent := rolo.Household{
		ID: "h_par", Address: rolo.Address{Lines: []string{"1 Oak Rd"}, Hidden: true},
		Adults: []rolo.Person{{ID: "p_par", Given: "Clyde", Surname: "Yoder"}},
	}
	sharer := rolo.Household{
		ID: "h_kid", Parent: "h_par", Address: rolo.Address{SharedWith: "h_par"},
		Adults: []rolo.Person{{ID: "p_kid", Given: "Kyle", Surname: "Yoder"}},
	}
	loopA := rolo.Household{
		ID: "h_la", Address: rolo.Address{SharedWith: "h_lb"},
		Adults: []rolo.Person{{ID: "p_la", Given: "Ann", Surname: "Loop"}},
	}
	loopB := rolo.Household{
		ID: "h_lb", Address: rolo.Address{SharedWith: "h_la"},
		Adults: []rolo.Person{{ID: "p_lb", Given: "Bob", Surname: "Loop"}},
	}
	five := yoders()
	five.Address.Lines = []string{"a", "b", "c", "d", "e"}

	tests := []struct {
		name      string
		hs        []rolo.Household
		id        rolo.HouseholdID
		title     string
		wantErr   error
		checkFunc func(t *testing.T, s render.ProofSheet)
	}{
		{
			name: "unknown household", hs: []rolo.Household{yoders()}, id: "h_nope",
			wantErr: render.ErrNoSuchHousehold,
		},
		{
			name: "memorial has nobody to send to", hs: []rolo.Household{memorial}, id: "h_old",
			wantErr: render.ErrNoLivingMember,
		},
		{
			name: "heading, title default and reply", hs: []rolo.Household{yoders()}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				require.Equal(t, render.DefaultTitle, s.Title)
				require.Equal(t, "Daryl & Dawn (Mitchell) Yoder", s.Name)
				require.Equal(t, "Send to Dawn", s.Reply)
				require.Equal(t, "h_yod", s.HouseholdID)
			},
		},
		{
			name: "title as set", hs: []rolo.Household{yoders()}, id: "h_yod", title: "The Yoders",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				require.Equal(t, "The Yoders", s.Title)
			},
		},
		{
			name: "withheld phone shows its value, marked, with the box ticked",
			hs:   []rolo.Household{yoders()}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				p := person(t, s.Adults, "p_dar")
				require.Equal(t,
					render.ProofField{ID: "p_dar-phone", Value: "(555) 201-4433", Withheld: true}, p.Phone)
				require.Equal(t, render.ProofCheck{ID: "p_dar-keep-phone", Checked: true}, p.KeepPhone)
				require.Equal(t, render.ProofField{ID: "p_dar-birth", Value: "Mar 12, 1965"}, p.Birth)
				require.Equal(t, "p_dar-remove", p.Remove.ID)
			},
		},
		{
			name: "withheld but empty birth date ticks the box without a marker",
			hs:   []rolo.Household{yoders()}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				p := person(t, s.Adults, "p_daw")
				require.False(t, p.Birth.Withheld)
				require.True(t, p.KeepBirth.Checked)
				require.Equal(t, "Mitchell", p.BirthName.Value)
			},
		},
		{
			name: "deceased dependent has names and dates only", hs: []rolo.Household{yoders()}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				p := person(t, s.Dependents, "p_tom")
				require.True(t, p.Deceased)
				require.Equal(t, render.ProofField{ID: "p_tom-death", Value: "Jun 3, 2001"}, p.Death)
				require.Equal(t, "1990", p.Birth.Value)
				require.Empty(t, p.Phone.ID)
				require.Empty(t, p.Email.ID)
				require.Empty(t, p.KeepBirth.ID)
				require.Empty(t, p.Remove.ID)
			},
		},
		{
			name: "address padded to four lines, anniversary Whole", hs: []rolo.Household{yoders()}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				require.Equal(t, []render.ProofField{
					{ID: "address-1", Value: "12 Elm St"}, {ID: "address-2", Value: "Springfield, IL 62704"},
					{ID: "address-3"}, {ID: "address-4"},
				}, s.Address)
				require.Equal(t, render.ProofField{ID: "anniversary", Value: "Jun 1988"}, s.Anniversary)
				require.Empty(t, s.Moved.ID)
				require.Empty(t, s.SharedWith)
				require.False(t, s.KeepAddress.Checked)
			},
		},
		{
			name: "five address lines get five fields", hs: []rolo.Household{five}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				require.Len(t, s.Address, 5)
			},
		},
		{
			name: "shared address shows withheld lines (ADR-0014)",
			hs:   []rolo.Household{parent, sharer}, id: "h_kid",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				require.Equal(t, "1 Oak Rd", s.Address[0].Value)
				require.True(t, s.AddressWithheld)
				require.True(t, s.KeepAddress.Checked)
				require.Equal(t, "Clyde Yoder", s.SharedWith)
				require.Equal(t, "address-moved", s.Moved.ID)
				require.Empty(t, s.Anniversary.ID, "single adult, no anniversary")
			},
		},
		{
			name: "shared address cycle ends without lines", hs: []rolo.Household{loopA, loopB}, id: "h_la",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				require.Len(t, s.Address, 4)
				require.Empty(t, s.Address[0].Value)
				require.Equal(t, "Bob Loop", s.SharedWith)
			},
		},
		{
			name: "two blank people, no remove box", hs: []rolo.Household{yoders()}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				require.Len(t, s.Missing, 2)
				m := s.Missing[1]
				require.Equal(t, render.ProofField{ID: "missing2-given"}, m.Given)
				require.Equal(t, "missing2-relationship", m.Relationship.ID)
				require.Equal(t, "missing2-keep-phone", m.KeepPhone.ID)
				require.Empty(t, m.Remove.ID)
			},
		},
		{
			name: "field ids are unique", hs: []rolo.Household{yoders()}, id: "h_yod",
			checkFunc: func(t *testing.T, s render.ProofSheet) {
				t.Helper()

				texts, checks := s.FormValues()
				for id := range checks {
					require.NotContains(t, texts, id)
				}

				require.Equal(t, "(555) 201-4433", texts["p_dar-phone"])
				require.True(t, checks["p_dar-keep-phone"])
				require.Contains(t, texts, "notes")
				require.Contains(t, checks, "all-correct")
				require.NotContains(t, texts, "")
				require.NotContains(t, checks, "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := render.BuildProof(proofTree(t, tt.hs...), tt.id, proofAsOf, tt.title, "Send to Dawn")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "Nov 15, 2026", s.GeneratedAt)
			tt.checkFunc(t, s)
		})
	}
}
