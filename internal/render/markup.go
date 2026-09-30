package render

import (
	"strconv"
	"strings"
)

// templateImport names the functions the on-disk template must provide. The
// generated markup emits data and calls these; it never sets a margin, a font
// or a spacing value itself, because layout belongs to the template the
// Operator edits (ADR-0004, §5.1).
const templateImport = `#import "directory.typ": directory, households, household, memorial, birthdays`

// Markup renders d as Typst source.
//
// The output is deterministic for a given Directory: nothing here reads the
// clock, the filesystem or a map in range order, so a golden comparison is a
// stable test.
func Markup(d Directory) string {
	var b strings.Builder

	b.WriteString(templateImport)
	b.WriteString("\n\n#directory(title: ")
	b.WriteString(quote(d.Title))
	b.WriteString(", generated: ")
	b.WriteString(quote(d.GeneratedAt))
	b.WriteString(", tier: ")
	b.WriteString(quote(d.Tier))
	b.WriteString(", restricted: ")
	b.WriteString(strconv.FormatBool(d.Restricted))
	b.WriteString(")[\n")

	// The section's heading is template copy, like the Birthday Calendar's, so
	// the call wraps the blocks rather than naming them.
	if len(d.Households) > 0 {
		b.WriteString("  #households[\n")

		for _, h := range d.Households {
			writeHousehold(&b, h)
		}

		b.WriteString("  ]\n")
	}

	if len(d.Birthdays) > 0 {
		writeBirthdays(&b, d.Birthdays)
	}

	b.WriteString("]\n")

	return b.String()
}

// writeHousehold emits one block: the compact memorial form, or the full one.
func writeHousehold(b *strings.Builder, h Household) {
	call := "#household("
	if h.Memorial {
		call = "#memorial("
	}

	b.WriteString("  ")
	b.WriteString(call)
	b.WriteString("\n    id: ")
	b.WriteString(quote(h.ID))
	b.WriteString(",\n    name: ")
	b.WriteString(quote(h.Name))
	b.WriteString(",\n    anniversary: ")
	b.WriteString(quote(h.Anniversary))
	b.WriteString(",\n    address: ")
	b.WriteString(quoteList(h.AddressLines))
	b.WriteString(",\n    shared: ")
	b.WriteString(quote(h.SharedWith))
	b.WriteString(",\n    adults: ")
	b.WriteString(peopleArray(h.Adults))
	b.WriteString(",\n    dependents: ")
	b.WriteString(peopleArray(h.Dependents))
	b.WriteString(",\n    contents: ")
	b.WriteString(strconv.FormatBool(h.Contents))
	b.WriteString(",\n  )\n")
}

// writeBirthdays emits the Birthday Calendar as one call, inside the
// directory body so it inherits the page setup and footer. The trailing comma
// after each row keeps a single-row calendar an array (see peopleArray).
func writeBirthdays(b *strings.Builder, rows []Birthday) {
	b.WriteString("  #birthdays((")

	for _, r := range rows {
		b.WriteString("\n    (name: ")
		b.WriteString(quote(r.Name))
		b.WriteString(", month: ")
		b.WriteString(strconv.Itoa(r.Month))
		b.WriteString(", day: ")
		b.WriteString(quote(r.Day))
		b.WriteString("),")
	}

	b.WriteString("\n  ))\n")
}

// peopleArray renders a slice of people as a Typst array of dictionaries.
//
// The trailing comma is not cosmetic: in Typst a single-element parenthesised
// expression is that element, not an array, so "(x)" and "(x,)" differ.
func peopleArray(people []Person) string {
	if len(people) == 0 {
		return "()"
	}

	var b strings.Builder

	b.WriteString("(")

	for _, p := range people {
		b.WriteString("\n      (name: ")
		b.WriteString(quote(p.Name))
		b.WriteString(", birth: ")
		b.WriteString(quote(p.Birth))
		b.WriteString(", death: ")
		b.WriteString(quote(p.Death))
		b.WriteString(", phone: ")
		b.WriteString(quote(p.Phone))
		b.WriteString(", email: ")
		b.WriteString(quote(p.Email))
		b.WriteString("),")
	}

	b.WriteString("\n    )")

	return b.String()
}

// quoteList renders a slice of strings as a Typst array, with the same trailing
// comma rule peopleArray documents.
func quoteList(items []string) string {
	if len(items) == 0 {
		return "()"
	}

	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, quote(s)+",")
	}

	return "(" + strings.Join(parts, " ") + ")"
}

// quote renders s as a Typst string literal.
//
// It escapes the backslash and the quotation mark, and nothing else — those are
// the only characters a string literal treats as special, alongside \n, \t and
// \u{}. Typst's markup metacharacters ("#", "@", "*", "_", "[", "$", "<") are
// ordinary text in this context, and escaping them would put a literal
// backslash into the printed Directory: an address would come out as
// "\#1 P.O. Box 212".
//
// Verified against typst 0.14.2: `"#1 Elm St".len()` is 9, while
// `"\#1 Elm St".at(0)` is a backslash and its length is 10.
//
// Every value this generator emits is a string literal, so these are the only
// rules that apply. If a later change ever interpolates a value into markup
// context instead, it needs a different escaper — this one is not it.
//
// DisplayName produces quotation marks for any Person with a nickname, so the
// quote case is a live path, not a defensive one.
func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)

	return `"` + s + `"`
}
