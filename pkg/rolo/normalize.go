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

// NormalizeEmail trims an address and lowercases its domain.
//
// Domains are case-insensitive, so Example.COM and example.com are the same
// host and the Directory should render one of them. Local parts are
// case-sensitive in principle, so Pat.Novak is left exactly as the Editor
// typed it.
//
// This function does not judge whether the value is a valid address — that is
// Validate's job, and an address it cannot parse is still returned trimmed
// rather than discarded.
func NormalizeEmail(s string) string {
	trimmed := strings.TrimSpace(s)

	at := strings.LastIndex(trimmed, "@")
	if at < 0 {
		return trimmed
	}

	return trimmed[:at] + "@" + strings.ToLower(trimmed[at+1:])
}

// Normalize rewrites the Person's fields into house style in place. It never
// fails and never empties a field that held something.
func (p *Person) Normalize() {
	p.Given = strings.TrimSpace(p.Given)
	p.Surname = strings.TrimSpace(p.Surname)
	p.BirthName = strings.TrimSpace(p.BirthName)
	p.Aka = strings.TrimSpace(p.Aka)

	p.Phone, _ = NormalizePhone(p.Phone)
	p.Email = NormalizeEmail(p.Email)
}

// Normalize rewrites the Household and everyone in it into house style in
// place. Structure — the ID, the Parent link, a Shared Address reference — is
// never touched: normalisation is about presentation, not about who belongs
// where.
func (h *Household) Normalize() {
	for i := range h.Adults {
		h.Adults[i].Normalize()
	}
	for i := range h.Dependents {
		h.Dependents[i].Normalize()
	}

	h.Address.normalize()
}

// normalize trims the Address's lines and drops any that are blank, so a
// stray empty line in the file does not become a blank line in the printed
// Directory.
func (a *Address) normalize() {
	if len(a.Lines) == 0 {
		return
	}

	kept := make([]string, 0, len(a.Lines))
	for _, line := range a.Lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}

	if len(kept) == 0 {
		a.Lines = nil
		return
	}
	a.Lines = kept
}
