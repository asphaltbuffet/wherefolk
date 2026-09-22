# Data Model & Store Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the recursive `Family` struct with flat Person and Household records carrying stable IDs, persisted as atomically-written JSON with a schema version, and derive the navigation tree at load time.

**Architecture:** Domain types live in `pkg/rolo` (Person, Household, Tree); persistence lives in `internal/store` (atomic write, load, ID generation). Households point *up* to their parent via a `parent` field; the children of a Household are derived by grouping on that field at load time, so a move is a one-field edit that cannot desynchronise. Sibling order is derived from the eldest adult's birth date, never stored.

**Tech Stack:** Go 1.24, `github.com/matoous/go-nanoid/v2` for IDs, `github.com/stretchr/testify` for assertions. No database.

## Global Constraints

- **Terminology** comes from `CONTEXT.md`. Use `Household`, `Dependent`, `Branch`, `Path`, `Memorial Household`. The word `Family` is banned in new code — it meant three different things in the old model.
- **All tests are table-driven**: a `tests []struct{ name string; ... }` slice iterated with `t.Run(tt.name, ...)`. Rows needing assertions beyond field comparison use a `checkFunc func(t *testing.T, ...)` field. This is a project rule from `CLAUDE.md`.
- **Tests live in an external test package** (`package rolo_test`, `package store_test`) and import the package under test, matching `pkg/rolo/family_test.go`.
- **Use `os.ReadFile`/`os.WriteFile`**, never `ioutil`.
- **Death is a property of a Person, never of a Household** (design §3).
- **A Household is never deleted while it has descendants** — it anchors the Path of every Branch beneath it.
- **Module path** is `github.com/asphaltbuffet/wherefolk`.
- **VCS is jujutsu (`jj`), not git.** Commit with `jj commit -m "..."`. Do not run `git` commands.

## File Structure

| File | Responsibility |
|---|---|
| `pkg/rolo/person.go` | The `Person` record: identity, names, dates, contact, per-field hidden flags. |
| `pkg/rolo/household.go` | The `Household` record: adults, dependents, address, anniversary, parent link. |
| `pkg/rolo/tree.go` | `Tree` — the derived navigation structure: children lookup, Path, sibling ordering. |
| `pkg/rolo/date.go` | `Date` — a partial-precision date that tolerates empty and year-only values. |
| `internal/store/id.go` | Stable ID generation (nanoid, Crockford base32, typed prefixes). |
| `internal/store/atomic.go` | Atomic file write: temp → fsync → rename. |
| `internal/store/store.go` | `Document` (the on-disk shape with `schema`), `Load`, `Save`. |
| `pkg/rolo/family.go` | **Deleted** at Task 2. |

**Deleted at Task 2:** `pkg/rolo/family.go`, `pkg/rolo/family_test.go`, `cmd/print.go`, `cmd/print_test.go`, `testdata/example.json`. The recursive `Family`, its rendering methods, and the CLI that drives them are all superseded. They go in Task 2 rather than at the end because `family.go` shares a package with `person.go`: Go compiles a package as a unit, so while both shapes coexist the package cannot build and no test in it can run, whatever `-run` filter is applied. `internal/tui` is left in place but unused; it is dealt with in a later work item.

---

### Task 1: The Date type

Dates in this domain are partial. A living person may have no birth date at all; an ancestor may have only a year. The design's truncation rules (§5.3) need to distinguish "no date" from "1938" from "1938-03-12", so a bare `string` or a `time.Time` both fail — `time.Time` cannot represent "year only" and has no zero-value that means "unknown".

**Files:**
- Create: `pkg/rolo/date.go`
- Test: `pkg/rolo/date_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `type Date struct{ Year, Month, Day int }`
  - `func ParseDate(s string) (Date, error)`
  - `func (d Date) IsZero() bool`
  - `func (d Date) HasYear() bool`
  - `func (d Date) String() string`
  - `func (d Date) MarshalJSON() ([]byte, error)`
  - `func (d *Date) UnmarshalJSON(b []byte) error`

- [ ] **Step 1: Write the failing test**

Create `pkg/rolo/date_test.go`:

```go
package rolo_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    rolo.Date
		wantErr bool
	}{
		{
			name:  "full date",
			input: "1938-03-12",
			want:  rolo.Date{Year: 1938, Month: 3, Day: 12},
		},
		{
			name:  "year and month",
			input: "1938-03",
			want:  rolo.Date{Year: 1938, Month: 3},
		},
		{
			name:  "year only",
			input: "1938",
			want:  rolo.Date{Year: 1938},
		},
		{
			name:  "empty string is the zero date",
			input: "",
			want:  rolo.Date{},
		},
		{
			name:    "not a date",
			input:   "sometime in the fifties",
			wantErr: true,
		},
		{
			name:    "month out of range",
			input:   "1938-13-01",
			wantErr: true,
		},
		{
			name:    "day out of range for month",
			input:   "1938-02-30",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rolo.ParseDate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDateString(t *testing.T) {
	tests := []struct {
		name string
		date rolo.Date
		want string
	}{
		{name: "full date", date: rolo.Date{Year: 1938, Month: 3, Day: 12}, want: "1938-03-12"},
		{name: "year and month", date: rolo.Date{Year: 1938, Month: 3}, want: "1938-03"},
		{name: "year only", date: rolo.Date{Year: 1938}, want: "1938"},
		{name: "zero date is empty", date: rolo.Date{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.date.String())
		})
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		date     rolo.Date
		wantJSON string
	}{
		{name: "full date", date: rolo.Date{Year: 1938, Month: 3, Day: 12}, wantJSON: `"1938-03-12"`},
		{name: "year only", date: rolo.Date{Year: 1938}, wantJSON: `"1938"`},
		{name: "zero date", date: rolo.Date{}, wantJSON: `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.date)
			require.NoError(t, err)
			assert.JSONEq(t, tt.wantJSON, string(b))

			var got rolo.Date
			require.NoError(t, json.Unmarshal(b, &got))
			assert.Equal(t, tt.date, got)
		})
	}
}

func TestDatePredicates(t *testing.T) {
	tests := []struct {
		name        string
		date        rolo.Date
		wantZero    bool
		wantHasYear bool
	}{
		{name: "zero date", date: rolo.Date{}, wantZero: true, wantHasYear: false},
		{name: "year only", date: rolo.Date{Year: 1938}, wantZero: false, wantHasYear: true},
		{name: "full date", date: rolo.Date{Year: 1938, Month: 3, Day: 12}, wantZero: false, wantHasYear: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantZero, tt.date.IsZero())
			assert.Equal(t, tt.wantHasYear, tt.date.HasYear())
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/rolo/ -run 'TestParseDate|TestDateString|TestDateJSON|TestDatePredicates' -v`
Expected: FAIL — `undefined: rolo.Date`, `undefined: rolo.ParseDate`

- [ ] **Step 3: Write the implementation**

Create `pkg/rolo/date.go`:

```go
package rolo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a calendar date of partial precision. A Date may be fully specified
// (1938-03-12), know only a year and month (1938-03), know only a year (1938),
// or be unknown entirely (the zero Date).
//
// Partial precision is a domain requirement, not a convenience: the export
// tiers truncate a living person's date to month and day, and many ancestors
// are recorded with only a year.
type Date struct {
	Year  int
	Month int
	Day   int
}

// ParseDate reads a date in YYYY, YYYY-MM, or YYYY-MM-DD form. The empty
// string yields the zero Date, which represents an unknown date rather than an
// error.
func ParseDate(s string) (Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, nil
	}

	parts := strings.Split(s, "-")
	if len(parts) > 3 {
		return Date{}, fmt.Errorf("parse date %q: too many components", s)
	}

	var d Date
	fields := []*int{&d.Year, &d.Month, &d.Day}
	widths := []int{4, 2, 2}

	for i, p := range parts {
		if len(p) != widths[i] {
			return Date{}, fmt.Errorf("parse date %q: component %q is not %d digits", s, p, widths[i])
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Date{}, fmt.Errorf("parse date %q: %w", s, err)
		}
		*fields[i] = n
	}

	if d.Month != 0 && (d.Month < 1 || d.Month > 12) {
		return Date{}, fmt.Errorf("parse date %q: month out of range", s)
	}

	// Verify the day exists in that month. time.Date normalises overflow
	// (Feb 30 becomes Mar 2), so a round-trip mismatch means the day was invalid.
	if d.Day != 0 {
		t := time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
		if t.Year() != d.Year || int(t.Month()) != d.Month || t.Day() != d.Day {
			return Date{}, fmt.Errorf("parse date %q: day out of range for month", s)
		}
	}

	return d, nil
}

// IsZero reports whether the date is unknown.
func (d Date) IsZero() bool { return d.Year == 0 && d.Month == 0 && d.Day == 0 }

// HasYear reports whether the year is known.
func (d Date) HasYear() bool { return d.Year != 0 }

// String renders the date at whatever precision it holds. The zero Date
// renders as the empty string.
func (d Date) String() string {
	switch {
	case d.IsZero():
		return ""
	case d.Month == 0:
		return fmt.Sprintf("%04d", d.Year)
	case d.Day == 0:
		return fmt.Sprintf("%04d-%02d", d.Year, d.Month)
	default:
		return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	}
}

// MarshalJSON renders the date as a string, so the stored document stays
// readable and hand-editable.
func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON parses a date string.
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/rolo/ -run 'TestParseDate|TestDateString|TestDateJSON|TestDatePredicates' -v`
Expected: PASS — all four test functions, every subtest

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(rolo): add partial-precision Date type"
```

---

### Task 2: The Person record

**Files:**
- Create: `pkg/rolo/person.go` (replaces the existing file entirely)
- Test: `pkg/rolo/person_test.go`

**Interfaces:**
- Consumes: `rolo.Date` (Task 1)
- Produces:
  - `type PersonID string`
  - `type Person struct{ ID PersonID; Given, Surname, BirthName, Aka string; Birth, Death Date; Phone, Email string; Hidden HiddenFields }`
  - `type HiddenFields struct{ Phone, Email, Birth bool }`
  - `func (p Person) IsDeceased() bool`
  - `func (p Person) IsMinor(asOf time.Time) bool`
  - `func (p Person) DisplayName() string`

Note on `IsMinor`: it takes the reference time as a parameter rather than calling `time.Now()` internally. That keeps it testable and makes the design's "exports are not reproducible" property (§5.7) explicit at the call site rather than hidden. A person with no birth date is a minor — fail closed, per §5.7.

- [ ] **Step 1: Write the failing test**

Create `pkg/rolo/person_test.go`:

```go
package rolo_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func TestPersonIsDeceased(t *testing.T) {
	tests := []struct {
		name   string
		person rolo.Person
		want   bool
	}{
		{
			name:   "living person has no death date",
			person: rolo.Person{Given: "Doris", Birth: rolo.Date{Year: 1940}},
			want:   false,
		},
		{
			name:   "deceased person has a death date",
			person: rolo.Person{Given: "Clyde", Birth: rolo.Date{Year: 1938}, Death: rolo.Date{Year: 2019}},
			want:   true,
		},
		{
			name:   "year-only death date still counts",
			person: rolo.Person{Given: "Thomas", Death: rolo.Date{Year: 2022}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.person.IsDeceased())
		})
	}
}

func TestPersonIsMinor(t *testing.T) {
	asOf := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		person rolo.Person
		want   bool
	}{
		{
			name:   "no birth date fails closed and is treated as a minor",
			person: rolo.Person{Given: "Unknown"},
			want:   true,
		},
		{
			name:   "child born 2015 is a minor",
			person: rolo.Person{Given: "Mia", Birth: rolo.Date{Year: 2015, Month: 4, Day: 30}},
			want:   true,
		},
		{
			name:   "adult born 1971 is not a minor",
			person: rolo.Person{Given: "Dave", Birth: rolo.Date{Year: 1971, Month: 3, Day: 2}},
			want:   false,
		},
		{
			name:   "turns 18 tomorrow is still a minor",
			person: rolo.Person{Given: "Ellie", Birth: rolo.Date{Year: 2008, Month: 9, Day: 23}},
			want:   true,
		},
		{
			name:   "turned 18 today is not a minor",
			person: rolo.Person{Given: "Ellie", Birth: rolo.Date{Year: 2008, Month: 9, Day: 22}},
			want:   false,
		},
		{
			name:   "year-only birth date uses January 1",
			person: rolo.Person{Given: "Harold", Birth: rolo.Date{Year: 2008}},
			want:   false,
		},
		{
			name:   "deceased minor is still a minor",
			person: rolo.Person{Given: "Infant", Birth: rolo.Date{Year: 2020}, Death: rolo.Date{Year: 2021}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.person.IsMinor(asOf))
		})
	}
}

func TestPersonDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		person rolo.Person
		want   string
	}{
		{
			name:   "given and surname",
			person: rolo.Person{Given: "Dave", Surname: "Whitlock"},
			want:   "Dave Whitlock",
		},
		{
			name:   "aka renders in double quotes between given and surname",
			person: rolo.Person{Given: "Patricia", Aka: "Pat", Surname: "Novak"},
			want:   `Patricia "Pat" Novak`,
		},
		{
			name:   "given name only",
			person: rolo.Person{Given: "Mia"},
			want:   "Mia",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.person.DisplayName())
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/rolo/ -run TestPerson -v`
Expected: FAIL — the existing `Person` has no `ID`, `Given` is currently the birth name, and there is no `IsDeceased`/`IsMinor`/`DisplayName`

- [ ] **Step 3: Write the implementation**

Replace `pkg/rolo/person.go` entirely:

```go
package rolo

import (
	"strings"
	"time"
)

// PersonID is the stable identity of a Person. It never changes and is never
// shown to the Editor; the human-readable identity of someone in the Directory
// is their Household's Path.
type PersonID string

// HiddenFields records which of a Person's fields the Editor has withheld from
// every export. A withheld field renders as "[private]" so that withheld data
// is distinguishable from data that was never collected.
type HiddenFields struct {
	Phone bool `json:"phone,omitempty"`
	Email bool `json:"email,omitempty"`
	Birth bool `json:"birth,omitempty"`
}

// Person is a single human being in the Directory. Every Person belongs to
// exactly one Household, either as one of its adults or as one of its
// Dependents.
type Person struct {
	ID PersonID `json:"id"`

	Given     string `json:"given"`
	Surname   string `json:"surname"`
	BirthName string `json:"birth_name,omitempty"`
	Aka       string `json:"aka,omitempty"`

	Birth Date `json:"birth"`
	Death Date `json:"death"`

	Phone string `json:"phone,omitempty"`
	Email string `json:"email,omitempty"`

	// omitzero, not omitempty: encoding/json's omitempty has no effect on a
	// struct field, so omitempty would write "hidden": {} for every Person and
	// clutter a document the Operator repairs by hand. omitzero (Go 1.24+)
	// omits the field when the struct is its zero value.
	Hidden HiddenFields `json:"hidden,omitzero"`
}

// IsDeceased reports whether a death date has been recorded. Death is a
// property of a Person, never of a Household.
func (p Person) IsDeceased() bool { return !p.Death.IsZero() }

// IsMinor reports whether the Person is under 18 at asOf.
//
// The reference time is a parameter rather than time.Now() so that the caller
// sees the time dependency: age gating means the same Directory exported
// months apart differs as people turn 18.
//
// A Person with no recorded birth date is treated as a minor. This fails
// closed — it suppresses contact details for elderly relatives whose birth year
// nobody knows, which export surfaces as a warning rather than hiding.
func (p Person) IsMinor(asOf time.Time) bool {
	if p.Birth.IsZero() {
		return true
	}

	// A partial date is read at its earliest possible instant: year-only 2008
	// becomes 2008-01-01, which is the oldest the person could be. That is the
	// generous reading, but it only applies where a year is known, and the
	// fail-closed default already covers the unknown case.
	month := p.Birth.Month
	if month == 0 {
		month = 1
	}
	day := p.Birth.Day
	if day == 0 {
		day = 1
	}

	eighteenth := time.Date(p.Birth.Year+18, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return asOf.Before(eighteenth)
}

// DisplayName renders the Person's name, with any nickname double-quoted
// between the given name and the surname.
func (p Person) DisplayName() string {
	parts := make([]string, 0, 3)
	if p.Given != "" {
		parts = append(parts, p.Given)
	}
	if p.Aka != "" {
		parts = append(parts, `"`+p.Aka+`"`)
	}
	if p.Surname != "" {
		parts = append(parts, p.Surname)
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/rolo/ -run TestPerson -v`
Expected: PASS

**This task also deletes the superseded code that Task 9 originally covered.** `pkg/rolo/family.go` lives in the same package as `person.go` and references the old field names, and Go compiles a package as a unit — so no test in `pkg/rolo` can run, whatever `-run` filter is given, while both shapes coexist. Deleting here is what makes this task verifiable:

```bash
rm -f pkg/rolo/family.go pkg/rolo/family_test.go cmd/print.go cmd/print_test.go testdata/example.json
rm -rf pkg/rolo/testdata
```

Then remove the `rootCmd.AddCommand(GetPrintCmd())` line from `cmd/root.go`.

After this, `go build ./...`, `go vet ./...` and `go test ./...` must all succeed, and they must keep succeeding for every task that follows.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(rolo): replace Person with flat record carrying stable ID"
```

---

### Task 3: The Household record

**Files:**
- Create: `pkg/rolo/household.go`
- Test: `pkg/rolo/household_test.go`

**Interfaces:**
- Consumes: `rolo.Person`, `rolo.PersonID`, `rolo.Date`
- Produces:
  - `type HouseholdID string`
  - `type Address struct{ Lines []string; SharedWith HouseholdID }`
  - `type Household struct{ ID HouseholdID; Parent HouseholdID; Adults []Person; Dependents []Person; Anniversary Date; Address Address }`
  - `func (h Household) IsMemorial() bool`
  - `func (h Household) HasLivingMember() bool`
  - `func (h Household) SharesAddress() bool`
  - `func (h Household) Label() string`
  - `func (h Household) EldestAdultBirth() Date`

`IsMemorial` reports a Memorial Household — one whose *adults* are all deceased (`CONTEXT.md`). `HasLivingMember` is broader and includes Dependents; Proof Sheets use it, because a household of a widow's surviving dependents still has someone to send a sheet to.

- [ ] **Step 1: Write the failing test**

Create `pkg/rolo/household_test.go`:

```go
package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func livingAdult(given string, birthYear int) rolo.Person {
	return rolo.Person{Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
}

func deceasedAdult(given string, birthYear, deathYear int) rolo.Person {
	return rolo.Person{
		Given:   given,
		Surname: "Whitlock",
		Birth:   rolo.Date{Year: birthYear},
		Death:   rolo.Date{Year: deathYear},
	}
}

func TestHouseholdIsMemorial(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      bool
	}{
		{
			name:      "both adults living",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Dave", 1971), livingAdult("Diane", 1973)}},
			want:      false,
		},
		{
			name:      "one adult deceased, one living",
			household: rolo.Household{Adults: []rolo.Person{deceasedAdult("Clyde", 1938, 2019), livingAdult("Doris", 1940)}},
			want:      false,
		},
		{
			name:      "both adults deceased",
			household: rolo.Household{Adults: []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)}},
			want:      true,
		},
		{
			name: "both adults deceased but a dependent survives is still memorial",
			household: rolo.Household{
				Adults:     []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)},
				Dependents: []rolo.Person{livingAdult("Junior", 1950)},
			},
			want: true,
		},
		{
			name:      "no adults at all is not memorial",
			household: rolo.Household{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.IsMemorial())
		})
	}
}

func TestHouseholdHasLivingMember(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      bool
	}{
		{
			name:      "living adult",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Doris", 1940)}},
			want:      true,
		},
		{
			name: "all adults deceased but a dependent lives",
			household: rolo.Household{
				Adults:     []rolo.Person{deceasedAdult("Aden", 1910, 1988)},
				Dependents: []rolo.Person{livingAdult("Junior", 1950)},
			},
			want: true,
		},
		{
			name: "everyone deceased",
			household: rolo.Household{
				Adults:     []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)},
				Dependents: []rolo.Person{deceasedAdult("Thomas", 1996, 2022)},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.HasLivingMember())
		})
	}
}

func TestHouseholdLabel(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      string
	}{
		{
			name:      "two adults join with a slash",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Dave", 1971), livingAdult("Diane", 1973)}},
			want:      "Dave/Diane",
		},
		{
			name:      "single adult is just the given name",
			household: rolo.Household{Adults: []rolo.Person{livingAdult("Doris", 1940)}},
			want:      "Doris",
		},
		{
			name:      "deceased adults still label the household",
			household: rolo.Household{Adults: []rolo.Person{deceasedAdult("Aden", 1910, 1988), deceasedAdult("Nettie", 1912, 1995)}},
			want:      "Aden/Nettie",
		},
		{
			name:      "no adults falls back to a placeholder",
			household: rolo.Household{ID: "h_3xqp1w"},
			want:      "(h_3xqp1w)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.Label())
		})
	}
}

func TestHouseholdSharesAddress(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      bool
	}{
		{
			name:      "own address",
			household: rolo.Household{Address: rolo.Address{Lines: []string{"1412 Oak St"}}},
			want:      false,
		},
		{
			name:      "shared with parent",
			household: rolo.Household{Address: rolo.Address{SharedWith: "h_9m2kfp"}},
			want:      true,
		},
		{
			name:      "no address at all",
			household: rolo.Household{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.SharesAddress())
		})
	}
}

func TestHouseholdEldestAdultBirth(t *testing.T) {
	tests := []struct {
		name      string
		household rolo.Household
		want      rolo.Date
	}{
		{
			name: "earlier of two birth dates",
			household: rolo.Household{Adults: []rolo.Person{
				{Given: "Diane", Birth: rolo.Date{Year: 1973, Month: 8, Day: 19}},
				{Given: "Dave", Birth: rolo.Date{Year: 1971, Month: 3, Day: 2}},
			}},
			want: rolo.Date{Year: 1971, Month: 3, Day: 2},
		},
		{
			name: "ignores adults with no birth date",
			household: rolo.Household{Adults: []rolo.Person{
				{Given: "Unknown"},
				{Given: "Dave", Birth: rolo.Date{Year: 1971}},
			}},
			want: rolo.Date{Year: 1971},
		},
		{
			name:      "no adults yields the zero date",
			household: rolo.Household{},
			want:      rolo.Date{},
		},
		{
			name: "no adult has a birth date yields the zero date",
			household: rolo.Household{Adults: []rolo.Person{{Given: "Unknown"}}},
			want: rolo.Date{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.household.EldestAdultBirth())
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/rolo/ -run TestHousehold -v`
Expected: FAIL — `undefined: rolo.Household`, `undefined: rolo.Address`

- [ ] **Step 3: Write the implementation**

Create `pkg/rolo/household.go`:

```go
package rolo

import "strings"

// HouseholdID is the stable identity of a Household. It survives restructuring
// of the tree, unlike the Path, which is a display identity.
type HouseholdID string

// Address is a Household's mailing address, either as its own lines or as a
// reference to another Household's address.
//
// A Shared Address is a reference rather than a copy so that it stays correct
// when the referenced Household moves, and so the Directory can render a
// back-reference instead of repeating the same lines.
type Address struct {
	Lines      []string    `json:"lines,omitempty"`
	SharedWith HouseholdID `json:"shared_with,omitempty"`
}

// Household is the unit that renders as one block in the Directory. A node
// becomes a Household when it has a spouse, has Dependents, or has its own
// Address.
//
// Parent is empty for a root Household. Children are not stored: they are
// derived by grouping on Parent when the Tree is built, so moving a Household
// is a one-field edit that cannot leave two records disagreeing.
type Household struct {
	ID     HouseholdID `json:"id"`
	Parent HouseholdID `json:"parent,omitempty"`

	Adults     []Person `json:"adults"`
	Dependents []Person `json:"dependents,omitempty"`

	Anniversary Date `json:"anniversary"`

	// omitzero for the same reason as Person.Hidden: Address is a struct, and
	// omitempty does not suppress an empty one.
	Address Address `json:"address,omitzero"`
}

// IsMemorial reports whether every adult in the Household is deceased. A
// Memorial Household persists permanently — it anchors the Path of every Branch
// beneath it — and carries no contact details, because it has no living adult
// to own them.
//
// A Household with no adults is not memorial; it is malformed, and the store
// rejects it at load.
func (h Household) IsMemorial() bool {
	if len(h.Adults) == 0 {
		return false
	}
	for _, a := range h.Adults {
		if !a.IsDeceased() {
			return false
		}
	}
	return true
}

// HasLivingMember reports whether anyone in the Household is living, including
// Dependents. Proof Sheets are produced only for Households where this holds.
func (h Household) HasLivingMember() bool {
	for _, p := range h.Adults {
		if !p.IsDeceased() {
			return true
		}
	}
	for _, p := range h.Dependents {
		if !p.IsDeceased() {
			return true
		}
	}
	return false
}

// SharesAddress reports whether this Household's address is a reference to
// another Household's.
func (h Household) SharesAddress() bool { return h.Address.SharedWith != "" }

// Label is the Household's name as it appears in the tree and in a Path:
// the adults' given names joined by a slash, as in "Dave/Diane".
func (h Household) Label() string {
	if len(h.Adults) == 0 {
		return "(" + string(h.ID) + ")"
	}

	names := make([]string, 0, len(h.Adults))
	for _, a := range h.Adults {
		names = append(names, a.Given)
	}
	return strings.Join(names, "/")
}

// EldestAdultBirth returns the earliest known birth date among the adults, or
// the zero Date when none is known. Sibling Households are ordered by this,
// which matches how families list their children.
func (h Household) EldestAdultBirth() Date {
	var eldest Date
	for _, a := range h.Adults {
		if a.Birth.IsZero() {
			continue
		}
		if eldest.IsZero() || dateLess(a.Birth, eldest) {
			eldest = a.Birth
		}
	}
	return eldest
}

// dateLess reports whether a falls before b, comparing at whatever precision
// each date holds. A missing month or day sorts as if it were the first of the
// period.
func dateLess(a, b Date) bool {
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	if a.Month != b.Month {
		return a.Month < b.Month
	}
	return a.Day < b.Day
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/rolo/ -run TestHousehold -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(rolo): add flat Household record with parent link"
```

---

### Task 4: ID generation

**Files:**
- Create: `internal/store/id.go`
- Test: `internal/store/id_test.go`
- Modify: `go.mod` (adds `github.com/matoous/go-nanoid/v2`)

**Interfaces:**
- Consumes: `rolo.PersonID`, `rolo.HouseholdID`
- Produces:
  - `func NewPersonID() (rolo.PersonID, error)`
  - `func NewHouseholdID() (rolo.HouseholdID, error)`
  - `const IDAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"`
  - `const IDSize = 6`

The alphabet is Crockford base32 — lowercase, omitting `i`, `l`, `o`, and `u`. Those are the characters people confuse when reading an ID aloud or retyping it during a hand repair over SSH, and omitting `u` also prevents the generator from producing accidental profanity. The `p_`/`h_` prefixes make a mistyped or cross-wired reference obvious on sight.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/matoous/go-nanoid/v2@v2.1.0
```

Expected: `go: added github.com/matoous/go-nanoid/v2 v2.1.0`

- [ ] **Step 2: Write the failing test**

Create `internal/store/id_test.go`:

```go
package store_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
)

func TestNewIDs(t *testing.T) {
	tests := []struct {
		name       string
		generate   func() (string, error)
		wantPrefix string
	}{
		{
			name: "person IDs carry the p_ prefix",
			generate: func() (string, error) {
				id, err := store.NewPersonID()
				return string(id), err
			},
			wantPrefix: "p_",
		},
		{
			name: "household IDs carry the h_ prefix",
			generate: func() (string, error) {
				id, err := store.NewHouseholdID()
				return string(id), err
			},
			wantPrefix: "h_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.generate()
			require.NoError(t, err)

			assert.True(t, strings.HasPrefix(id, tt.wantPrefix), "id %q lacks prefix %q", id, tt.wantPrefix)
			assert.Len(t, id, len(tt.wantPrefix)+store.IDSize)

			body := strings.TrimPrefix(id, tt.wantPrefix)
			for _, r := range body {
				assert.True(t, strings.ContainsRune(store.IDAlphabet, r),
					"id %q contains %q, which is outside the alphabet", id, r)
			}
		})
	}
}

func TestNewIDsAreUnique(t *testing.T) {
	const n = 1000

	seen := make(map[string]struct{}, n)
	for range n {
		id, err := store.NewHouseholdID()
		require.NoError(t, err)

		_, dup := seen[string(id)]
		require.False(t, dup, "generated duplicate id %q", id)
		seen[string(id)] = struct{}{}
	}
}

func TestIDAlphabetExcludesConfusableCharacters(t *testing.T) {
	tests := []struct {
		name string
		char rune
	}{
		{name: "i is confusable with 1", char: 'i'},
		{name: "l is confusable with 1", char: 'l'},
		{name: "o is confusable with 0", char: 'o'},
		{name: "u is excluded by Crockford base32", char: 'u'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotContains(t, store.IDAlphabet, string(tt.char))
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/store/ -v`
Expected: FAIL — no such package `internal/store`

- [ ] **Step 4: Write the implementation**

Create `internal/store/id.go`:

```go
// Package store persists the Directory as a single atomically-written JSON
// document and generates the stable identities its records carry.
package store

import (
	"fmt"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

const (
	// IDAlphabet is Crockford base32 in lowercase: the digits and the letters,
	// less i, l, o and u. Those are the characters people confuse when reading
	// an ID aloud or retyping one while repairing the document by hand.
	IDAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

	// IDSize is the number of random characters after the prefix. 32^6 is
	// about a billion values, which is ample for a directory of a few hundred
	// people; the store still rejects a collision at load rather than assuming.
	IDSize = 6

	personPrefix    = "p_"
	householdPrefix = "h_"
)

// NewPersonID generates a stable identity for a Person.
func NewPersonID() (rolo.PersonID, error) {
	body, err := gonanoid.Generate(IDAlphabet, IDSize)
	if err != nil {
		return "", fmt.Errorf("generate person id: %w", err)
	}
	return rolo.PersonID(personPrefix + body), nil
}

// NewHouseholdID generates a stable identity for a Household.
func NewHouseholdID() (rolo.HouseholdID, error) {
	body, err := gonanoid.Generate(IDAlphabet, IDSize)
	if err != nil {
		return "", fmt.Errorf("generate household id: %w", err)
	}
	return rolo.HouseholdID(householdPrefix + body), nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS — all three test functions

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(store): add prefixed nanoid identity generation"
```

---

### Task 5: The Tree

This is where the flat records become navigable. The Tree is derived, never stored: it is rebuilt from the `Parent` fields every time the document loads.

**Files:**
- Create: `pkg/rolo/tree.go`
- Test: `pkg/rolo/tree_test.go`

**Interfaces:**
- Consumes: `rolo.Household`, `rolo.HouseholdID`
- Produces:
  - `type Tree struct{ ... }` (unexported fields)
  - `func BuildTree(households []Household) (*Tree, error)`
  - `func (t *Tree) Roots() []Household`
  - `func (t *Tree) Children(id HouseholdID) []Household`
  - `func (t *Tree) Get(id HouseholdID) (Household, bool)`
  - `func (t *Tree) Path(id HouseholdID) ([]Household, error)`
  - `func (t *Tree) PathString(id HouseholdID) (string, error)`
  - `func (t *Tree) Walk(fn func(h Household, depth int) error) error`
  - `var ErrUnknownParent, ErrDuplicateID, ErrCycle, ErrNoAdults error`

`Walk` visits in depth-first order with siblings ordered by eldest adult's birth date — the order the rendered Directory uses.

- [ ] **Step 1: Write the failing test**

Create `pkg/rolo/tree_test.go`:

```go
package rolo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// sampleHouseholds returns a three-generation tree:
//
//	Aden/Nettie
//	├── Clyde/Doris   (b. 1938)
//	│   └── Dave/Diane
//	├── Harold/June   (b. 1941)
//	└── Susan/Ray     (b. 1944)
//
// Deliberately supplied out of order, to prove ordering is derived.
func sampleHouseholds() []rolo.Household {
	adult := func(given string, birthYear int) rolo.Person {
		return rolo.Person{Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
	}

	return []rolo.Household{
		{ID: "h_susan", Parent: "h_aden", Adults: []rolo.Person{adult("Susan", 1944), adult("Ray", 1943)}},
		{ID: "h_dave", Parent: "h_clyde", Adults: []rolo.Person{adult("Dave", 1971), adult("Diane", 1973)}},
		{ID: "h_aden", Adults: []rolo.Person{adult("Aden", 1910), adult("Nettie", 1912)}},
		{ID: "h_harold", Parent: "h_aden", Adults: []rolo.Person{adult("Harold", 1941), adult("June", 1942)}},
		{ID: "h_clyde", Parent: "h_aden", Adults: []rolo.Person{adult("Clyde", 1938), adult("Doris", 1940)}},
	}
}

func TestBuildTreeStructure(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, tree *rolo.Tree)
	}{
		{
			name: "one root",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				roots := tree.Roots()
				require.Len(t, roots, 1)
				assert.Equal(t, rolo.HouseholdID("h_aden"), roots[0].ID)
			},
		},
		{
			name: "children are ordered by eldest adult's birth date",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				kids := tree.Children("h_aden")
				require.Len(t, kids, 3)
				assert.Equal(t, "Clyde/Doris", kids[0].Label())
				assert.Equal(t, "Harold/June", kids[1].Label())
				assert.Equal(t, "Susan/Ray", kids[2].Label())
			},
		},
		{
			name: "a leaf has no children",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				assert.Empty(t, tree.Children("h_dave"))
			},
		},
		{
			name: "get returns a known household",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_dave")
				require.True(t, ok)
				assert.Equal(t, "Dave/Diane", h.Label())
			},
		},
		{
			name: "get reports an unknown household",
			checkFunc: func(t *testing.T, tree *rolo.Tree) {
				t.Helper()
				_, ok := tree.Get("h_nobody")
				assert.False(t, ok)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := rolo.BuildTree(sampleHouseholds())
			require.NoError(t, err)
			tt.checkFunc(t, tree)
		})
	}
}

func TestTreePath(t *testing.T) {
	tests := []struct {
		name    string
		id      rolo.HouseholdID
		want    string
		wantErr bool
	}{
		{
			name: "three generations deep",
			id:   "h_dave",
			want: "Aden/Nettie › Clyde/Doris › Dave/Diane",
		},
		{
			name: "one generation deep",
			id:   "h_clyde",
			want: "Aden/Nettie › Clyde/Doris",
		},
		{
			name: "the root is its own path",
			id:   "h_aden",
			want: "Aden/Nettie",
		},
		{
			name:    "unknown household",
			id:      "h_nobody",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := rolo.BuildTree(sampleHouseholds())
			require.NoError(t, err)

			got, err := tree.PathString(tt.id)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTreeWalkOrder(t *testing.T) {
	tree, err := rolo.BuildTree(sampleHouseholds())
	require.NoError(t, err)

	type visit struct {
		label string
		depth int
	}

	var got []visit
	require.NoError(t, tree.Walk(func(h rolo.Household, depth int) error {
		got = append(got, visit{label: h.Label(), depth: depth})
		return nil
	}))

	want := []visit{
		{label: "Aden/Nettie", depth: 0},
		{label: "Clyde/Doris", depth: 1},
		{label: "Dave/Diane", depth: 2},
		{label: "Harold/June", depth: 1},
		{label: "Susan/Ray", depth: 1},
	}
	assert.Equal(t, want, got)
}

func TestBuildTreeRejectsMalformedData(t *testing.T) {
	adult := func(given string) rolo.Person {
		return rolo.Person{Given: given, Birth: rolo.Date{Year: 1950}}
	}

	tests := []struct {
		name       string
		households []rolo.Household
		wantErr    error
	}{
		{
			name: "parent that does not exist",
			households: []rolo.Household{
				{ID: "h_dave", Parent: "h_ghost", Adults: []rolo.Person{adult("Dave")}},
			},
			wantErr: rolo.ErrUnknownParent,
		},
		{
			name: "duplicate household id",
			households: []rolo.Household{
				{ID: "h_dave", Adults: []rolo.Person{adult("Dave")}},
				{ID: "h_dave", Adults: []rolo.Person{adult("David")}},
			},
			wantErr: rolo.ErrDuplicateID,
		},
		{
			name: "household is its own parent",
			households: []rolo.Household{
				{ID: "h_dave", Parent: "h_dave", Adults: []rolo.Person{adult("Dave")}},
			},
			wantErr: rolo.ErrCycle,
		},
		{
			name: "two households parent each other",
			households: []rolo.Household{
				{ID: "h_a", Parent: "h_b", Adults: []rolo.Person{adult("A")}},
				{ID: "h_b", Parent: "h_a", Adults: []rolo.Person{adult("B")}},
			},
			wantErr: rolo.ErrCycle,
		},
		{
			name: "household with no adults",
			households: []rolo.Household{
				{ID: "h_empty"},
			},
			wantErr: rolo.ErrNoAdults,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rolo.BuildTree(tt.households)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestBuildTreeAcceptsMultipleRoots(t *testing.T) {
	adult := func(given string, birthYear int) rolo.Person {
		return rolo.Person{Given: given, Birth: rolo.Date{Year: birthYear}}
	}

	households := []rolo.Household{
		{ID: "h_novak", Adults: []rolo.Person{adult("Patricia", 1952)}},
		{ID: "h_aden", Adults: []rolo.Person{adult("Aden", 1910)}},
	}

	tree, err := rolo.BuildTree(households)
	require.NoError(t, err)

	roots := tree.Roots()
	require.Len(t, roots, 2)
	assert.Equal(t, rolo.HouseholdID("h_aden"), roots[0].ID, "roots sort by eldest adult's birth date")
	assert.Equal(t, rolo.HouseholdID("h_novak"), roots[1].ID)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/rolo/ -run 'TestBuildTree|TestTree' -v`
Expected: FAIL — `undefined: rolo.BuildTree`, `undefined: rolo.Tree`

- [ ] **Step 3: Write the implementation**

Create `pkg/rolo/tree.go`:

```go
package rolo

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// PathSeparator joins Household labels in a rendered Path.
const PathSeparator = " › "

var (
	// ErrUnknownParent means a Household names a parent that is not in the document.
	ErrUnknownParent = errors.New("unknown parent household")
	// ErrDuplicateID means two Households share an ID.
	ErrDuplicateID = errors.New("duplicate household id")
	// ErrCycle means the parent links form a loop, so a Household is its own ancestor.
	ErrCycle = errors.New("cycle in household parentage")
	// ErrNoAdults means a Household has no adults, which cannot be labelled or rendered.
	ErrNoAdults = errors.New("household has no adults")
	// ErrUnknownHousehold means a lookup named a Household that is not in the tree.
	ErrUnknownHousehold = errors.New("unknown household")
)

// Tree is the navigation structure derived from Households' parent links. It
// is never stored: it is rebuilt whenever the document is loaded, so the parent
// links are the single source of truth and no stored child list can drift.
type Tree struct {
	byID     map[HouseholdID]Household
	children map[HouseholdID][]HouseholdID
	roots    []HouseholdID
}

// BuildTree derives the navigation tree from a flat slice of Households,
// validating that the parent links form a forest.
//
// Sibling order is derived from each Household's eldest adult's birth date,
// which is how families list their children. Households whose eldest adult has
// no birth date sort last, by label.
func BuildTree(households []Household) (*Tree, error) {
	t := &Tree{
		byID:     make(map[HouseholdID]Household, len(households)),
		children: make(map[HouseholdID][]HouseholdID),
	}

	for _, h := range households {
		if _, dup := t.byID[h.ID]; dup {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateID, h.ID)
		}
		if len(h.Adults) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrNoAdults, h.ID)
		}
		t.byID[h.ID] = h
	}

	for _, h := range households {
		if h.Parent == "" {
			t.roots = append(t.roots, h.ID)
			continue
		}
		if h.Parent == h.ID {
			return nil, fmt.Errorf("%w: %s is its own parent", ErrCycle, h.ID)
		}
		if _, ok := t.byID[h.Parent]; !ok {
			return nil, fmt.Errorf("%w: %s names parent %s", ErrUnknownParent, h.ID, h.Parent)
		}
		t.children[h.Parent] = append(t.children[h.Parent], h.ID)
	}

	// Every Household must reach a root by following parent links. A node that
	// does not is part of a cycle: it has a valid parent, but that chain loops
	// rather than terminating, so it never appears under any root.
	if err := t.detectCycles(); err != nil {
		return nil, err
	}

	t.sortSiblings()

	return t, nil
}

// detectCycles walks upward from every Household, bounding each walk by the
// number of Households. A chain longer than that must have revisited a node.
func (t *Tree) detectCycles() error {
	for id := range t.byID {
		current := id
		for steps := 0; ; steps++ {
			h := t.byID[current]
			if h.Parent == "" {
				break
			}
			if steps > len(t.byID) {
				return fmt.Errorf("%w: %s is its own ancestor", ErrCycle, id)
			}
			current = h.Parent
		}
	}
	return nil
}

// sortSiblings orders the roots and every child list by eldest adult's birth
// date, falling back to label for Households with no known birth date.
func (t *Tree) sortSiblings() {
	sortIDs := func(ids []HouseholdID) {
		sort.SliceStable(ids, func(i, j int) bool {
			a, b := t.byID[ids[i]], t.byID[ids[j]]
			ab, bb := a.EldestAdultBirth(), b.EldestAdultBirth()

			switch {
			case ab.IsZero() && bb.IsZero():
				return a.Label() < b.Label()
			case ab.IsZero():
				return false // unknown birth dates sort last
			case bb.IsZero():
				return true
			case ab != bb:
				return dateLess(ab, bb)
			default:
				return a.Label() < b.Label()
			}
		})
	}

	sortIDs(t.roots)
	for parent := range t.children {
		sortIDs(t.children[parent])
	}
}

// Get returns a Household by ID.
func (t *Tree) Get(id HouseholdID) (Household, bool) {
	h, ok := t.byID[id]
	return h, ok
}

// Roots returns the Households with no parent, in sibling order.
func (t *Tree) Roots() []Household { return t.lookup(t.roots) }

// Children returns a Household's children, in sibling order.
func (t *Tree) Children(id HouseholdID) []Household { return t.lookup(t.children[id]) }

func (t *Tree) lookup(ids []HouseholdID) []Household {
	out := make([]Household, 0, len(ids))
	for _, id := range ids {
		out = append(out, t.byID[id])
	}
	return out
}

// Path returns the chain of Households from the root down to id, inclusive.
func (t *Tree) Path(id HouseholdID) ([]Household, error) {
	if _, ok := t.byID[id]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownHousehold, id)
	}

	var chain []Household
	for current := id; current != ""; {
		h := t.byID[current]
		chain = append(chain, h)
		current = h.Parent
	}

	// Reverse: the walk collected leaf-to-root.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	return chain, nil
}

// PathString renders a Household's Path as "Aden/Nettie › Clyde/Doris ›
// Dave/Diane". This is the human-readable identity of a Household, and is what
// disambiguates relatives who share a given name.
func (t *Tree) PathString(id HouseholdID) (string, error) {
	chain, err := t.Path(id)
	if err != nil {
		return "", err
	}

	labels := make([]string, 0, len(chain))
	for _, h := range chain {
		labels = append(labels, h.Label())
	}
	return strings.Join(labels, PathSeparator), nil
}

// Walk visits every Household depth-first in sibling order, which is the order
// the rendered Directory uses. Returning an error from fn stops the walk and
// returns that error.
func (t *Tree) Walk(fn func(h Household, depth int) error) error {
	var visit func(id HouseholdID, depth int) error
	visit = func(id HouseholdID, depth int) error {
		if err := fn(t.byID[id], depth); err != nil {
			return err
		}
		for _, child := range t.children[id] {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}

	for _, root := range t.roots {
		if err := visit(root, 0); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/rolo/ -run 'TestBuildTree|TestTree' -v`
Expected: PASS — all five test functions

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(rolo): derive navigation tree from parent links"
```

---

### Task 6: Atomic file write

**Files:**
- Create: `internal/store/atomic.go`
- Test: `internal/store/atomic_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `func writeFileAtomic(path string, data []byte, perm os.FileMode) error` (unexported — used by `Save` in Task 7)

Because `writeFileAtomic` is unexported, its test lives in `package store` (internal), unlike every other test in this plan. That is deliberate: the function is an implementation detail of `Save`, but the temp-file-and-rename behaviour is exactly what needs direct testing.

- [ ] **Step 1: Write the failing test**

Create `internal/store/atomic_test.go`:

```go
package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic(t *testing.T) {
	tests := []struct {
		name      string
		existing  string
		write     string
		checkFunc func(t *testing.T, path string)
	}{
		{
			name:  "creates a new file",
			write: `{"schema":1}`,
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				b, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, `{"schema":1}`, string(b))
			},
		},
		{
			name:     "replaces an existing file",
			existing: `{"schema":1,"old":true}`,
			write:    `{"schema":2}`,
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				b, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, `{"schema":2}`, string(b))
			},
		},
		{
			name:     "shrinking the file leaves no trailing bytes",
			existing: strings.Repeat("x", 4096),
			write:    "small",
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				b, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, "small", string(b))
			},
		},
		{
			name:  "leaves no temp files behind",
			write: `{"schema":1}`,
			checkFunc: func(t *testing.T, path string) {
				t.Helper()
				entries, err := os.ReadDir(filepath.Dir(path))
				require.NoError(t, err)
				assert.Len(t, entries, 1, "expected only the target file to remain")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "directory.json")

			if tt.existing != "" {
				require.NoError(t, os.WriteFile(path, []byte(tt.existing), 0o600))
			}

			require.NoError(t, writeFileAtomic(path, []byte(tt.write), 0o600))
			tt.checkFunc(t, path)
		})
	}
}

func TestWriteFileAtomicSetsPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directory.json")

	require.NoError(t, writeFileAtomic(path, []byte("data"), 0o600))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"the document holds relatives' addresses and must not be world-readable")
}

func TestWriteFileAtomicFailsOnMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "directory.json")

	err := writeFileAtomic(path, []byte("data"), 0o600)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestWriteFileAtomic -v`
Expected: FAIL — `undefined: writeFileAtomic`

- [ ] **Step 3: Write the implementation**

Create `internal/store/atomic.go`:

```go
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path so that a reader sees either the previous
// contents or the complete new contents, never a partial write.
//
// The document is the only copy of the Directory, and the Editor's machine may
// lose power mid-save. Writing in place would risk leaving a truncated JSON
// file, so the new contents go to a temp file in the same directory, are
// flushed to disk, and are then renamed over the target — rename is atomic
// within a filesystem.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Remove the temp file on any failure after this point. Once the rename
	// succeeds the temp name no longer exists and the remove is a harmless
	// no-op.
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Flush to the platter before renaming. Without this the rename can land
	// while the contents are still in the page cache, so a crash leaves the
	// target pointing at an empty or partial file.
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestWriteFileAtomic -v`
Expected: PASS — all three test functions

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(store): add atomic file write"
```

---

### Task 7: The Document and the store

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `rolo.Household`, `rolo.BuildTree`, `writeFileAtomic` (Task 6)
- Produces:
  - `const CurrentSchema = 1`
  - `type Document struct{ Schema int; Households []rolo.Household }`
  - `func Load(path string) (*Document, error)`
  - `func Save(path string, doc *Document) error`
  - `func (d *Document) Tree() (*rolo.Tree, error)`
  - `var ErrSchemaTooNew, ErrSchemaMissing error`

`Load` refuses a document whose schema is newer than `CurrentSchema` — the rollback case from design §2.1, where silently dropping unknown fields would corrupt the Directory. It also validates the tree, so a document with a dangling parent fails at load rather than at render.

- [ ] **Step 1: Write the failing test**

Create `internal/store/store_test.go`:

```go
package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

func sampleDocument() *store.Document {
	adult := func(id rolo.PersonID, given string, birthYear int) rolo.Person {
		return rolo.Person{ID: id, Given: given, Surname: "Whitlock", Birth: rolo.Date{Year: birthYear}}
	}

	return &store.Document{
		Schema: store.CurrentSchema,
		Households: []rolo.Household{
			{
				ID:     "h_aden",
				Adults: []rolo.Person{adult("p_aden01", "Aden", 1910), adult("p_nett01", "Nettie", 1912)},
			},
			{
				ID:          "h_clyde",
				Parent:      "h_aden",
				Adults:      []rolo.Person{adult("p_clyd01", "Clyde", 1938), adult("p_dori01", "Doris", 1940)},
				Anniversary: rolo.Date{Year: 1961, Month: 6, Day: 14},
				Address:     rolo.Address{Lines: []string{"88 Oakwood Drive", "Shelbyville, IL 62565"}},
			},
		},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, got *store.Document)
	}{
		{
			name: "schema is preserved",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				assert.Equal(t, store.CurrentSchema, got.Schema)
			},
		},
		{
			name: "households survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				require.Len(t, got.Households, 2)
			},
		},
		{
			name: "dates survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				h, ok := tree.Get("h_clyde")
				require.True(t, ok)
				assert.Equal(t, rolo.Date{Year: 1961, Month: 6, Day: 14}, h.Anniversary)
				assert.Equal(t, rolo.Date{Year: 1938}, h.Adults[0].Birth)
			},
		},
		{
			name: "addresses survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				h, ok := tree.Get("h_clyde")
				require.True(t, ok)
				assert.Equal(t, []string{"88 Oakwood Drive", "Shelbyville, IL 62565"}, h.Address.Lines)
			},
		},
		{
			name: "parent links survive the round trip",
			checkFunc: func(t *testing.T, got *store.Document) {
				t.Helper()
				tree, err := got.Tree()
				require.NoError(t, err)

				path, err := tree.PathString("h_clyde")
				require.NoError(t, err)
				assert.Equal(t, "Aden/Nettie › Clyde/Doris", path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "directory.json")
			require.NoError(t, store.Save(path, sampleDocument()))

			got, err := store.Load(path)
			require.NoError(t, err)
			tt.checkFunc(t, got)
		})
	}
}

func TestLoadRejectsBadDocuments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{
			name:    "schema newer than this binary understands",
			content: `{"schema": 99, "households": []}`,
			wantErr: store.ErrSchemaTooNew,
		},
		{
			name:    "no schema field at all",
			content: `{"households": []}`,
			wantErr: store.ErrSchemaMissing,
		},
		{
			name:    "household names a parent that does not exist",
			content: `{"schema": 1, "households": [{"id": "h_a", "parent": "h_ghost", "adults": [{"id": "p_a", "given": "A", "birth": "1950"}]}]}`,
			wantErr: rolo.ErrUnknownParent,
		},
		{
			name:    "duplicate household ids",
			content: `{"schema": 1, "households": [{"id": "h_a", "adults": [{"id": "p_a", "given": "A", "birth": "1950"}]}, {"id": "h_a", "adults": [{"id": "p_b", "given": "B", "birth": "1950"}]}]}`,
			wantErr: rolo.ErrDuplicateID,
		},
		{
			name:    "household with no adults",
			content: `{"schema": 1, "households": [{"id": "h_a", "adults": []}]}`,
			wantErr: rolo.ErrNoAdults,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "directory.json")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			_, err := store.Load(path)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := store.Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestLoadMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"schema": 1, "households": [`), 0o600))

	_, err := store.Load(path)
	require.Error(t, err)
}

func TestSaveIsReadableAndIndented(t *testing.T) {
	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, sampleDocument()))

	b, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Contains(t, string(b), "\n  ", "the document is hand-edited during repair and must be indented")
	assert.Contains(t, string(b), `"schema": 1`)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'TestSaveLoad|TestLoad|TestSave' -v`
Expected: FAIL — `undefined: store.Document`, `undefined: store.Load`

- [ ] **Step 3: Write the implementation**

Create `internal/store/store.go`:

```go
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

// CurrentSchema is the document version this binary writes and is the highest
// it can read.
const CurrentSchema = 1

var (
	// ErrSchemaTooNew means the document was written by a newer binary. This is
	// the rollback case: reading it would silently drop fields this binary does
	// not know about, so the store refuses rather than corrupting the Directory.
	ErrSchemaTooNew = errors.New("document schema is newer than this binary understands")

	// ErrSchemaMissing means the document has no schema field, so its version
	// cannot be established.
	ErrSchemaMissing = errors.New("document has no schema version")
)

// documentPerm keeps the document readable only by its owner. It holds
// relatives' home addresses, birth dates and phone numbers.
const documentPerm os.FileMode = 0o600

// Document is the on-disk shape of the Directory: a schema version and a flat
// slice of Households. The navigation tree is not stored; it is derived from
// the Households' parent links by Tree.
type Document struct {
	Schema     int              `json:"schema"`
	Households []rolo.Household `json:"households"`
}

// Tree derives the navigation tree, validating the parent links.
func (d *Document) Tree() (*rolo.Tree, error) { return rolo.BuildTree(d.Households) }

// Load reads and validates the document at path.
//
// Validation happens here rather than at render time so that a structural
// problem surfaces at startup, where the Operator sees it, rather than when the
// Editor clicks Export.
func Load(path string) (*Document, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read document: %w", err)
	}

	// Decode the schema first: a document from a newer binary may contain
	// shapes this one cannot parse, so its version must be checked before any
	// attempt to read the rest.
	var probe struct {
		Schema *int `json:"schema"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, fmt.Errorf("read document schema: %w", err)
	}
	if probe.Schema == nil {
		return nil, fmt.Errorf("%w: %s", ErrSchemaMissing, path)
	}
	if *probe.Schema > CurrentSchema {
		return nil, fmt.Errorf("%w: document is version %d, this binary reads up to %d",
			ErrSchemaTooNew, *probe.Schema, CurrentSchema)
	}

	var doc Document
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse document: %w", err)
	}

	if _, err := doc.Tree(); err != nil {
		return nil, fmt.Errorf("validate document: %w", err)
	}

	return &doc, nil
}

// Save writes the document to path atomically.
//
// The output is indented because the Operator repairs this file by hand over
// SSH; a single-line document would make that impractical.
func Save(path string, doc *Document) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode document: %w", err)
	}
	b = append(b, '\n')

	if err := writeFileAtomic(path, b, documentPerm); err != nil {
		return fmt.Errorf("save document: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS — every test function in the package

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(store): add versioned document with atomic save and validating load"
```

---

### Task 8: Round-trip the real dataset

Everything so far has been tested against hand-built fixtures. This task proves the model holds against `testdata/example.json`'s actual shape — two root Households, a three-generation branch, a deceased dependent, a nickname, and multiple addresses — converted by hand to the new format.

This hand-conversion is also the specification for the automated migration in work item 2.

**Files:**
- Create: `testdata/directory.json`
- Test: `internal/store/golden_test.go`

**Interfaces:**
- Consumes: `store.Load`, `rolo.Tree`
- Produces: `testdata/directory.json` — the canonical example document in the new format

- [ ] **Step 1: Write the new-format test data**

Create `testdata/directory.json`. This is `testdata/example.json` converted by hand: Robert/Susan and their branch, plus Patricia Novak. Note that Mia and Emma are Dependents of their respective Households rather than Households of their own, Thomas is a deceased Dependent, and Daniel/Claire is a Household because it has both a spouse and a Dependent.

```json
{
  "schema": 1,
  "households": [
    {
      "id": "h_lang01",
      "adults": [
        {
          "id": "p_rob001",
          "given": "Robert",
          "surname": "Langford",
          "birth": "1965-03-12",
          "death": "",
          "phone": "555-201-0001",
          "email": "robert.langford@example.com"
        },
        {
          "id": "p_sus001",
          "given": "Susan",
          "surname": "Langford",
          "birth_name": "Marsh",
          "birth": "1967-08-24",
          "death": "",
          "phone": "555-201-0002",
          "email": "susan.langford@example.com"
        }
      ],
      "dependents": [
        {
          "id": "p_tho001",
          "given": "Thomas",
          "surname": "Langford",
          "birth": "1996-07-19",
          "death": "2022-01-08"
        },
        {
          "id": "p_emm001",
          "given": "Emma",
          "surname": "Langford",
          "birth": "1999-12-25",
          "death": "",
          "phone": "555-201-0020",
          "email": "emma.langford@example.com"
        }
      ],
      "anniversary": "1991-06-15",
      "address": {
        "lines": [
          "42 Elm Street",
          "Springfield, IL 62701"
        ]
      }
    },
    {
      "id": "h_lang02",
      "parent": "h_lang01",
      "adults": [
        {
          "id": "p_dan001",
          "given": "Daniel",
          "surname": "Langford",
          "birth": "1993-11-03",
          "death": "",
          "phone": "555-201-0010",
          "email": "dan.langford@example.com"
        },
        {
          "id": "p_cla001",
          "given": "Claire",
          "surname": "Langford",
          "birth_name": "Ortega",
          "birth": "1995-02-17",
          "death": "",
          "phone": "555-201-0011",
          "email": "claire.langford@example.com"
        }
      ],
      "dependents": [
        {
          "id": "p_mia001",
          "given": "Mia",
          "surname": "Langford",
          "birth": "2021-04-30",
          "death": ""
        }
      ],
      "anniversary": "2019-09-07",
      "address": {
        "lines": [
          "17 Birch Lane",
          "Springfield, IL 62704"
        ]
      }
    },
    {
      "id": "h_nova01",
      "adults": [
        {
          "id": "p_pat001",
          "given": "Patricia",
          "surname": "Novak",
          "aka": "Pat",
          "birth": "1952-05-09",
          "death": "",
          "phone": "555-307-0001",
          "email": "pat.novak@example.com"
        }
      ],
      "anniversary": "",
      "address": {
        "lines": [
          "88 Oakwood Drive",
          "Shelbyville, IL 62565",
          "P.O. Box 212, Shelbyville, IL 62565"
        ]
      }
    }
  ]
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/store/golden_test.go`:

```go
package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
	"github.com/asphaltbuffet/wherefolk/pkg/rolo"
)

const examplePath = "../../testdata/directory.json"

func TestLoadExampleDirectory(t *testing.T) {
	tests := []struct {
		name      string
		checkFunc func(t *testing.T, doc *store.Document, tree *rolo.Tree)
	}{
		{
			name: "three households",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				assert.Len(t, doc.Households, 3)
			},
		},
		{
			name: "two roots, ordered by eldest adult's birth date",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				roots := tree.Roots()
				require.Len(t, roots, 2)
				assert.Equal(t, "Patricia", roots[0].Label(), "Patricia b. 1952 precedes Robert b. 1965")
				assert.Equal(t, "Robert/Susan", roots[1].Label())
			},
		},
		{
			name: "a married child with a dependent is its own household",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				kids := tree.Children("h_lang01")
				require.Len(t, kids, 1)
				assert.Equal(t, "Daniel/Claire", kids[0].Label())
			},
		},
		{
			name: "path disambiguates a nested household",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				path, err := tree.PathString("h_lang02")
				require.NoError(t, err)
				assert.Equal(t, "Robert/Susan › Daniel/Claire", path)
			},
		},
		{
			name: "an unmarried adult child remains a dependent",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang01")
				require.True(t, ok)

				var emma rolo.Person
				for _, d := range h.Dependents {
					if d.Given == "Emma" {
						emma = d
					}
				}
				require.Equal(t, rolo.PersonID("p_emm001"), emma.ID)
				assert.Equal(t, "555-201-0020", emma.Phone,
					"a dependent carries contact details; this is an address book")
			},
		},
		{
			name: "a deceased dependent keeps both dates",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang01")
				require.True(t, ok)

				var thomas rolo.Person
				for _, d := range h.Dependents {
					if d.Given == "Thomas" {
						thomas = d
					}
				}
				require.True(t, thomas.IsDeceased())
				assert.Equal(t, rolo.Date{Year: 1996, Month: 7, Day: 19}, thomas.Birth)
				assert.Equal(t, rolo.Date{Year: 2022, Month: 1, Day: 8}, thomas.Death)
			},
		},
		{
			name: "a nickname renders in double quotes",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_nova01")
				require.True(t, ok)
				assert.Equal(t, `Patricia "Pat" Novak`, h.Adults[0].DisplayName())
			},
		},
		{
			name: "multiple address lines survive",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_nova01")
				require.True(t, ok)
				assert.Len(t, h.Address.Lines, 3)
			},
		},
		{
			name: "a minor dependent is gated by age",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				h, ok := tree.Get("h_lang02")
				require.True(t, ok)

				asOf := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
				require.Len(t, h.Dependents, 1)
				assert.True(t, h.Dependents[0].IsMinor(asOf), "Mia b. 2021 is a minor in 2026")
			},
		},
		{
			name: "no household in the example is memorial",
			checkFunc: func(t *testing.T, doc *store.Document, tree *rolo.Tree) {
				t.Helper()
				require.NoError(t, tree.Walk(func(h rolo.Household, _ int) error {
					assert.False(t, h.IsMemorial(), "%s should not be memorial", h.Label())
					return nil
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := store.Load(examplePath)
			require.NoError(t, err)

			tree, err := doc.Tree()
			require.NoError(t, err)

			tt.checkFunc(t, doc, tree)
		})
	}
}

func TestExampleDirectorySurvivesSaveLoad(t *testing.T) {
	original, err := store.Load(examplePath)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, original))

	reloaded, err := store.Load(path)
	require.NoError(t, err)

	assert.Equal(t, original, reloaded, "a save/load cycle must not alter the document")
}

func TestExampleDirectoryIsCanonicallyFormatted(t *testing.T) {
	original, err := store.Load(examplePath)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "directory.json")
	require.NoError(t, store.Save(path, original))

	want, err := os.ReadFile(path)
	require.NoError(t, err)

	got, err := os.ReadFile(examplePath)
	require.NoError(t, err)

	assert.Equal(t, string(want), string(got),
		"testdata/directory.json should match what Save produces; run the test, then copy the saved file over it")
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestLoadExample -v`
Expected: FAIL — `open ../../testdata/directory.json: no such file or directory` if Step 1 was skipped; otherwise the assertions run

- [ ] **Step 4: Make the tests pass**

No production code is needed — Tasks 1–7 already implement everything, and the JSON in Step 1 is already in the exact form `Save` produces.

If `TestExampleDirectoryIsCanonicallyFormatted` nonetheless fails on whitespace or key order — which would mean a struct field or tag was mistyped in an earlier task — regenerate the file rather than hand-fixing it:

```bash
cat > internal/store/canon_test.go <<'EOF'
package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherefolk/internal/store"
)

func TestRegenerateCanonical(t *testing.T) {
	doc, err := store.Load("../../testdata/directory.json")
	require.NoError(t, err)
	require.NoError(t, store.Save("../../testdata/directory.json", doc))
}
EOF
go test ./internal/store/ -run TestRegenerateCanonical
rm internal/store/canon_test.go
```

Then inspect the diff before re-running the suite — if a field vanished, a tag is wrong.

- [ ] **Step 5: Run the full store suite**

Run: `go test ./internal/store/ -v`
Expected: PASS — every test function

- [ ] **Step 6: Commit**

```bash
jj commit -m "test(store): round-trip the example directory in the new format"
```

---

### Task 9: Verify no superseded code remains

**Superseded — the deletions moved into Task 2.** `family.go` shares a package with `person.go`, so the package could not compile, and no test in it could run, while both shapes existed. Task 2 therefore deletes the old model and CLI as part of its own commit.

What remains here is a verification pass.

**Files:**
- Delete: `pkg/rolo/family.go`, `pkg/rolo/family_test.go`
- Delete: `cmd/print.go`, `cmd/print_test.go`
- Delete: `testdata/example.json`, `pkg/rolo/testdata/example.golden`
- Modify: `cmd/root.go` (drop the print command registration)

**Interfaces:**
- Consumes: nothing
- Produces: a package that builds

- [ ] **Step 1: Confirm what references the old code**

```bash
rg -n "rolo\.Family|LoadJSON|GetPrintCmd|MakeTree" --type go
```

Expected: matches only in the files being deleted, plus `cmd/root.go`

- [ ] **Step 2: Confirm the files are already gone**

```bash
ls pkg/rolo/family.go cmd/print.go testdata/example.json 2>&1
```

Expected: "No such file or directory" for each. If any still exists, delete it and note the discrepancy in your report.

- [ ] **Step 3: Confirm `cmd/root.go` no longer registers the print command**

```bash
rg -n "AddCommand" cmd/root.go
```

Expected: no matches.

- [ ] **Step 4: Verify the module builds and every test passes**

```bash
go build ./...
go vet ./...
go test ./...
```

Expected: build succeeds, vet is silent, and `pkg/rolo` and `internal/store` both pass. `internal/tui` has no tests and is currently unreferenced; that is expected and is addressed in a later work item.

- [ ] **Step 5: Commit**

```bash
jj commit -m "refactor: remove recursive Family model and print CLI"
```

---

### Task 10: Document the model

**Files:**
- Modify: `CLAUDE.md`

**Interfaces:**
- Consumes: everything above
- Produces: project documentation matching the new code

The project instruction is to update documentation before declaring work done, and `CLAUDE.md` currently describes the recursive model in detail — every statement in its Architecture and Data Format sections is now wrong.

- [ ] **Step 1: Rewrite the Architecture and Data Format sections**

In `CLAUDE.md`, replace the `## Architecture` and `## Data Format` sections with:

```markdown
## Architecture

- **`main.go`** — entry point, delegates to `cmd.Execute()`
- **`pkg/rolo/`** — domain types, no persistence
  - `Person` — a flat record with a stable `PersonID`, partial-precision `Date`s, and per-field `Hidden` flags
  - `Household` — adults, dependents, anniversary, address, and a `Parent` link. Children are **not** stored
  - `Tree` — derived at load time by grouping Households on `Parent`. Provides `Roots`, `Children`, `Path`, `PathString`, and `Walk`
  - `Date` — partial precision: a date may know a year only, a year and month, or nothing at all
- **`internal/store/`** — persistence
  - `Document` — the on-disk shape: a `schema` version plus a flat `[]Household`
  - `Load` validates the schema version and the tree, refusing a document newer than `CurrentSchema`
  - `Save` writes atomically (temp → fsync → rename) with `0600` permissions
  - `NewPersonID`/`NewHouseholdID` generate prefixed nanoids over a Crockford base32 alphabet
- **`internal/tui/`** — lipgloss styles, currently unreferenced

## Data Format

`testdata/directory.json` is the canonical example. A document is an object with a `schema`
version and a flat `households` array — **not** a nested tree. Each Household carries its own `id`
and an optional `parent`; the hierarchy is rebuilt from those links by `rolo.BuildTree`.

Sibling Households are ordered by their eldest adult's birth date, derived rather than stored.

See `CONTEXT.md` for the domain vocabulary and `docs/adr/` for the decisions behind this shape.
```

- [ ] **Step 2: Remove the stale Notes entries**

In the `## Notes` section of `CLAUDE.md`, delete these three lines, which refer to deleted code:

- The line about `MakeTree()` and `Info()` being commented out in `cmd/print.go`
- The line about `ioutil.ReadFile` in `family.go`
- The line about `Person.Aka` rendering via `familyName()`

Replace the last with:

```markdown
- `Person.DisplayName()` renders a nickname as `Given "Aka" Surname`.
```

- [ ] **Step 3: Verify the documented commands still work**

```bash
go build ./...
go test ./...
```

Expected: both succeed

- [ ] **Step 4: Commit**

```bash
jj commit -m "docs: update CLAUDE.md for the flat data model"
```

---

## Verification

After Task 10, the following should all hold:

```bash
go build ./...                    # succeeds
go vet ./...                      # silent
go test ./...                     # all pass
rg -n "rolo\.Family" --type go    # no matches
```

And the design's requirements for this work item are met:

| Requirement (design §3) | Where |
|---|---|
| Flat records | Task 2, Task 3 |
| Stable IDs | Task 4 |
| Atomic JSON write | Task 6 |
| Load-time tree derivation | Task 5 |
| `schema` version field | Task 7 |
| Refuse data newer than the binary | Task 7 |
| Shared Address as a reference | Task 3 (`Address.SharedWith`) |
| Death as a property of a Person | Task 2 (`IsDeceased`), Task 3 (`IsMemorial`) |
| Per-field hidden flags | Task 2 (`HiddenFields`) |

## Not in this work item

- **The migration runner** (work item 2). Task 8's hand-converted `testdata/directory.json` is its specification, and `CurrentSchema = 1` is its starting point.
- **Undo, trash, and snapshots** (work item 6). `Save` is a plain overwrite for now.
- **Normalisation of phones, emails, and addresses** (work item 3). `ParseDate` validates dates, but nothing yet normalises `555.201.0001` to `555-201-0001`.
- **Promotion of a Dependent to a Household** (work item 5). The model expresses both states; no code performs the transition yet.
