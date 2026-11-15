# Pull requests

PR bodies follow `.github/pull_request_template.md`. GitHub pre-fills it only in the web UI and interactive `gh pr create`; `--body` / `--body-file` bypass it, so agents must apply it by hand.

## Conventions

- **Title**: a conventional commit (`feat(scope): …`, `fix: …`, `docs: …`), matching the primary commit.
- **Body**: copy the template to a scratch file, fill **every** section, and write `N/A` in sections that don't apply — never delete a heading.
- **Remove the `<!-- … -->` guidance comments** from the filled body. They don't render, but they clutter the raw body that review agents read via `gh pr view`.
- **Testing**: paste real results (e.g. `mise run test` → 975 pass, 2 skipped), not "tests pass".
- **Create**: `gh pr create --title "…" --body-file <scratch file>`. Push the bookmark first with `jj git push --bookmark <name>`.
- **Draft**: add `--draft` if any checklist item is unchecked for a reason other than N/A.

## When a skill says "open a PR" or "create a pull request"

Follow the conventions above. If the repo has no PR template, fall back to the skill's own format.
