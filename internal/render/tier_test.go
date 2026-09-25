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
