package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/internal/render"
)

func TestEscape(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text is unchanged",
			in:   "42 Elm Street",
			want: "42 Elm Street",
		},
		{
			name: "a hash would start a typst expression",
			in:   "#1 Elm Street",
			want: `\#1 Elm Street`,
		},
		{
			name: "a backslash is escaped first, so escaping is not applied twice",
			in:   `a\b`,
			want: `a\\b`,
		},
		{
			name: "an at sign would start a label reference",
			in:   "pat.novak@example.com",
			want: `pat.novak\@example.com`,
		},
		{
			name: "asterisks would turn on bold",
			in:   "*Pat*",
			want: `\*Pat\*`,
		},
		{
			name: "underscores would turn on emphasis",
			in:   "birth_name",
			want: `birth\_name`,
		},
		{
			name: "brackets would open a content block",
			in:   "P.O. Box 212 [rear]",
			want: `P.O. Box 212 \[rear\]`,
		},
		{
			name: "a dollar would open math mode",
			in:   "$5 Apartment",
			want: `\$5 Apartment`,
		},
		{
			name: "angle brackets would declare a label",
			in:   "<pat>",
			want: `\<pat\>`,
		},
		{
			name: "a quoted nickname survives, since DisplayName produces one",
			in:   `Patricia "Pat" Novak`,
			want: `Patricia "Pat" Novak`,
		},
		{
			name: "empty stays empty",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, render.Escape(tt.in))
		})
	}
}
