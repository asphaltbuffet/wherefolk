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

**Shared Address**:
A Household whose Address is a reference to its parent Household's Address rather than its own
text. Stays in sync when the parent's Address changes and renders as a back-reference instead of a
repeated Address.

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
