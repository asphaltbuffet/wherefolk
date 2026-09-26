package render_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/internal/render"
)

func TestTierString(t *testing.T) {
	tests := []struct {
		name string
		in   render.Tier
		want string
	}{
		{name: "mail", in: render.Mail, want: "Mail"},
		{name: "call", in: render.Call, want: "Call"},
		{name: "digital", in: render.Digital, want: "Digital"},
		{name: "full", in: render.Full, want: "Full"},
		{name: "the zero tier names nothing", in: render.Tier(0), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.String())
		})
	}
}

func TestParseTier(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   render.Tier
		wantOK bool
	}{
		{name: "mail", in: "mail", want: render.Mail, wantOK: true},
		{name: "call", in: "call", want: render.Call, wantOK: true},
		{name: "digital", in: "digital", want: render.Digital, wantOK: true},
		{name: "full", in: "full", want: render.Full, wantOK: true},
		{name: "empty is no tier", in: "", wantOK: false},
		{name: "names are lower case only", in: "Full", wantOK: false},
		{name: "anything else is no tier", in: "everything", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := render.ParseTier(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTierKey(t *testing.T) {
	tests := []struct {
		name string
		in   render.Tier
		want string
	}{
		{name: "mail", in: render.Mail, want: "mail"},
		{name: "call", in: render.Call, want: "call"},
		{name: "digital", in: render.Digital, want: "digital"},
		{name: "full", in: render.Full, want: "full"},
		{name: "the zero tier has no key", in: render.Tier(0), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.Key())

			if tt.want != "" {
				back, ok := render.ParseTier(tt.want)
				assert.True(t, ok, "every key parses back")
				assert.Equal(t, tt.in, back)
			}
		})
	}
}
