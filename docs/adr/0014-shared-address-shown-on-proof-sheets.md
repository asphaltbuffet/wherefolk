# Show a Shared Address on a Proof Sheet even when it is withheld

Amends ADR-0005, whose consequences say a Proof Sheet never shows withheld fields belonging to
another Household. A Household with a Shared Address is sent the Address it actually resolves to,
withheld or not, marked as shared and naming the Household it is shared with, so the recipient can
say it is no longer shared or correct the Address itself. The people sharing an address live there,
so withholding it is a statement about the Directory's audiences, not about them.

## Considered Options

- **Hold ADR-0005's rule.** When any link of the reference chain is withheld, print "address
  withheld" with empty fields. Protects the referenced Household when a reference has gone stale,
  at the cost of an empty field the sharer must fill from memory.

## Consequences

A stale reference leaks: if a Dependent's grown Household still shares their parents' Address after
moving out, and the parents move and withhold the new one, the sharer's Proof Sheet carries it.
Keeping references current is the Editor's job, and the "no longer shared" checkbox on every sheet
is how a stale one gets reported. A correction made on a sharer's sheet changes the referenced
Household's Address, and the sheet says so.
