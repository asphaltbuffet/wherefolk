package cmd_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/cmd"
)

func TestGetPrintCmd(t *testing.T) {
	t.Run("new command", func(t *testing.T) {
		assert.NotNil(t, cmd.GetPrintCmd())
	})

	t.Run("existing command", func(t *testing.T) {
		c := cmd.GetPrintCmd()
		assert.Equal(t, c, cmd.GetPrintCmd())
	})
}

func TestRunPrintCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains []string
		wantErr  bool
	}{
		{
			name:    "missing argument returns error",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "missing file returns error",
			args:    []string{"../testdata/nonexistent.json"},
			wantErr: true,
		},
		{
			name: "prints all top-level families",
			args: []string{"../testdata/example.json"},
			contains: []string{
				"Langford",
				"Novak",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cmd.GetPrintCmd()
			buf := &bytes.Buffer{}
			c.SetOut(buf)
			c.SetErr(buf)
			c.SetArgs(tt.args)

			err := c.Execute()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			out := buf.String()
			for _, want := range tt.contains {
				assert.Contains(t, out, want)
			}
		})
	}
}
