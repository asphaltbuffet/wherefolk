# Control disclosure through export tiers, not document hardening

An exported Directory cannot be protected from a recipient who is meant to read it — PDF permission
flags are advisory and stripped trivially, and rasterising text merely invites OCR — so disclosure
is controlled by what enters the file rather than by hardening the file itself. Four tiers named
for what the recipient will do with them (Mail, Call, Digital, Full) progressively admit phone
numbers, email addresses, and finally minors' details and birth years.

## Consequences

Contact details are gated on age computed at export time, so exports are not reproducible: the same
Directory exported months apart differs as people turn 18. People with no recorded birthdate are
treated as minors, which fails closed but silently suppresses details for elderly relatives whose
birth year is unknown, so export must warn about them by name. Fields the author marked hidden
render as `[private]` to distinguish withheld data from missing data, while tier-suppressed fields
render as absence — a marker there would advertise the existence of the data the tier omits.

Date truncation is governed by whether the people concerned are living, not by tier alone: a
deceased person's dates, and an Anniversary where both adults have died, always render in full,
because truncation exists to keep a living person's identity-verification key out of circulation
and buys nothing for the dead.

Passphrase protection of the Full tier is a post-processing step via `pdfcpu`, since the renderer
chosen in ADR-0004 cannot encrypt PDFs.
