# Promotion is one-way

§3 says a node becomes a Household when it has a spouse, has Dependents, or has its own Address,
and that Promotion is structural rather than declared: adding a spouse to Dave *is* the act of
making Dave a Household. Read as a biconditional, that rule also says what happens when the last
of those three goes away — clearing Dave's Address would dissolve his Household and return him to
his parents' block as a Dependent.

It is not a biconditional. The three triggers say when a Dependent **becomes** a Household. Nothing
demotes a Household afterwards. A Household that loses its Address is an ordinary Household with no
Address recorded, which is a common and valid state.

## Consequences

A `HouseholdID` is stable in the strong sense `pkg/rolo` already claims for it: once issued it
names that Household for the life of the Directory. Bookmarks, `?open=` sets, and — once item 6
lands — snapshot diffs can rely on it, because the only way a Household leaves the document is an
explicit deletion the Editor asked for.

Clearing a mistyped Address is a field edit and nothing more. Under symmetric demotion it would
have destroyed a record, reparented a Person, and invalidated every link to them, all as a silent
consequence of emptying a text box. §3 introduces the announcement precisely because structural
change should never be a surprise; automatic demotion would have manufactured the surprises the
announcement exists to report.

The orphan case disappears. A Household with Households beneath it can never vanish on its own, so
no code needs to decide what becomes of a Branch whose root dissolved, and §3's promise that a
Memorial Household is never deleted needs no special-casing against a demotion path — the
Memorial rule and the persistence rule now say the same thing for the same reason.

The cost is that a genuine mistake — promoting the wrong Dependent — is not reversible by undoing
the trigger. The Editor must delete the Household explicitly, which is a deliberate act with its
own confirmation, and which item 6's Trash makes recoverable for 30 days. That is the right shape:
destroying a record should require asking for it.
