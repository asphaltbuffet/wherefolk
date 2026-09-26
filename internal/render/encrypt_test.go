package render_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/render"
)

// passphrase is limited to space-free ASCII because our test oracle, pdfcpu's
// api.Decrypt, wrongly applies PRECIS instead of SASLprep and rejects spaces.
// Real passphrases may contain spaces and accents (verified independently with mupdf).
const passphrase = "correcthorsebattery"

// decrypt tries to open pdf with pw and reports pdfcpu's verdict.
func decrypt(pdf []byte, pw string) error {
	var out bytes.Buffer

	return api.Decrypt(bytes.NewReader(pdf), &out, model.NewAESConfiguration(pw, "", 256))
}

func TestEncrypt(t *testing.T) {
	fixture, err := os.ReadFile("testdata/minimal.pdf")
	require.NoError(t, err)

	tests := []struct {
		name       string
		pdf        []byte
		passphrase string
		wantErr    bool
		checkFunc  func(t *testing.T, out []byte)
	}{
		{
			name:       "the output is an encrypted pdf",
			pdf:        fixture,
			passphrase: passphrase,
			checkFunc: func(t *testing.T, out []byte) {
				t.Helper()
				assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
				assert.Contains(t, string(out), "/Encrypt")
			},
		},
		{
			name:       "the passphrase opens it",
			pdf:        fixture,
			passphrase: passphrase,
			checkFunc: func(t *testing.T, out []byte) {
				t.Helper()
				assert.NoError(t, decrypt(out, passphrase))
			},
		},
		{
			name:       "any other passphrase does not",
			pdf:        fixture,
			passphrase: passphrase,
			checkFunc: func(t *testing.T, out []byte) {
				t.Helper()
				require.Error(t, decrypt(out, "wronghorsebattery"))
				require.Error(t, decrypt(out, ""), "the file cannot be opened without a password")
			},
		},
		{
			name:       "an empty passphrase is refused rather than producing an open file",
			pdf:        fixture,
			passphrase: "",
			wantErr:    true,
		},
		{
			name:       "input that is not a pdf is an error",
			pdf:        []byte("not a pdf"),
			passphrase: passphrase,
			wantErr:    true,
		},
		{
			name:       "a passphrase with spaces and accents encrypts",
			pdf:        fixture,
			passphrase: "café crème brûlée",
			checkFunc: func(t *testing.T, out []byte) {
				t.Helper()
				// No decrypt check: pdfcpu's Decrypt rejects this passphrase due to its PRECIS bug.
				assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
				assert.Contains(t, string(out), "/Encrypt")
			},
		},
		{
			name:       "each export gets a different owner password",
			pdf:        fixture,
			passphrase: passphrase,
			checkFunc: func(t *testing.T, out []byte) {
				t.Helper()
				again, encErr := render.Encrypt(fixture, passphrase)
				require.NoError(t, encErr)
				assert.False(t, bytes.Equal(out, again),
					"two encryptions of the same input should not be byte-identical")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, encErr := render.Encrypt(tt.pdf, tt.passphrase)
			if tt.wantErr {
				require.Error(t, encErr)
				if tt.passphrase != "" {
					assert.NotContains(t, encErr.Error(), tt.passphrase, "a passphrase never reaches an error message")
				}
				return
			}

			require.NoError(t, encErr)
			tt.checkFunc(t, out)
		})
	}
}
