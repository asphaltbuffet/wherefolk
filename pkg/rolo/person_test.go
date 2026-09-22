package rolo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestPersonIsDeceased(t *testing.T) {
	tests := []struct {
		name   string
		person rolo.Person
		want   bool
	}{
		{
			name:   "living person has no death date",
			person: rolo.Person{Given: "Doris", Birth: rolo.Date{Year: 1940}},
			want:   false,
		},
		{
			name:   "deceased person has a death date",
			person: rolo.Person{Given: "Clyde", Birth: rolo.Date{Year: 1938}, Death: rolo.Date{Year: 2019}},
			want:   true,
		},
		{
			name:   "year-only death date still counts",
			person: rolo.Person{Given: "Thomas", Death: rolo.Date{Year: 2022}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.person.IsDeceased())
		})
	}
}

func TestPersonIsMinor(t *testing.T) {
	asOf := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		person rolo.Person
		want   bool
	}{
		{
			name:   "no birth date fails closed and is treated as a minor",
			person: rolo.Person{Given: "Unknown"},
			want:   true,
		},
		{
			name:   "child born 2015 is a minor",
			person: rolo.Person{Given: "Mia", Birth: rolo.Date{Year: 2015, Month: 4, Day: 30}},
			want:   true,
		},
		{
			name:   "adult born 1971 is not a minor",
			person: rolo.Person{Given: "Dave", Birth: rolo.Date{Year: 1971, Month: 3, Day: 2}},
			want:   false,
		},
		{
			name:   "turns 18 tomorrow is still a minor",
			person: rolo.Person{Given: "Ellie", Birth: rolo.Date{Year: 2008, Month: 9, Day: 23}},
			want:   true,
		},
		{
			name:   "turned 18 today is not a minor",
			person: rolo.Person{Given: "Ellie", Birth: rolo.Date{Year: 2008, Month: 9, Day: 22}},
			want:   false,
		},
		{
			name:   "year-only birth date uses January 1",
			person: rolo.Person{Given: "Harold", Birth: rolo.Date{Year: 2008}},
			want:   false,
		},
		{
			name:   "deceased minor is still a minor",
			person: rolo.Person{Given: "Infant", Birth: rolo.Date{Year: 2020}, Death: rolo.Date{Year: 2021}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.person.IsMinor(asOf))
		})
	}
}

func TestPersonDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		person rolo.Person
		want   string
	}{
		{
			name:   "given and surname",
			person: rolo.Person{Given: "Dave", Surname: "Whitlock"},
			want:   "Dave Whitlock",
		},
		{
			name:   "aka renders in double quotes between given and surname",
			person: rolo.Person{Given: "Patricia", Aka: "Pat", Surname: "Novak"},
			want:   `Patricia "Pat" Novak`,
		},
		{
			name:   "given name only",
			person: rolo.Person{Given: "Mia"},
			want:   "Mia",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.person.DisplayName())
		})
	}
}
