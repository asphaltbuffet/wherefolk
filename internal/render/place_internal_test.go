package render

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlaceFields(t *testing.T) {
	sheet := ProofSheet{
		Notes:      ProofField{ID: "notes", Value: "hi"},
		AllCorrect: ProofCheck{ID: "all-correct", Checked: true},
	}
	notes := fieldMark{ID: "notes", Kind: MultilineField, Page: 1, X: 1, Y: 2, W: 3, H: 4, PH: 792}
	allOK := fieldMark{ID: "all-correct", Kind: CheckField, Page: 2, X: 5, Y: 6, W: 9, H: 9, PH: 792}

	tests := []struct {
		name    string
		marks   []fieldMark
		want    []FormField
		wantErr string
	}{
		{
			name:  "every field placed once",
			marks: []fieldMark{notes, allOK},
			want: []FormField{
				{ID: "notes", Kind: MultilineField, Page: 1, X: 1, Y: 2, W: 3, H: 4, PageHeight: 792, Value: "hi"},
				{ID: "all-correct", Kind: CheckField, Page: 2, X: 5, Y: 6, W: 9, H: 9, PageHeight: 792, Checked: true},
			},
		},
		{name: "template forgot a field", marks: []fieldMark{notes}, wantErr: `did not place field "all-correct"`},
		{
			name:    "template invented a field",
			marks:   []fieldMark{notes, allOK, {ID: "extra", Kind: TextField}},
			wantErr: `"extra"`,
		},
		{name: "placed twice", marks: []fieldMark{notes, notes, allOK}, wantErr: "placed twice"},
		{
			name:    "check marked as text",
			marks:   []fieldMark{notes, {ID: "all-correct", Kind: TextField}},
			wantErr: `"all-correct"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := placeFields(tt.marks, sheet)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
