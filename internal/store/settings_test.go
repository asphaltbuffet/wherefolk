package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
)

func TestLoadSettings(t *testing.T) {
	tests := []struct {
		name      string
		contents  *string // nil means the file does not exist
		wantErr   error   // checked with ErrorIs when set
		wantAnErr bool    // any error
		want      *store.Settings
	}{
		{
			name: "a missing file is an untitled Directory",
			want: &store.Settings{Schema: store.CurrentSettingsSchema},
		},
		{
			name:     "reads the title",
			contents: new(`{"schema": 1, "title": "The Langford Family Directory"}`),
			want:     &store.Settings{Schema: 1, Title: "The Langford Family Directory"},
		},
		{
			name:     "a newer schema is refused",
			contents: new(`{"schema": 2, "title": "x"}`),
			wantErr:  store.ErrSchemaTooNew,
		},
		{
			name:     "a missing schema is refused",
			contents: new(`{"title": "x"}`),
			wantErr:  store.ErrSchemaMissing,
		},
		{
			name:      "malformed json is an error",
			contents:  new(`{"schema": 1, "title": `),
			wantAnErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if tt.contents != nil {
				require.NoError(t, os.WriteFile(path, []byte(*tt.contents), 0o600))
			}

			got, err := store.LoadSettings(path)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantAnErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestSaveSettings(t *testing.T) {
	tests := []struct {
		name string
		in   *store.Settings
	}{
		{name: "an untitled Directory round-trips", in: store.NewSettings()},
		{name: "a title round-trips", in: store.NewSettings().WithTitle(`The "Langford" Family & Friends`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")

			require.NoError(t, store.SaveSettings(path, tt.in))

			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "same permissions as the document")

			got, err := store.LoadSettings(path)
			require.NoError(t, err)
			assert.Equal(t, tt.in, got)
		})
	}
}

func TestSettingsWithTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{name: "sets a title", title: "The Langford Family Directory"},
		{name: "clears a title", title: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := &store.Settings{Schema: store.CurrentSettingsSchema, Title: "Before"}

			got := orig.WithTitle(tt.title)

			assert.Equal(t, tt.title, got.Title)
			assert.Equal(t, "Before", orig.Title, "the original is never mutated")
			assert.NotSame(t, orig, got, "Persist compares pointers, so a change must be a new value")
		})
	}
}
