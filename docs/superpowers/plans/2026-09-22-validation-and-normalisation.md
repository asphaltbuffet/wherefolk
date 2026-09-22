# Validation & Normalisation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Normalise phone numbers and email addresses to a house style on save, and report data-quality problems as structured findings the UI can display beside the offending field — without ever rejecting the Editor's input.

**Architecture:** Two separate layers with different jobs. **Normalisation** (`pkg/rolo/normalize.go`) rewrites a value into house style and is called explicitly by the editing layer before save; it never fails, and leaves anything it cannot confidently reformat exactly as typed. **Validation** (`pkg/rolo/validate.go`) inspects a whole `Document` and returns `[]Finding` — it changes nothing and blocks nothing. Both are pure domain logic in `pkg/rolo`; `store.Load` and `store.Save` are untouched, so a hand-repaired file is never silently rewritten.

**Tech Stack:** Go 1.24 standard library only — `strings`, `net/mail`. No new dependencies.

## Global Constraints

- **Terminology** comes from `CONTEXT.md`. Use `Household`, `Dependent`, `Branch`, `Path`, `Memorial Household`. The word `Family` is banned in new code.
- **All tests are table-driven**: a `tests []struct{ name string; ... }` slice iterated with `t.Run(tt.name, ...)`. Rows needing assertions beyond field comparison use a `checkFunc func(t *testing.T, ...)` field. This is a project rule from `CLAUDE.md`.
- **Tests live in an external test package** (`package rolo_test`).
- **Module path** is `github.com/asphaltbuffet/wherefolk`.
- **VCS is jujutsu (`jj`), not git.** Commit with `jj commit -m "..."`. Do not run `git` commands.
- **Validation never blocks and never rejects.** Design §4.5: "Forgiving, and phrased as observation rather than rejection… failures explain rather than block." A `Finding` is a note, not an error.
- **Normalisation never destroys input.** If a value cannot be confidently reformatted, it is kept verbatim. Design §4.4: input is forgiving, storage is consistent — but a relative's international number is not garbage to be discarded.
- **An empty field is not a problem.** Most people in the Directory have no email; a missing birthdate is "a fact with an export consequence" (§4.5), surfaced at export, never nagged about during editing. Empty must never produce a Finding in this work item.
- **`store.Load` and `store.Save` are not modified by this work item.**
- **No new dependencies.** `net/mail` is stdlib.

## File Structure

| File | Responsibility |
|---|---|
| `pkg/rolo/normalize.go` | `NormalizePhone`, `NormalizeEmail`, and the `Normalize()` methods that apply them across a Person and a Household. |
| `pkg/rolo/validate.go` | `Finding`, `Severity`, and `Validate(*Document-shaped input) []Finding`. |
| `pkg/rolo/normalize_test.go` | Table-driven tests for both normalisers and both methods. |
| `pkg/rolo/validate_test.go` | Table-driven tests for findings, including the must-not-fire cases. |

Nothing existing is modified except `CLAUDE.md` in the final task.

**Why `pkg/rolo` and not `internal/store`:** normalisation and validation are statements about the *domain* — what a phone number is, what makes a Household's data questionable — not about persistence. Keeping them in `rolo` means `store` stays a persistence layer that knows nothing about phone formats, and it keeps the rule that a hand-edited file is loaded exactly as written.

---

### Task 1: Phone normalisation

`testdata/directory.json` stores phone numbers as `555-201-0001`. This task makes that the house style for anything recognisably a North American number, while leaving everything else untouched.

**Files:**
- Create: `pkg/rolo/normalize.go`
- Test: `pkg/rolo/normalize_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `func NormalizePhone(s string) (normalized string, recognized bool)`

`recognized` reports whether the input was reduced to house style. A `false` return is **not** an error — the caller stores the verbatim value and may raise a Finding (Task 3). The empty string returns `("", true)`: nothing to normalise is not a failure.

- [ ] **Step 1: Write the failing test**

Create `pkg/rolo/normalize_test.go`:

```go
package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		want           string
		wantRecognized bool
	}{
		{
			name:           "already in house style",
			input:          "555-201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "parenthesised area code",
			input:          "(555) 201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "dot separated",
			input:          "555.201.0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "space separated",
			input:          "555 201 0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "bare digits",
			input:          "5552010001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "leading country code without plus",
			input:          "15552010001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "explicit +1 country code",
			input:          "+1 555 201 0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "explicit +1 with punctuation",
			input:          "+1 (555) 201-0001",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "surrounding whitespace is trimmed",
			input:          "  555-201-0001  ",
			want:           "555-201-0001",
			wantRecognized: true,
		},
		{
			name:           "empty is not a failure",
			input:          "",
			want:           "",
			wantRecognized: true,
		},
		{
			name:           "whitespace only becomes empty",
			input:          "   ",
			want:           "",
			wantRecognized: true,
		},
		{
			name:           "international number is kept verbatim",
			input:          "+44 20 7946 0958",
			want:           "+44 20 7946 0958",
			wantRecognized: false,
		},
		{
			name:           "another international number is kept verbatim",
			input:          "+61 2 9374 4000",
			want:           "+61 2 9374 4000",
			wantRecognized: false,
		},
		{
			name:           "extension is kept verbatim",
			input:          "555-201-0001 x12",
			want:           "555-201-0001 x12",
			wantRecognized: false,
		},
		{
			name:           "two numbers in one field are kept verbatim",
			input:          "555-201-0001, 555-201-0002",
			want:           "555-201-0001, 555-201-0002",
			wantRecognized: false,
		},
		{
			name:           "too few digits is kept verbatim",
			input:          "555-0001",
			want:           "555-0001",
			wantRecognized: false,
		},
		{
			name:           "vanity number is kept verbatim",
			input:          "1-800-FLOWERS",
			want:           "1-800-FLOWERS",
			wantRecognized: false,
		},
		{
			name:           "a note rather than a number is kept verbatim",
			input:          "call the house",
			want:           "call the house",
			wantRecognized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, recognized := rolo.NormalizePhone(tt.input)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantRecognized, recognized)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/rolo/ -run TestNormalizePhone -v`
Expected: FAIL — `undefined: rolo.NormalizePhone`

- [ ] **Step 3: Write the implementation**

Create `pkg/rolo/normalize.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/rolo/ -run TestNormalizePhone -v`
Expected: PASS — 18 subtests

- [ ] **Step 5: Verify the whole module is healthy**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build succeeds, vet silent, all packages pass

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(rolo): normalise North American phone numbers to house style"
```

---

### Task 2: Email normalisation and the Normalize methods

`net/mail.ParseAddress` is the validator (Task 3 uses it). This task handles the two things worth *changing* about an email: trimming it, and lowercasing the domain. Domains are case-insensitive per RFC 1035, so `Example.COM` and `example.com` are the same host; local parts are technically case-sensitive, so `Pat` is left alone.

**Files:**
- Modify: `pkg/rolo/normalize.go`
- Test: `pkg/rolo/normalize_test.go`

**Interfaces:**
- Consumes: `NormalizePhone` (Task 1)
- Produces:
  - `func NormalizeEmail(s string) string`
  - `func (p *Person) Normalize()`
  - `func (h *Household) Normalize()`

Both methods take pointer receivers and mutate in place. `Household.Normalize` also normalises every adult and Dependent, and trims blank lines out of the Address.

- [ ] **Step 1: Write the failing test**

Append to `pkg/rolo/normalize_test.go`:

```go
func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "already normal",
			input: "pat.novak@example.com",
			want:  "pat.novak@example.com",
		},
		{
			name:  "domain is lowercased",
			input: "pat@Example.COM",
			want:  "pat@example.com",
		},
		{
			name:  "local part keeps its case",
			input: "Pat.Novak@example.com",
			want:  "Pat.Novak@example.com",
		},
		{
			name:  "surrounding whitespace is trimmed",
			input: "  pat@example.com  ",
			want:  "pat@example.com",
		},
		{
			name:  "empty stays empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only becomes empty",
			input: "   ",
			want:  "",
		},
		{
			name:  "a value with no at sign is left alone but trimmed",
			input: "  not an email  ",
			want:  "not an email",
		},
		{
			name:  "only the last at sign separates the domain",
			input: "odd@local@Example.COM",
			want:  "odd@local@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, rolo.NormalizeEmail(tt.input))
		})
	}
}

func TestPersonNormalize(t *testing.T) {
	tests := []struct {
		name      string
		person    rolo.Person
		checkFunc func(t *testing.T, p rolo.Person)
	}{
		{
			name:   "phone and email are both normalised",
			person: rolo.Person{Given: "Pat", Phone: "(555) 201-0001", Email: "pat@Example.COM"},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, "555-201-0001", p.Phone)
				assert.Equal(t, "pat@example.com", p.Email)
			},
		},
		{
			name:   "an unrecognised phone survives untouched but trimmed",
			person: rolo.Person{Given: "Nigel", Phone: "  +44 20 7946 0958  "},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, "+44 20 7946 0958", p.Phone)
			},
		},
		{
			name:   "names are trimmed",
			person: rolo.Person{Given: "  Pat  ", Surname: "  Novak  ", Aka: "  Patty  ", BirthName: "  Marsh  "},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, "Pat", p.Given)
				assert.Equal(t, "Novak", p.Surname)
				assert.Equal(t, "Patty", p.Aka)
				assert.Equal(t, "Marsh", p.BirthName)
			},
		},
		{
			name:   "empty fields stay empty",
			person: rolo.Person{Given: "Mia"},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Empty(t, p.Phone)
				assert.Empty(t, p.Email)
			},
		},
		{
			name: "identity and hidden flags are not touched",
			person: rolo.Person{
				ID:     "p_pat001",
				Given:  "Pat",
				Phone:  "(555) 201-0001",
				Hidden: rolo.HiddenFields{Phone: true},
			},
			checkFunc: func(t *testing.T, p rolo.Person) {
				t.Helper()
				assert.Equal(t, rolo.PersonID("p_pat001"), p.ID)
				assert.True(t, p.Hidden.Phone, "normalising a withheld field must not un-withhold it")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.person
			p.Normalize()
			tt.checkFunc(t, p)
		})
	}
}

func TestHouseholdNormalize(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		checkFunc func(t *testing.T, h rolo.Household)
	}{
		{
			name: "adults are normalised",
			household: rolo.Household{
				Adults: []rolo.Person{
					{Given: "Pat", Phone: "555.201.0001"},
					{Given: "Sam", Email: "sam@Example.COM"},
				},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, "555-201-0001", h.Adults[0].Phone)
				assert.Equal(t, "sam@example.com", h.Adults[1].Email)
			},
		},
		{
			name: "dependents are normalised too",
			household: rolo.Household{
				Adults:     []rolo.Person{{Given: "Pat"}},
				Dependents: []rolo.Person{{Given: "Mia", Phone: "(555) 201-0020"}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, "555-201-0020", h.Dependents[0].Phone)
			},
		},
		{
			name: "address lines are trimmed and blanks removed",
			household: rolo.Household{
				Adults:  []rolo.Person{{Given: "Pat"}},
				Address: rolo.Address{Lines: []string{"  88 Oakwood Drive  ", "", "   ", "Shelbyville, IL 62565"}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, []string{"88 Oakwood Drive", "Shelbyville, IL 62565"}, h.Address.Lines)
			},
		},
		{
			name: "a shared address reference is not disturbed",
			household: rolo.Household{
				Adults:  []rolo.Person{{Given: "Pat"}},
				Address: rolo.Address{SharedWith: "h_lang01"},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, rolo.HouseholdID("h_lang01"), h.Address.SharedWith)
				assert.Empty(t, h.Address.Lines)
			},
		},
		{
			name: "an address of only blank lines becomes empty",
			household: rolo.Household{
				Adults:  []rolo.Person{{Given: "Pat"}},
				Address: rolo.Address{Lines: []string{"", "  "}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Empty(t, h.Address.Lines)
			},
		},
		{
			name: "structure is not touched",
			household: rolo.Household{
				ID:     "h_nova01",
				Parent: "h_lang01",
				Adults: []rolo.Person{{Given: "Pat", Phone: "555.201.0001"}},
			},
			checkFunc: func(t *testing.T, h rolo.Household) {
				t.Helper()
				assert.Equal(t, rolo.HouseholdID("h_nova01"), h.ID)
				assert.Equal(t, rolo.HouseholdID("h_lang01"), h.Parent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.household
			h.Normalize()
			tt.checkFunc(t, h)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/rolo/ -run 'TestNormalizeEmail|TestPersonNormalize|TestHouseholdNormalize' -v`
Expected: FAIL — `undefined: rolo.NormalizeEmail`, and `p.Normalize undefined`

- [ ] **Step 3: Write the implementation**

Append to `pkg/rolo/normalize.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/rolo/ -run 'TestNormalizeEmail|TestPersonNormalize|TestHouseholdNormalize' -v`
Expected: PASS — 19 subtests across three functions

- [ ] **Step 5: Verify the whole module is healthy**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build succeeds, vet silent, all packages pass

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(rolo): add email normalisation and Normalize methods"
```

---

### Task 3: Findings and validation

Validation reports; it never rejects. A `Finding` carries enough location for the UI to render it beside the field it concerns.

**Files:**
- Create: `pkg/rolo/validate.go`
- Test: `pkg/rolo/validate_test.go`

**Interfaces:**
- Consumes: `NormalizePhone` (Task 1), `Person`, `Household`, `HouseholdID`, `PersonID`
- Produces:
  - `type Severity int` with `SeverityNotice` and `SeverityWarning`
  - `func (s Severity) String() string`
  - `type Finding struct{ Household HouseholdID; Person PersonID; Field string; Message string; Severity Severity }`
  - `func ValidateHouseholds(households []Household) []Finding`

`ValidateHouseholds` takes a slice rather than a `*Document` because `Document` lives in `internal/store`, which imports `rolo` — the dependency runs one way and must keep doing so. The store will call this with `doc.Households`.

- [ ] **Step 1: Write the failing test**

Create `pkg/rolo/validate_test.go`:

```go
package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// findingFor returns the first Finding about the given field, and whether one
// was present at all.
func findingFor(findings []rolo.Finding, field string) (rolo.Finding, bool) {
	for _, f := range findings {
		if f.Field == field {
			return f, true
		}
	}
	return rolo.Finding{}, false
}

func TestValidateHouseholdsReportsProblems(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		checkFunc func(t *testing.T, findings []rolo.Finding)
	}{
		{
			name: "an unrecognised phone is a notice",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Nigel", Phone: "+44 20 7946 0958"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "phone")
				require.True(t, ok, "expected a finding about phone")
				assert.Equal(t, rolo.SeverityNotice, f.Severity)
				assert.Equal(t, rolo.HouseholdID("h_a"), f.Household)
				assert.Equal(t, rolo.PersonID("p_a"), f.Person)
				assert.Contains(t, f.Message, "+44 20 7946 0958",
					"the message should quote the value so the Editor can see what it means")
			},
		},
		{
			name: "an unparseable email is a warning",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "not an email"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "email")
				require.True(t, ok, "expected a finding about email")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
				assert.Contains(t, f.Message, "not an email")
			},
		},
		{
			name: "an email with a display name is a warning",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "Pat Novak <pat@example.com>"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "email")
				require.True(t, ok, "a display-name address should be flagged, not silently accepted")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
			},
		},
		{
			name: "a death date before a birth date is a warning",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Thomas",
					Birth: rolo.Date{Year: 1996, Month: 7, Day: 19},
					Death: rolo.Date{Year: 1990, Month: 1, Day: 8},
				}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "death")
				require.True(t, ok, "expected a finding about death")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
			},
		},
		{
			name: "an anniversary before an adult's birth is a warning",
			household: rolo.Household{
				ID:          "h_a",
				Anniversary: rolo.Date{Year: 1960},
				Adults:      []rolo.Person{{ID: "p_a", Given: "Pat", Birth: rolo.Date{Year: 1975}}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "anniversary")
				require.True(t, ok, "expected a finding about anniversary")
				assert.Equal(t, rolo.SeverityWarning, f.Severity)
				assert.Equal(t, rolo.PersonID(""), f.Person,
					"an anniversary belongs to the Household, not to one Person")
			},
		},
		{
			name: "problems are reported for dependents too",
			household: rolo.Household{
				ID:         "h_a",
				Adults:     []rolo.Person{{ID: "p_a", Given: "Pat"}},
				Dependents: []rolo.Person{{ID: "p_kid", Given: "Mia", Email: "not an email"}},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				f, ok := findingFor(findings, "email")
				require.True(t, ok)
				assert.Equal(t, rolo.PersonID("p_kid"), f.Person)
			},
		},
		{
			name: "several problems produce several findings",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{
					{ID: "p_a", Given: "Pat", Phone: "call the house", Email: "not an email"},
				},
			},
			checkFunc: func(t *testing.T, findings []rolo.Finding) {
				t.Helper()
				assert.Len(t, findings, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkFunc(t, rolo.ValidateHouseholds([]rolo.Household{tt.household}))
		})
	}
}

func TestValidateHouseholdsStaysQuiet(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
	}{
		{
			name: "a complete, well-formed household",
			household: rolo.Household{
				ID:          "h_a",
				Anniversary: rolo.Date{Year: 1991, Month: 6, Day: 15},
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Robert",
					Birth: rolo.Date{Year: 1965, Month: 3, Day: 12},
					Phone: "555-201-0001",
					Email: "robert@example.com",
				}},
			},
		},
		{
			name: "no contact details at all",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Mia"}},
			},
		},
		{
			name: "no birth date is not a problem during editing",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Harold", Phone: "555-201-0001"}},
			},
		},
		{
			name: "a deceased person with sensible dates",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Thomas",
					Birth: rolo.Date{Year: 1996, Month: 7, Day: 19},
					Death: rolo.Date{Year: 2022, Month: 1, Day: 8},
				}},
			},
		},
		{
			name: "a death date with no birth date cannot be compared",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Nettie", Death: rolo.Date{Year: 1995}}},
			},
		},
		{
			name: "an anniversary with no birth dates cannot be compared",
			household: rolo.Household{
				ID:          "h_a",
				Anniversary: rolo.Date{Year: 1961},
				Adults:      []rolo.Person{{ID: "p_a", Given: "Clyde"}},
			},
		},
		{
			name: "a year-only birth equal to the death year is not backwards",
			household: rolo.Household{
				ID: "h_a",
				Adults: []rolo.Person{{
					ID:    "p_a",
					Given: "Infant",
					Birth: rolo.Date{Year: 2020},
					Death: rolo.Date{Year: 2020},
				}},
			},
		},
		{
			name: "an email that is unusual but valid",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "pat@localhost"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Empty(t, rolo.ValidateHouseholds([]rolo.Household{tt.household}),
				"validation must stay quiet about data that is merely incomplete")
		})
	}
}

func TestValidateHouseholdsAcrossTheDirectory(t *testing.T) {
	households := []rolo.Household{
		{ID: "h_a", Adults: []rolo.Person{{ID: "p_a", Given: "Pat", Email: "not an email"}}},
		{ID: "h_b", Adults: []rolo.Person{{ID: "p_b", Given: "Sam", Phone: "555-201-0002"}}},
		{ID: "h_c", Adults: []rolo.Person{{ID: "p_c", Given: "Nigel", Phone: "call the house"}}},
	}

	findings := rolo.ValidateHouseholds(households)
	require.Len(t, findings, 2)

	assert.Equal(t, rolo.HouseholdID("h_a"), findings[0].Household,
		"findings should arrive in document order")
	assert.Equal(t, rolo.HouseholdID("h_c"), findings[1].Household)
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		name     string
		severity rolo.Severity
		want     string
	}{
		{name: "notice", severity: rolo.SeverityNotice, want: "notice"},
		{name: "warning", severity: rolo.SeverityWarning, want: "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.severity.String())
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/rolo/ -run 'TestValidate|TestSeverity' -v`
Expected: FAIL — `undefined: rolo.ValidateHouseholds`, `undefined: rolo.Finding`

- [ ] **Step 3: Write the implementation**

Create `pkg/rolo/validate.go`:

```go
package rolo

import (
	"fmt"
	"net/mail"
	"strings"
)

// Severity says how much attention a Finding deserves. Neither level blocks
// anything: validation is an observation, never a rejection.
type Severity int

const (
	// SeverityNotice marks something the Editor probably meant, which will
	// simply appear in the Directory as typed.
	SeverityNotice Severity = iota

	// SeverityWarning marks something likely to be a mistake — a date that
	// runs backwards, an address that cannot be parsed.
	SeverityWarning
)

// String renders the severity for display.
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	default:
		return "notice"
	}
}

// Finding is one observation about the Directory's contents, carrying enough
// location for the UI to show it beside the field it concerns.
//
// Person is empty when the finding is about the Household itself, such as an
// Anniversary.
type Finding struct {
	Household HouseholdID
	Person    PersonID
	Field     string
	Message   string
	Severity  Severity
}

// ValidateHouseholds inspects every Household and reports what looks wrong. It
// changes nothing and rejects nothing.
//
// It stays silent about data that is merely absent. Most people in the
// Directory have no email, and a missing birth date is a fact with an export
// consequence rather than an error — export surfaces that, so nagging about it
// during editing would train the Editor to ignore findings.
//
// Findings arrive in document order so the UI can show them against the tree
// without sorting.
func ValidateHouseholds(households []Household) []Finding {
	var findings []Finding

	for _, h := range households {
		for _, p := range h.Adults {
			findings = append(findings, validatePerson(h.ID, p)...)
		}
		for _, p := range h.Dependents {
			findings = append(findings, validatePerson(h.ID, p)...)
		}
		findings = append(findings, validateAnniversary(h)...)
	}

	return findings
}

// validatePerson reports on one Person's contact details and dates.
func validatePerson(household HouseholdID, p Person) []Finding {
	var findings []Finding

	add := func(field, message string, severity Severity) {
		findings = append(findings, Finding{
			Household: household,
			Person:    p.ID,
			Field:     field,
			Message:   message,
			Severity:  severity,
		})
	}

	if p.Phone != "" {
		if _, recognized := NormalizePhone(p.Phone); !recognized {
			add("phone", fmt.Sprintf("%q is not a standard phone number, so it will print exactly as written", p.Phone), SeverityNotice)
		}
	}

	if p.Email != "" {
		if err := checkEmail(p.Email); err != nil {
			add("email", fmt.Sprintf("%q does not look like an email address: %s", p.Email, err), SeverityWarning)
		}
	}

	// Both dates must be known before they can be compared. A year-only date
	// is compared at its earliest instant, so a birth and a death in the same
	// year is not backwards.
	if !p.Birth.IsZero() && !p.Death.IsZero() && dateLess(p.Death, p.Birth) {
		add("death", fmt.Sprintf("died %s but was born %s", p.Death, p.Birth), SeverityWarning)
	}

	return findings
}

// validateAnniversary reports an Anniversary that predates an adult's birth.
func validateAnniversary(h Household) []Finding {
	if h.Anniversary.IsZero() {
		return nil
	}

	for _, p := range h.Adults {
		if p.Birth.IsZero() {
			continue
		}
		if dateLess(h.Anniversary, p.Birth) {
			return []Finding{{
				Household: h.ID,
				Field:     "anniversary",
				Message: fmt.Sprintf("the anniversary %s is before %s was born (%s)",
					h.Anniversary, strings.TrimSpace(p.Given+" "+p.Surname), p.Birth),
				Severity: SeverityWarning,
			}}
		}
	}

	return nil
}

// checkEmail reports whether a value is a plain email address.
//
// net/mail.ParseAddress also accepts the display-name form, Pat Novak
// <pat@example.com>, which is not what belongs in a directory's email field —
// so an address that parses but carries a name is rejected here.
func checkEmail(s string) error {
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return err
	}
	if addr.Name != "" {
		return fmt.Errorf("it includes a name; enter only the address, %s", addr.Address)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/rolo/ -run 'TestValidate|TestSeverity' -v`
Expected: PASS — 19 subtests across four functions

- [ ] **Step 5: Verify the whole module is healthy**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build succeeds, vet silent, all packages pass

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(rolo): report data-quality findings without rejecting input"
```

---

### Task 4: Prove it against the real dataset, and document it

The example Directory must produce no findings — if it did, the Editor would be greeted by warnings about data that is fine. This also pins normalisation as idempotent, which matters because `testdata/directory.json` is byte-compared by `TestExampleDirectoryIsCanonicallyFormatted`.

**Files:**
- Test: `pkg/rolo/validate_test.go` (append)
- Test: `internal/store/golden_test.go` (append)
- Modify: `CLAUDE.md`

**Interfaces:**
- Consumes: everything above, plus `store.Load` and `store.Document`
- Produces: nothing new

- [ ] **Step 1: Write the failing test**

Append to `internal/store/golden_test.go`:

```go
func TestExampleDirectoryHasNoFindings(t *testing.T) {
	doc, err := store.Load(examplePath)
	require.NoError(t, err)

	findings := rolo.ValidateHouseholds(doc.Households)
	assert.Empty(t, findings,
		"the example Directory should be clean; a finding here means either the data or the rules are wrong")
}

func TestExampleDirectoryIsAlreadyNormalised(t *testing.T) {
	doc, err := store.Load(examplePath)
	require.NoError(t, err)

	before, err := os.ReadFile(examplePath)
	require.NoError(t, err)

	for i := range doc.Households {
		doc.Households[i].Normalize()
	}

	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, doc))

	after, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Equal(t, string(before), string(after),
		"normalising the example Directory must change nothing, or the fixture is not in house style")
}
```

Append to `pkg/rolo/validate_test.go`:

```go
// deepCopyHousehold copies a Household including its slice contents.
//
// A plain struct assignment shares the Adults, Dependents and Address.Lines
// backing arrays, so normalising the copy would mutate the original too — and
// an idempotence test written that way compares a value to itself and can
// never fail.
func deepCopyHousehold(h rolo.Household) rolo.Household {
	out := h
	out.Adults = append([]rolo.Person(nil), h.Adults...)
	out.Dependents = append([]rolo.Person(nil), h.Dependents...)
	out.Address.Lines = append([]string(nil), h.Address.Lines...)
	return out
}

func TestNormalizeIsIdempotent(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
	}{
		{
			name: "recognised phone and email",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "  Pat  ", Phone: "(555) 201-0001", Email: "pat@Example.COM"}},
			},
		},
		{
			name: "unrecognised phone",
			household: rolo.Household{
				ID:     "h_a",
				Adults: []rolo.Person{{ID: "p_a", Given: "Nigel", Phone: "+44 20 7946 0958"}},
			},
		},
		{
			name: "address with blank lines",
			household: rolo.Household{
				ID:      "h_a",
				Adults:  []rolo.Person{{ID: "p_a", Given: "Pat"}},
				Address: rolo.Address{Lines: []string{"  88 Oakwood Drive  ", "", "Shelbyville, IL 62565"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			once := deepCopyHousehold(tt.household)
			once.Normalize()

			twice := deepCopyHousehold(tt.household)
			twice.Normalize()
			twice.Normalize()

			assert.Equal(t, once, twice, "normalising twice must equal normalising once")
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'TestExampleDirectoryHasNoFindings|TestExampleDirectoryIsAlreadyNormalised' -v`
Expected: FAIL — `undefined: rolo` if the import is missing, or `rolo.ValidateHouseholds` undefined. Add `"github.com/asphaltbuffet/wherefolk/pkg/rolo"` and `"os"`/`"path/filepath"` to `golden_test.go`'s imports if they are not already present.

- [ ] **Step 3: Make the tests pass**

No production code is needed — Tasks 1–3 implement everything.

If `TestExampleDirectoryHasNoFindings` fails, read the finding before changing anything. Either `testdata/directory.json` contains something genuinely odd, or a validation rule is too eager. Fix whichever is actually wrong; do not silence the rule to make the test pass.

If `TestExampleDirectoryIsAlreadyNormalised` fails, the fixture is not in house style. Correct the fixture, then re-run `TestExampleDirectoryIsCanonicallyFormatted` — both must pass together.

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS for `pkg/rolo` and `internal/store`

- [ ] **Step 5: Update CLAUDE.md**

In `CLAUDE.md`, add these bullets to the end of the `## Architecture` section's `pkg/rolo` list, matching the existing style:

```markdown
  - `NormalizePhone`/`NormalizeEmail` and the `Normalize()` methods rewrite values into house
    style; they never fail and never discard input they cannot reformat
  - `ValidateHouseholds` returns `[]Finding` — observations for the UI to display, never rejections
```

Then add this bullet to the `## Notes` section:

```markdown
- Normalisation is called explicitly by the editing layer, not by `store.Save` — a hand-repaired
  document is loaded and saved exactly as written.
```

- [ ] **Step 6: Verify and commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all succeed

```bash
jj commit -m "test(rolo): prove the example Directory is clean and normalisation is idempotent"
```

---

## Verification

After Task 4:

```bash
go build ./...                       # succeeds
go vet ./...                         # silent
go test ./...                        # all pass
rg -n "store\.(Load|Save)" pkg/rolo/ # no matches: rolo does not depend on the store
```

Design requirements covered:

| Requirement | Where |
|---|---|
| Normalise on save, display the normalised value (§4.4) | Task 1, Task 2 |
| Input forgiving, storage consistent (§4.4) | Task 1 — unrecognised values kept verbatim |
| Phones, emails, dates checked (§4.5) | Task 3 |
| Failures explain rather than block (§4.5) | Task 3 — `Finding`, never an error |
| A missing birthdate is not an error (§4.5) | Task 3 — `TestValidateHouseholdsStaysQuiet` |
| Enforced consistent output formatting (§1 constraints) | Task 1, Task 2, Task 4's idempotence test |

## Not in this work item

- **Calling `Normalize()` from anywhere.** Nothing in the codebase calls it yet; the editing layer (work item 5) is its first caller. This work item provides the capability.
- **Surfacing Findings in a UI.** Work items 4 and 5 render them; work item 9 shows the export pre-flight warnings.
- **Address parsing.** Addresses stay `[]string`. Splitting them into structured fields is not in the design.
- **Duplicate detection** — two Persons with the same name, two Households at the same address. Plausible future findings, not specified here.
- **`store.Load` or `store.Save` changes.** Explicitly out of scope; the store does not know about phone formats.
