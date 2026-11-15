package render

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// aesKeyLength is the strongest key pdfcpu offers, and what every current PDF
// reader opens.
const aesKeyLength = 256

// aesConfiguration is pdfcpu's AES configuration built on its stateless
// defaults. model.NewAESConfiguration starts from NewDefaultConfiguration,
// which creates a configuration directory under $HOME on first use; a
// container user may have no writable home, and the service has no reason to
// leave files there. The stateless base never touches the filesystem and needs
// no process-global switch.
func aesConfiguration(userPW, ownerPW string) *model.Configuration {
	conf := model.NewStatelessConfiguration()
	conf.UserPW = userPW
	conf.OwnerPW = ownerPW
	conf.EncryptUsingAES = true
	conf.EncryptKeyLength = aesKeyLength

	return conf
}

// Encrypt protects a rendered PDF with passphrase (§5.2, ADR-0011).
//
// passphrase becomes the user password — the one a relative types to open the
// file. The owner password, which governs changing the file's permissions,
// also opens the file: PDF standard security derives the file's encryption
// key from either password, so an owner password recoverable by brute force
// would let an attacker open the file without ever knowing passphrase. It is
// therefore random, at least 128 bits, generated per call, and discarded:
// nobody needs it, and PDF permission flags are advisory anyway (§5.8).
//
// An empty passphrase is an error rather than a no-op, so no caller can
// produce an unprotected Full-tier file by passing through an unset secret.
// Errors never include the passphrase.
//
// ctx is the request's: pdfcpu honours its cancellation, so an Editor who has
// already gone away is not waited on.
//
// Passphrases are ordinary phrases and may contain spaces and accents.
// pdfcpu encrypts them correctly; do not use its Decrypt function to verify
// a real passphrase, as it incorrectly applies PRECIS instead of SASLprep,
// which rejects spaces (its NFKC normalisation accepts accents fine).
func Encrypt(ctx context.Context, pdf []byte, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, errors.New("render: refusing to encrypt with an empty passphrase")
	}

	// rand.Text returns 26 characters (~130 bits) from the base32 alphabet
	// A-Z, 2-7, which pdfcpu's password handling accepts without issue.
	ownerPW := rand.Text()

	var out bytes.Buffer

	err := api.Encrypt(ctx, bytes.NewReader(pdf), &out, aesConfiguration(passphrase, ownerPW))
	if err != nil {
		return nil, fmt.Errorf("render: encrypt: %w", err)
	}

	return out.Bytes(), nil
}
