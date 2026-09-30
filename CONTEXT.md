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

**Birthday Calendar**:
Pages at the end of every Directory listing living people by name against the twelve months, with
the day of each person's birthday in their month — so the family knows when to send a card.
A person appears only when their birth month is known and their birth date is not withheld; a
month known without its day shows `?` in that month. The deceased never appear: a row would read
as a reminder to send them a card.

Each person is listed surname first — `Novak, Patricia "Pat"` — with their given name and any
nickname exactly as their own row prints it, then any birth name that differs from their surname in
parentheses, as in the Household Name: `Yoder, Dawn (Mitchell)`. The birth name lets a relative find
someone by the name they grew up with, and tells apart two people who share a name. Rows are
ordered by surname and then given name without regard to capitalisation.

Beside each name is the page on which that person's Household begins, so a reader who wants to
send the card can turn straight to the address. On screen, the name and the page both jump there.

A Proof Sheet never carries a Birthday Calendar: it shows one Household's own entry, and the
calendar lists the whole family.
_Avoid_: Birthday list, birthday page, calendar

**Title page**:
The first page of a Directory, and page 1 of its numbering, though it prints no page number. It
carries the Directory Title and the facts that let a reader judge a copy at a glance: when it was
generated and which audience it was made for. A Directory for the Full audience says DO NOT
DISTRIBUTE here too, because the cover is the page most likely to be seen on its own.
_Avoid_: Cover sheet, front matter

**Directory Title**:
The name the Editor gives the Directory as a whole — `The Langford Family Directory`. One line,
the same in every tier, never withheld. When the Editor has not named it, the Directory is titled
`Family Directory`. Changing it is an edit like any other and can be undone.
_Avoid_: Name (that belongs to a Person or a Household), heading

**Table of Contents**:
The page near the front of a Directory that lists where to find each first-generation Branch.
It names every root Household and every child of a root Household by its Household Name, with the
page its block starts on. Deeper Households are not listed: each one's block follows its parent's
in the Directory, so the reader finds it by turning a page or two from its ancestor's entry. The
Birthday Calendar and any other sections are listed too.
_Avoid_: Index (that would imply every Household or every person), outline

**Proof Sheet**:
A single Household's own entry, rendered on its own page and sent to that Household so they can
confirm or correct what the Directory holds about them. Shows their own withheld fields, marked as
withheld; never shows another Household's. Produced only for Households with a living member.
It carries no Title page and no Table of Contents, whose entries would name other Households —
only the Directory Title as a line at its head, so the recipient knows what they are checking.
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

**Household Name**:
The name a Household goes by wherever it stands alone — `Daryl & Dawn (Mitchell) Yoder`. It heads
the Household's block in the Directory, and in the editing UI it names the Household in the tree and
heads its detail pane, so the Editor sees each Household as the family will. It carries surnames
and birth names that a Path segment omits. Only a Path keeps the compact `Daryl/Dawn` form, because
a Path chains several generations and a full name per link would be unreadable.

When the adults share a surname it is printed once, last, and each adult who was born under a
different surname carries it in parentheses after their given name. When their surnames differ,
each adult is named in full — `Chris Yoder & Sam Patel` — and no birth name is shown, since a birth
name only reads as "née" beside a surname taken in marriage. A birth name equal to the current
surname is never shown. A single adult prints as their own name: `Patricia Novak`.

A nickname never appears in the Household Name, which is the formal name one would write on an
envelope. It appears on the person's own row — `Patricia "Pat"` — where a reader looks for them.

Beneath the Household Name, each person's row drops their surname when that surname already
appears in it, and prints it otherwise: under `Daryl & Dawn (Mitchell) Yoder`, a Dependent Kyle
Yoder is `Kyle` but a stepchild Jordan Mitchell is `Jordan Mitchell`, because a parenthesised birth
name is not a surname the Household carries.

A Memorial Household prints under a Household Name by the same rules, and a Shared Address
back-reference names the referenced Household by its Household Name, so the pointer matches the
heading the reader is scanning for.

A birth name prints in every tier, as part of the Household Name, and is not a Withheld field.
_Avoid_: Label (that is the Path segment), title

**Shared Address**:
A Household whose Address is a reference to its parent Household's Address rather than its own
text. Stays in sync when the parent's Address changes and renders as a back-reference instead of a
repeated Address.

The back-reference is printed only when the referenced Household's own block shows address lines.
Otherwise the sharer prints what its Address actually resolves to, following the references to
their end. When the referenced block shows `[private]` the sharer shows `[private]` too, rather
than pointing the reader at a marker. When it shows nothing because it is a Memorial Household, or
shows a back-reference of its own — three generations under one roof — the sharer prints the lines
itself, so the reader never follows a pointer to a block that holds no address.

**Render model**:
The Directory as a flat sequence of already-rendered strings, one step before it becomes Typst
markup. It holds no dates, no flags and no domain types, so that every decision about what a
given audience may see has already been made by the time anything is printed. Constructing the
render model is where tier filtering happens; rendering it is not.
_Avoid_: View model (that is the editing UI's, in `internal/web`), DTO

**Withheld field**:
A field the Editor has marked hidden. Renders as `[private]` in every Directory tier, so nobody
re-collects it next year. Distinct from a **Suppressed field**, which renders as nothing at all.
Phone, email, and birth date are withheld per Person; an Address is withheld per Household.

Suppression takes precedence: `[private]` appears only where the audience would otherwise have seen
the value. A withheld phone in the Mail tier prints nothing, like every other phone there, because
a lone marker would single that person out. A withheld field with nothing recorded also prints
nothing — the marker says "we have this and are not sharing it", which would be untrue.

Withholding is a statement about **export only**. The editing UI always shows the stored value,
marked withheld by a checkbox beside it — the Editor is the document's author, not one of its
audiences, and a value they cannot see is a value they cannot correct or clear.
_Avoid_: Private field, redacted, suppressed

**Suppressed field**:
A field omitted for a whole audience rather than by the Editor's choice — a phone number in the
Mail tier, a minor's phone and email outside Full, or a deceased person's contact details anywhere.
A minor's birth date is not suppressed: it is Truncated like any living person's, because the
Directory is how the family knows when to send a birthday card.
Renders as **nothing at all**, never as a marker: a placeholder would advertise that the data
exists, leaking exactly what the suppression is meant to omit. Contrast a **Withheld field**,
which the Editor marked hidden and which renders as `[private]`.

Suppression, like withholding, applies to **export only**. The editing UI shows a deceased
person's recorded contact details like any other field, because the Editor must be able to correct
or clear them.
_Avoid_: Tier-suppressed (suppression is not always a tier rule), filtered, hidden, redacted

### Safety net

**Trash**:
Where a deleted Household waits for at least 30 days before it is gone for good, so a deletion
noticed a fortnight later can still be reversed. Only a Household can be deleted, and only one that
nothing depends on: no Household beneath it, no Household sharing its Address, and never a Memorial
Household. Restoring a Household whose parent or Shared Address was deleted after it brings those
back too, so nothing returns pointing at a Household that is not there — and for the same reason a
Household is kept in the Trash for as long as anything else in it still needs it.
A Person removed from a Household does not enter the Trash; that is an edit, reversed by Undo.
The Editor sees it as *Recently deleted*.
_Avoid_: Recycle bin, archive, deleted items

**Undo**:
Reverses the Editor's most recent save — an edit, a deletion or a restore — exactly, as offered
beside the sentence announcing it. One step only, and only while nothing has been saved since:
undoing a change that later work was built on would discard that work. It lasts as long as the
service keeps running; beyond that, a deleted Household can still be restored from the Trash; any
other change can only be reversed by editing it back.
_Avoid_: Revert, rollback, history

### Dates

**Truncated Date**:
A date rendered as month and day, omitting the year, to keep a living person's full date of birth
or marriage out of wide circulation — `Mar 12`. The default for living people outside the Full
tier. A date known only to the month truncates to the month alone (`Mar`); a date known only to
the year truncates to nothing, because the year is all it holds.

**Whole Date**:
A date rendered in full, with year — `Mar 12, 1965`, or `Mar 1938` and `1938` at lesser
precision. Used for any date concerning only deceased people — a death, a deceased person's birth,
or an Anniversary where both adults have died — and for every date in the Full tier.

A deceased person's dates print together as one range, `Mar 12, 1965 – Jun 3, 2001`, whether
they are in a Memorial Household or not.

Both forms name the month as a three-letter abbreviation — `Jan`, `Sep`, `Dec`. A printed Directory
is read by relatives, not parsed, and `03-12` cannot say whether it means March or December; the
abbreviation keeps the word and loses only width.

### Flagged ambiguities

**"Family"** is deliberately absent from this glossary. The existing code uses `Family` for three
distinct concepts — the printable block (**Household**), a single dependent person (**Dependent**),
and a subtree (**Branch**). Use the specific term.
