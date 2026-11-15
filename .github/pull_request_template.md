## Linked issues

<!--
`Closes #N` for each issue this PR resolves (one per line, so GitHub auto-closes them).
`Refs #N` for related issues it does not resolve. Write N/A if there are none.
-->

## Summary

<!--
What this PR does and WHY, in 2-5 sentences. Lead with the problem or motivation;
the diff already shows what changed. Link the design discussion or ADR if there is one.
-->

## Changes

<!--
Bullets grouped by behaviour or component, not by file. Bold a short label per bullet.
Call out anything user-visible (editing UI, exported Directory, env vars, container image).
-->

## How was this tested

<!--
Exact commands run and their real results (counts, exit status), not "tests pass".
Name new or changed tests. List manual checks as `- [x]` done / `- [ ]` not done.
Render tests skip without typst: say whether they ran or skipped.
-->

## Risk & compatibility

<!--
Breaking changes, migrations, and how to roll back. For this repo, state explicitly:
- document, Trash, or settings shape changes (`schema` bumps; `Load` refuses a newer document)
- audience rules in `render.Build`: anything that could put a withheld or suppressed value into an export tier
- `WHEREFOLK_*` env vars, the Typst pin, or deployment artefacts in `deploy/` (Funnel stays off)
Write N/A if none apply.
-->

## Reviewer focus

<!--
Where you are least confident, inputs or edge cases your tests do not cover,
and judgment calls you made that a reviewer might decide differently.
-->

## Out of scope / follow-ups

<!--
What was deliberately left out and why, with links to issues tracking it.
Deferred work is filed as an issue, never left as a TODO in code. Write N/A if none.
-->

## Checklist

- [ ] Title follows conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`)
- [ ] `mise run lint` and `mise run test` pass
- [ ] `changie new` fragment added, or not needed because: <!-- e.g. docs-only, test-only, CI-only -->
- [ ] README.md, `docs/`, `CONTEXT.md`, and ADRs updated where behaviour or terminology changed (or N/A)
- [ ] Deferred work tracked as GitHub issues, not TODOs
