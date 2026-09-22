package store_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
)

func TestNewIDs(t *testing.T) {
	tests := []struct {
		name       string
		generate   func() (string, error)
		wantPrefix string
	}{
		{
			name: "person IDs carry the p_ prefix",
			generate: func() (string, error) {
				id, err := store.NewPersonID()
				return string(id), err
			},
			wantPrefix: "p_",
		},
		{
			name: "household IDs carry the h_ prefix",
			generate: func() (string, error) {
				id, err := store.NewHouseholdID()
				return string(id), err
			},
			wantPrefix: "h_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.generate()
			require.NoError(t, err)

			assert.True(t, strings.HasPrefix(id, tt.wantPrefix), "id %q lacks prefix %q", id, tt.wantPrefix)
			assert.Len(t, id, len(tt.wantPrefix)+store.IDSize)

			body := strings.TrimPrefix(id, tt.wantPrefix)
			for _, r := range body {
				assert.True(t, strings.ContainsRune(store.IDAlphabet, r),
					"id %q contains %q, which is outside the alphabet", id, r)
			}
		})
	}
}

func TestNewIDsAreUnique(t *testing.T) {
	const n = 1000

	seen := make(map[string]struct{}, n)
	for range n {
		id, err := store.NewHouseholdID()
		require.NoError(t, err)

		_, dup := seen[string(id)]
		require.False(t, dup, "generated duplicate id %q", id)
		seen[string(id)] = struct{}{}
	}
}

func TestIDAlphabetExcludesConfusableCharacters(t *testing.T) {
	tests := []struct {
		name string
		char rune
	}{
		{name: "i is confusable with 1", char: 'i'},
		{name: "l is confusable with 1", char: 'l'},
		{name: "o is confusable with 0", char: 'o'},
		{name: "u is excluded by Crockford base32", char: 'u'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotContains(t, store.IDAlphabet, string(tt.char))
		})
	}
}
