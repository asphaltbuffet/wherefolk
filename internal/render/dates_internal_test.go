package render

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestExportDates(t *testing.T) {
	tests := []struct {
		name      string
		in        rolo.Date
		wantWhole string
		wantTrunc string
	}{
		{
			name:      "unknown date prints nothing either way",
			in:        rolo.Date{},
			wantWhole: "",
			wantTrunc: "",
		},
		{
			name:      "year only truncates to nothing, because the year is all it holds",
			in:        rolo.Date{Year: 1938},
			wantWhole: "1938",
			wantTrunc: "",
		},
		{
			name:      "year and month",
			in:        rolo.Date{Year: 1938, Month: 3},
			wantWhole: "Mar 1938",
			wantTrunc: "Mar",
		},
		{
			name:      "full date",
			in:        rolo.Date{Year: 1965, Month: 3, Day: 12},
			wantWhole: "Mar 12, 1965",
			wantTrunc: "Mar 12",
		},
		{
			name:      "single-digit day has no leading zero",
			in:        rolo.Date{Year: 2022, Month: 1, Day: 8},
			wantWhole: "Jan 8, 2022",
			wantTrunc: "Jan 8",
		},
		{
			name:      "every month is three letters, September included",
			in:        rolo.Date{Year: 2011, Month: 9, Day: 30},
			wantWhole: "Sep 30, 2011",
			wantTrunc: "Sep 30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantWhole, wholeDate(tt.in), "Whole Date")
			assert.Equal(t, tt.wantTrunc, truncatedDate(tt.in), "Truncated Date")
		})
	}
}
