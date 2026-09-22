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

There is no CLI. The binary is a service: `main` loads the store and starts the web server.
An Operator CLI (export, validate, repair) may return as its own work item, built with the
standard library `flag` package. See [docs/design/high-level-design.md](docs/design/high-level-design.md).

## Architecture

- **`main.go`** — entry point: reads config, loads the store, starts the web server
- **`internal/config/`** — environment parsing (`WHEREFOLK_DATA`, `WHEREFOLK_PORT`)
- **`internal/web/`** — HTTP handlers and embedded templates
- **`pkg/rolo/`** — domain types, no persistence
  - `Person` — a flat record with a stable `PersonID`, partial-precision `Date`s, and per-field `Hidden` flags
  - `Household` — adults, dependents, anniversary, address, and a `Parent` link. Children are **not** stored
  - `Tree` — derived at load time by grouping Households on `Parent`. Provides `Roots`, `Children`, `Path`, `PathString`, and `Walk`
  - `Date` — partial precision: a date may know a year only, a year and month, or nothing at all
  - `NormalizePhone`/`NormalizeEmail` and the `Normalize()` methods rewrite values into house
    style; they never fail and never discard input they cannot reformat
  - `ValidateHouseholds` returns `[]Finding` — observations for the UI to display, never rejections
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
- Test data lives in `testdata/directory.json` — three Households (one Branch two levels deep, one standalone root) covering: a birth name, an anniversary, living and deceased Dependents, an `aka`, and multiple address lines. `TestExampleDirectoryIsCanonicallyFormatted` pins this file's byte-level formatting, so hand edits must match its exact indentation and key order or that test fails.
- Use a `checkFunc func(t *testing.T, ...)` field in table rows that need assertions beyond simple field comparisons.

## Notes

- `Person.DisplayName()` renders a nickname as `Given "Aka" Surname`.
- Normalisation is called explicitly by the editing layer, not by `store.Save` — a hand-repaired
  document is loaded and saved exactly as written.

## Agent skills

### Issue tracker

Issues live in GitHub Issues at `asphaltbuffet/wherefolk`, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles use their default label strings. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
