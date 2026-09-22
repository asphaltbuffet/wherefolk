package rolo

import "strings"

// housePhoneDigits is the number of digits in a North American phone number
// once any country code is removed.
const housePhoneDigits = 10

// NormalizePhone rewrites a North American phone number into the Directory's
// house style, 555-201-0001, and reports whether it recognised the input.
//
// Anything it does not recognise — an international number, an extension, two
// numbers in one field, a note like "call the house" — is returned trimmed but
// otherwise exactly as typed, with recognized false. Discarding a relative's
// real phone number to enforce a format would be worse than an inconsistent
// directory, so normalisation never destroys input.
//
// The empty string returns ("", true): there is nothing to normalise, and most
// people in the Directory have no phone on file. That is not a failure.
func NormalizePhone(s string) (normalized string, recognized bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", true
	}

	// An extension, a list, or a slashed alternative cannot be reduced to a
	// single number, so reformatting would lose information.
	if strings.ContainsAny(trimmed, "xX#,;/") {
		return trimmed, false
	}

	var b strings.Builder
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			b.WriteByte(byte(r))
		}
	}
	digits := b.String()

	// A leading + means an international number unless the country code is 1.
	if strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(digits, "1") {
		return trimmed, false
	}

	// Drop a North American country code.
	if len(digits) == housePhoneDigits+1 && digits[0] == '1' {
		digits = digits[1:]
	}

	if len(digits) != housePhoneDigits {
		return trimmed, false
	}

	return digits[0:3] + "-" + digits[3:6] + "-" + digits[6:], true
}
