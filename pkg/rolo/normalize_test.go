package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		want           string
		wantRecognized bool
	}{
		{
			name:           "already in house style",
			input:          "555-201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "parenthesised area code",
			input:          "(555) 201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "dot separated",
			input:          "555.201.0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "space separated",
			input:          "555 201 0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "bare digits",
			input:          "5552010001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "leading country code without plus",
			input:          "15552010001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "explicit +1 country code",
			input:          "+1 555 201 0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "explicit +1 with punctuation",
			input:          "+1 (555) 201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "surrounding whitespace is trimmed",
			input:          "  555-201-0001  ",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "empty is not a failure",
			input:          "",
			want:           "",
			wantRecognized: true,
		},
		{
			name:           "whitespace only becomes empty",
			input:          "   ",
			want:           "",
			wantRecognized: true,
		},
		{
			name:           "international number is kept verbatim",
			input:          "+44 20 7946 0958",
			want:           "+44 20 7946 0958",
			wantRecognized: false,
		},
		{
			name:           "another international number is kept verbatim",
			input:          "+61 2 9374 4000",
			want:           "+61 2 9374 4000",
			wantRecognized: false,
		},
		{
			name:           "extension is kept verbatim",
			input:          "555-201-0001 x12",
			want:           "555-201-0001 x12",
			wantRecognized: false,
		},
		{
			name:           "two numbers in one field are kept verbatim",
			input:          "555-201-0001, 555-201-0002",
			want:           "555-201-0001, 555-201-0002",
			wantRecognized: false,
		},
		{
			name:           "too few digits is kept verbatim",
			input:          "555-0001",
			want:           "555-0001",
			wantRecognized: false,
		},
		{
			name:           "vanity number is kept verbatim",
			input:          "1-800-FLOWERS",
			want:           "1-800-FLOWERS",
			wantRecognized: false,
		},
		{
			name:           "a note rather than a number is kept verbatim",
			input:          "call the house",
			want:           "call the house",
			wantRecognized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, recognized := rolo.NormalizePhone(tt.input)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantRecognized, recognized)
		})
	}
}
