package render

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// aesKeyLength is the strongest key pdfcpu offers, and what every current PDF
// reader opens.
const aesKeyLength = 256

// ownerPasswordLength is the byte length of the randomly generated owner password.
const ownerPasswordLength = 4

// disableConfigDir stops pdfcpu creating a configuration directory under $HOME
// on first use. A container user may have no writable home, and the service
// has no reason to leave files there. The setting is process-global, so it is
// applied once.
var disableConfigDir = sync.OnceFunc(api.DisableConfigDir)

//nolint:gochecknoinits // pdfcpu disables config-dir creation once, globally; must be called at package init
func init() {
	disableConfigDir()
}

// Encrypt protects a rendered PDF with passphrase (§5.2, ADR-0011).
//
// passphrase becomes the user password — the one a relative types to open the
// file. The owner password, which governs changing the file's permissions, is
// random per call and discarded: nobody needs it, and PDF permission flags are
// advisory anyway (§5.8).
//
// An empty passphrase is an error rather than a no-op, so no caller can
// produce an unprotected Full-tier file by passing through an unset secret.
// Errors never include the passphrase.
//
// Passphrases are ordinary phrases and may contain spaces and accents.
// pdfcpu encrypts them correctly; do not use its Decrypt function to verify
// a real passphrase, as it incorrectly applies PRECIS instead of SASLprep.
func Encrypt(pdf []byte, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, errors.New("render: refusing to encrypt with an empty passphrase")
	}

	disableConfigDir()

	// Generate a random owner password
	randomBytes := make([]byte, ownerPasswordLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("render: encrypt: %w", err)
	}
	ownerPW := hex.EncodeToString(randomBytes)

	conf := model.NewAESConfiguration(passphrase, ownerPW, aesKeyLength)

	var out bytes.Buffer

	err := api.Encrypt(bytes.NewReader(pdf), &out, conf)
	if err != nil {
		return nil, fmt.Errorf("render: encrypt: %w", err)
	}

	return out.Bytes(), nil
}
