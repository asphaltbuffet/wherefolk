# Keep document-level settings in a sidecar file

The Directory Title is the first value that belongs to the Directory as a whole rather than to a
Household. It lives in `settings.json` beside the document, at its own schema version, and
`directory.json` stays at schema 1. This is the choice ADR-0012 made for the Trash, for the same
reason: putting the field in the document would make a rollback's older binary drop it silently on
its next save, and the refuse-if-newer guard only prevents that after a bump to schema 2. That bump
would force the deferred migration runner (§2.1, item 2) into existence for a purely additive field.

## Considered Options

- **The document at schema 2.** Keeps family data in one file and in its snapshots, but builds the
  migration runner around a migration that does nothing but change the version number.
- **An environment variable.** Needs no file, but the title is the Editor's to set and to undo, and
  the Editor does not edit the Operator's `.env`.

## Consequences

A missing `settings.json` means no title has been set, and the Directory is titled
`Family Directory`. The file holds Editor-set values only; the Operator's configuration stays in
the environment. A save changes either the settings file or the document and Trash, never both, so
`Persist` needs no new ordering rule — keep it that way, or a crash between writes needs one.
Snapshots (item 14) must copy `settings.json` along with the document and the Trash, or a restored
snapshot comes back untitled.
