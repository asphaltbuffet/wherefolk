package rolo_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    rolo.Date
		wantErr bool
	}{
		{
			name:  "full date",
			input: "1938-03-12",
			want:  rolo.Date{Year: 1938, Month: 3, Day: 12},
		},
		{
			name:  "year and month",
			input: "1938-03",
			want:  rolo.Date{Year: 1938, Month: 3},
		},
		{
			name:  "year only",
			input: "1938",
			want:  rolo.Date{Year: 1938},
		},
		{
			name:  "empty string is the zero date",
			input: "",
			want:  rolo.Date{},
		},
		{
			name:    "not a date",
			input:   "sometime in the fifties",
			wantErr: true,
		},
		{
			name:    "month out of range",
			input:   "1938-13-01",
			wantErr: true,
		},
		{
			name:    "day out of range for month",
			input:   "1938-02-30",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rolo.ParseDate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDateString(t *testing.T) {
	tests := []struct {
		name string
		date rolo.Date
		want string
	}{
		{name: "full date", date: rolo.Date{Year: 1938, Month: 3, Day: 12}, want: "1938-03-12"},
		{name: "year and month", date: rolo.Date{Year: 1938, Month: 3}, want: "1938-03"},
		{name: "year only", date: rolo.Date{Year: 1938}, want: "1938"},
		{name: "zero date is empty", date: rolo.Date{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.date.String())
		})
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		date     rolo.Date
		wantJSON string
	}{
		{name: "full date", date: rolo.Date{Year: 1938, Month: 3, Day: 12}, wantJSON: `"1938-03-12"`},
		{name: "year only", date: rolo.Date{Year: 1938}, wantJSON: `"1938"`},
		{name: "zero date", date: rolo.Date{}, wantJSON: `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.date)
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(b))

			var got rolo.Date
			require.NoError(t, json.Unmarshal(b, &got))
			assert.Equal(t, tt.date, got)
		})
	}
}

func TestDatePredicates(t *testing.T) {
	tests := []struct {
		name        string
		date        rolo.Date
		wantZero    bool
		wantHasYear bool
	}{
		{name: "zero date", date: rolo.Date{}, wantZero: true, wantHasYear: false},
		{name: "year only", date: rolo.Date{Year: 1938}, wantZero: false, wantHasYear: true},
		{name: "full date", date: rolo.Date{Year: 1938, Month: 3, Day: 12}, wantZero: false, wantHasYear: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantZero, tt.date.IsZero())
			assert.Equal(t, tt.wantHasYear, tt.date.HasYear())
		})
	}
}
