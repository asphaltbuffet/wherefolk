# Collect corrections outbound only, through Proof Sheets

Relatives cannot correct what they have never seen, but accepting inbound submissions would
reintroduce the public exposure, identity model, and review queue that ADR-0001 removed. Instead
the Directory renders a Proof Sheet per living Household, which the Editor sends out and the
recipient replies to by any channel they like, leaving the Editor the sole writer.

## Consequences

A Proof Sheet shows the recipient their own withheld fields, clearly marked as withheld, because
the recipient is the data subject and a `[private]` placeholder would give them nothing to confirm;
it never shows withheld fields belonging to another Household. Because it paginates one Household
per page rather than merely filtering fields, it is a different kind of tier from Mail, Call,
Digital and Full, and a full run produces roughly one page per living Household — so the export UI
should support generating sheets for a single Branch rather than only the whole family.
