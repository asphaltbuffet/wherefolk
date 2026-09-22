# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`wherefolk` (module: `github.com/asphaltbuffet/wherefolk`) is a CLI tool called `wherefolk` for managing and displaying family contact information from JSON files.

## Commands

```bash
# Build
go build ./...

# Run tests
go test ./...

# Run a single test
go test ./pkg/rolo/ -run TestBuildTree

# Run the tool
go run .
```

The CLI is currently a bare Cobra root with no subcommands — the `print` command was
removed along with the recursive model. The web UI replaces it; see
[docs/design/high-level-design.md](docs/design/high-level-design.md).

## Architecture

- **`main.go`** — entry point, delegates to `cmd.Execute()`
- **`pkg/rolo/`** — domain types, no persistence
  - `Person` — a flat record with a stable `PersonID`, partial-precision `Date`s, and per-field `Hidden` flags
  - `Household` — adults, dependents, anniversary, address, and a `Parent` link. Children are **not** stored
  - `Tree` — derived at load time by grouping Households on `Parent`. Provides `Roots`, `Children`, `Path`, `PathString`, and `Walk`
  - `Date` — partial precision: a date may know a year only, a year and month, or nothing at all
- **`internal/store/`** — persistence
  - `Document` — the on-disk shape: a `schema` version plus a flat `[]Household`
  - `Load` validates the schema version and the tree, refusing a document newer than `CurrentSchema`
  - `Save` writes atomically (temp → fsync → rename) with `0600` permissions
  - `NewPersonID`/`NewHouseholdID` generate prefixed nanoids over a Crockford base32 alphabet
- **`internal/tui/`** — lipgloss styles, currently unreferenced

## Data Format

`testdata/directory.json` is the canonical example. A document is an object with a `schema`
version and a flat `households` array — **not** a nested tree. Each Household carries its own `id`
and an optional `parent`; the hierarchy is rebuilt from those links by `rolo.BuildTree`.

Sibling Households are ordered by their eldest adult's birth date, derived rather than stored.

See `CONTEXT.md` for the domain vocabulary and `docs/adr/` for the decisions behind this shape.

## Testing

- All tests must be table-driven: a `tests []struct{ name string; ... }` slice iterated with `t.Run(tt.name, ...)`.
- Test data lives in `testdata/example.json` — a two-family dataset covering: maiden names, marriage dates, dependents (living and deceased), nested sub-families, multiple addresses, and `aka`.
- Use a `checkFunc func(t *testing.T, ...)` field in table rows that need assertions beyond simple field comparisons.

## Notes

- Cobra commands must write output via `cmd.OutOrStdout()` (e.g. `fmt.Fprintln(cmd.OutOrStdout(), ...)`) — bare `fmt.Println` bypasses `SetOut` and breaks test capture.
- `Person.DisplayName()` renders a nickname as `Given "Aka" Surname`.

## Agent skills

### Issue tracker

Issues live in GitHub Issues at `asphaltbuffet/wherefolk`, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles use their default label strings. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
