# Keep the Trash in a sidecar file, not in the document

Deleted Households wait in the Trash for at least 30 days (§3). They are kept in `trash.json`
beside `directory.json`, rather than as a `trash` array inside the document or as a `deleted` flag
on each Household, and `directory.json` stays at schema 1.

Adding the Trash to the document would change its shape. An older binary that loaded it after a
rollback would drop the unknown field on its next save and erase the Trash without a word —
exactly the silent truncation §2.1 calls the worst bug in the system. The refuse-if-newer guard
would prevent that only after a bump to schema 2, and that bump would force the migration runner
that item 2 deliberately deferred into existence for a change that is purely additive. A flag on
each Household would be worse still: every reader of the document — the tree, search, `render.Build`
— would have to remember to filter it, which is the class of leak ADR-0010 and the render model's
single constructor were built to prevent. A Household in `trash.json` is simply absent from the
Directory, so nothing has to remember anything.

## Consequences

A deletion or restore changes two files, and no filesystem makes two renames atomic. Every write
therefore saves the file *gaining* a copy of the Household before the one losing it, so a crash
between the two leaves the Household in both files rather than in neither, and loading reconciles
that duplicate by dropping the Trash entry whose ID is already in the document. The document wins
because it is what the Editor last saw.

The Operator's hand-repair path now has a second file to read, but it is the same shape of JSON
written the same way (atomic, `0600`), and a document restored from a snapshot needs nothing from
it.
