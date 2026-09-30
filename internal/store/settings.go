package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// CurrentSettingsSchema is the settings file version this binary writes and is
// the highest it can read. It is versioned apart from the document, for the
// reason ADR-0013 keeps the file apart at all: the two change separately.
const CurrentSettingsSchema = 1

// Settings holds the Editor's choices about the Directory as a whole rather
// than about any one Household. Today that is only the Directory Title
// (CONTEXT.md). It lives in its own file beside the document; see ADR-0013.
//
// Like Document and Trash, Settings is never mutated once built: a change is a
// new value, which is what lets Persist tell what changed by comparing
// pointers.
type Settings struct {
	Schema int `json:"schema"`

	// Title is the Directory Title exactly as the Editor set it. Empty means
	// it was never set; the printed default belongs to internal/render.
	Title string `json:"title"`
}

// NewSettings returns the settings of a Directory nobody has titled yet.
func NewSettings() *Settings { return &Settings{Schema: CurrentSettingsSchema} }

// LoadSettings reads the settings at path. A missing file is an untitled
// Directory: that is every installation's first state, and every installation
// that predates the file.
func LoadSettings(path string) (*Settings, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return NewSettings(), nil
	}

	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}

	err = checkSchema(b, path, CurrentSettingsSchema)
	if err != nil {
		return nil, err
	}

	s := NewSettings()

	err = json.Unmarshal(b, s)
	if err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	return s, nil
}

// SaveSettings writes the settings to path atomically, with the document's
// permissions.
func SaveSettings(path string, s *Settings) error {
	err := writeJSON(path, s)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}

	return nil
}

// WithTitle returns a copy of s carrying title, leaving s untouched.
func (s *Settings) WithTitle(title string) *Settings {
	next := *s
	next.Title = title

	return &next
}
