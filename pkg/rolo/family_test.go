package rolo_test

import (
	_ "embed"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

//go:embed testdata/example.golden
var goldenTable string

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(t *testing.T, s string) string {
	t.Helper()
	return ansiEscape.ReplaceAllString(s, "")
}

func TestLoadJSON(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantLen   int
		wantErr   bool
		checkFunc func(t *testing.T, fams []rolo.Family)
	}{
		{
			name:    "loads all top-level families",
			file:    "../../testdata/example.json",
			wantLen: 2,
		},
		{
			name:    "missing file returns error",
			file:    "../../testdata/nonexistent.json",
			wantErr: true,
		},
		{
			name:    "second family is Patricia Novak",
			file:    "../../testdata/example.json",
			wantLen: 2,
			checkFunc: func(t *testing.T, fams []rolo.Family) {
				t.Helper()
				require.Len(t, fams[1].People, 1)
				assert.Equal(t, "Patricia", fams[1].People[0].Name)
				assert.Equal(t, "Novak", fams[1].People[0].Surname)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fams, err := rolo.LoadJSON(tt.file)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantLen > 0 {
				assert.Len(t, fams, tt.wantLen)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, fams)
			}
		})
	}
}

func TestTable(t *testing.T) {
	fams, err := rolo.LoadJSON("../../testdata/example.json")
	require.NoError(t, err)
	require.Len(t, fams, 2)

	var combined strings.Builder
	for _, f := range fams {
		combined.WriteString(f.Table(0))
		combined.WriteString("\n")
	}

	got := StripANSI(t, combined.String())
	assert.Equal(t, goldenTable, got)
}

func TestIsFamily(t *testing.T) {
	tests := []struct {
		name string
		fam  rolo.Family
		want bool
	}{
		{
			name: "single person no address no children",
			fam: rolo.Family{
				People: []rolo.Person{{Name: "Mia", Surname: "Langford"}},
			},
			want: false,
		},
		{
			name: "single person with address",
			fam: rolo.Family{
				People:    []rolo.Person{{Name: "Patricia", Surname: "Novak"}},
				Addresses: []string{"88 Oakwood Drive"},
			},
			want: true,
		},
		{
			name: "two people no address",
			fam: rolo.Family{
				People: []rolo.Person{
					{Name: "Robert", Surname: "Langford"},
					{Name: "Susan", Surname: "Langford"},
				},
			},
			want: true,
		},
		{
			name: "single person with children",
			fam: rolo.Family{
				People:   []rolo.Person{{Name: "Alice", Surname: "Smith"}},
				Children: []rolo.Family{{People: []rolo.Person{{Name: "Bob", Surname: "Smith"}}}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.fam.IsFamily())
		})
	}
}
