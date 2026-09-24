# Wherefolk

Wherefolk produces a family directory: a printable, shareable address book covering an extended
family. Its primary job is contact information; secondarily it records important dates and makes
familial relationships legible.

## Language

### Core entities

**Person**:
A single human being in the directory. Carries name, dates, and contact details (phone, email).
Every Person belongs to exactly one Household.

**Household**:
The unit that renders as one block in the Directory. A node becomes a Household when it has a
spouse, has Dependents, or has its own Address. Holds one or two adults, an optional Anniversary,
an optional Address, and any Dependents.
_Avoid_: Family, unit

**Dependent**:
A Person listed under a Household who does not head a Household of their own — a minor child, an
adult child still at home, a deceased child. Carries contact details but never an Address or an
Anniversary; acquiring either makes them a Household.
_Avoid_: Child, member

**Memorial Household**:
A Household whose adults are all deceased. Persists permanently — it anchors the Path of every
Branch beneath it — and appears in every tier as a names-and-dates reference, carrying no Address
or contact details because it has no living adult to own them.
_Avoid_: Dead household, historical household, ancestor node

**Branch**:
A Household together with every Household descended from it. A navigation and grouping concept
only; Branches are never nested in rendered output.
_Avoid_: Subtree, family line

**Directory**:
The complete published artifact — every Household, ordered and formatted for printing or sharing.
_Avoid_: Book, export, roster

**Proof Sheet**:
A single Household's own entry, rendered on its own page and sent to that Household so they can
confirm or correct what the Directory holds about them. Shows their own withheld fields, marked as
withheld; never shows another Household's. Produced only for Households with a living member.
_Avoid_: Review sheet, verification page

### Structure and navigation

**Path**:
The chain of Households from the root to a given Household, rendered as
`Aden/Nettie › Clyde/Doris › Dave/Diane`. Serves as the human-readable identity of a Household,
disambiguating people who share a given name. A Path is a display identity and changes if the tree
is restructured; it is never a storage identity.

**Promotion**:
The transition of a Dependent into a Household, triggered by gaining a spouse, a Dependent, or an
Address. Structural rather than declared — the act of adding a spouse *is* the promotion.

Promotion is **one-way**. The three triggers say when a Dependent *becomes* a Household; they are
not a condition a Household must keep satisfying. A Household that loses its Address stays a
Household, because its identity is stable, it may anchor a Branch beneath it, and silently
dissolving a record to undo a typo is precisely the surprise the announcement exists to prevent.
There is no demotion.

In the editing UI the Editor asks for a Promotion directly, on the Dependent's row, and then fills
in the spouse or Address in the new Household's own pane. The trigger is still structural — what
makes Dave a Household is that he now *has* one — but the editing pane governs one Household at a
time, so it cannot grow a nested form for each Dependent's future spouse.

A Household's form offers two blank slots, one for an adult and one for a Dependent, because the
two are structurally different (§3) and a Household with a single adult must still be able to gain
a Dependent. The slot a person arrives in is what decides where they belong; nothing infers it.

**Shared Address**:
A Household whose Address is a reference to its parent Household's Address rather than its own
text. Stays in sync when the parent's Address changes and renders as a back-reference instead of a
repeated Address.

**Withheld field**:
A field the Editor has marked hidden. Renders as `[private]` in every Directory tier, so nobody
re-collects it next year. Distinct from a **Suppressed field**, which renders as nothing at all.
Phone, email, and birth date are withheld per Person; an Address is withheld per Household.

Withholding is a statement about **export only**. The editing UI always shows the stored value,
marked withheld by a checkbox beside it — the Editor is the document's author, not one of its
audiences, and a value they cannot see is a value they cannot correct or clear.
_Avoid_: Private field, redacted, suppressed

**Suppressed field**:
A field omitted for a whole audience rather than by the Editor's choice — a phone number in the
Mail tier, a minor's details outside Full, or a deceased person's contact details anywhere.
Renders as **nothing at all**, never as a marker: a placeholder would advertise that the data
exists, leaking exactly what the suppression is meant to omit. Contrast a **Withheld field**,
which the Editor marked hidden and which renders as `[private]`.

Suppression, like withholding, applies to **export only**. The editing UI shows a deceased
person's recorded contact details like any other field, because the Editor must be able to correct
or clear them.
_Avoid_: Tier-suppressed (suppression is not always a tier rule), filtered, hidden, redacted

### Dates

**Truncated Date**:
A date rendered as month and day, omitting the year, to keep a living person's full date of birth
or marriage out of wide circulation. The default for living people outside the Full tier.

**Whole Date**:
A date rendered in full, with year. Used for any date concerning only deceased people — a death, a
deceased person's birth, or an Anniversary where both adults have died — and for every date in the
Full tier.

### Flagged ambiguities

**"Family"** is deliberately absent from this glossary. The existing code uses `Family` for three
distinct concepts — the printable block (**Household**), a single dependent person (**Dependent**),
and a subtree (**Branch**). Use the specific term.
