package render

import (
	"strconv"
	"strings"
)

// proofImport names the template function a Proof Sheet calls. It is separate
// from directory() because a Proof Sheet carries no Title page and no Table of
// Contents (CONTEXT.md, Proof Sheet).
const proofImport = `#import "directory.typ": proof`

// ProofMarkup renders s as Typst source.
//
// Only field IDs and withheld flags reach Typst, never field values: a value
// lives only in its form field (ADR-0015), so the printed page cannot disagree
// with what the recipient edits. The strings that are printed (title, name,
// reply, shared-with) go through quote like every other value.
func ProofMarkup(s ProofSheet) string {
	var b strings.Builder

	b.WriteString(proofImport)
	b.WriteString("\n\n#proof(")
	arg(&b, "title", quote(s.Title))
	arg(&b, "generated", quote(s.GeneratedAt))
	arg(&b, "reply", quote(s.Reply))
	arg(&b, "name", quote(s.Name))
	arg(&b, "shared", quote(s.SharedWith))
	arg(&b, "address", proofFieldArray(s.Address))
	arg(&b, "address_withheld", strconv.FormatBool(s.AddressWithheld))
	arg(&b, "keep_address", quote(s.KeepAddress.ID))
	arg(&b, "moved", quote(s.Moved.ID))
	arg(&b, "anniversary", proofField(s.Anniversary))
	arg(&b, "adults", proofPeopleArray(s.Adults))
	arg(&b, "dependents", proofPeopleArray(s.Dependents))
	arg(&b, "missing", proofPeopleArray(s.Missing))
	arg(&b, "all_correct", quote(s.AllCorrect.ID))
	arg(&b, "notes", quote(s.Notes.ID))
	b.WriteString("\n)\n")

	return b.String()
}

// arg writes one named argument on its own line, trailing comma included.
func arg(b *strings.Builder, name, value string) {
	b.WriteString("\n  ")
	b.WriteString(name)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString(",")
}

// proofField is a field as the template sees it: its ID and whether to mark it
// withheld, never its value.
func proofField(f ProofField) string {
	return "(id: " + quote(f.ID) + ", withheld: " + strconv.FormatBool(f.Withheld) + ")"
}

// proofFieldArray keeps the trailing comma peopleArray documents, so a
// one-element array stays an array.
func proofFieldArray(fs []ProofField) string {
	if len(fs) == 0 {
		return "()"
	}

	parts := make([]string, 0, len(fs))
	for _, f := range fs {
		parts = append(parts, proofField(f)+",")
	}

	return "(" + strings.Join(parts, " ") + ")"
}

// proofPeopleArray renders people as a Typst array of dictionaries, one per
// line, with the trailing comma peopleArray documents.
func proofPeopleArray(ps []ProofPerson) string {
	if len(ps) == 0 {
		return "()"
	}

	var b strings.Builder

	b.WriteString("(")

	for _, p := range ps {
		b.WriteString("\n    ")
		b.WriteString(proofPersonDict(p))
		b.WriteString(",")
	}

	b.WriteString("\n  )")

	return b.String()
}

// proofPersonDict is one person's dictionary: fields as proofField, checks as
// their bare IDs.
func proofPersonDict(p ProofPerson) string {
	fields := []struct {
		k string
		f ProofField
	}{
		{"given", p.Given}, {"surname", p.Surname}, {"aka", p.Aka}, {"birth_name", p.BirthName},
		{"birth", p.Birth}, {"death", p.Death}, {"phone", p.Phone}, {"email", p.Email},
		{"relationship", p.Relationship},
	}
	checks := []struct {
		k string
		c ProofCheck
	}{
		{"keep_birth", p.KeepBirth}, {"keep_phone", p.KeepPhone}, {"keep_email", p.KeepEmail},
		{"remove", p.Remove},
	}

	parts := make([]string, 0, 1+len(fields)+len(checks))
	parts = append(parts, "deceased: "+strconv.FormatBool(p.Deceased))

	for _, kv := range fields {
		parts = append(parts, kv.k+": "+proofField(kv.f))
	}

	for _, kv := range checks {
		parts = append(parts, kv.k+": "+quote(kv.c.ID))
	}

	return "(" + strings.Join(parts, ", ") + ")"
}
