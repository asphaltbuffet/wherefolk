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
go test ./cmd/... -run TestGetPrintCmd

# Run the tool
go run . print <filename.json>
```

## Architecture

The application follows a standard Cobra CLI layout:

- **`main.go`** — entry point, delegates to `cmd.Execute()`
- **`cmd/`** — Cobra commands. Commands are constructed via lazy singleton getters (e.g., `GetPrintCmd()`) rather than `init()`, which allows commands to be instantiated and tested without global side effects.
- **`pkg/rolo/`** — core data model and rendering logic
  - `Family` is a recursive struct (`Children []Family`) representing a household that may contain sub-households. All three rendering methods (`Table`, `Info`, `MakeTree`) walk this tree recursively.
  - `LoadJSON` deserializes a flat `[]Family` (not a `Directory`) from the input file.
  - `Person.Details()` returns `[]string` (used as a table row), not a formatted string.
- **`internal/tui/`** — lipgloss styles used across rendering (colors, `Household`, `Address`, `Anniversary`, `Dead`, `Generation` styles).

## Data Format

Input files are JSON arrays of `Family` objects. See `short.json` for a small example. Each `Family` has `people`, `marriage`, `children` (nested families), and `addresses`. A child entry with only one person and no address/children is treated as a dependent (not an independent household) by `IsFamily()`.

## Testing

- All tests must be table-driven: a `tests []struct{ name string; ... }` slice iterated with `t.Run(tt.name, ...)`.
- Test data lives in `testdata/example.json` — a two-family dataset covering: maiden names, marriage dates, dependents (living and deceased), nested sub-families, multiple addresses, and `aka`.
- Use a `checkFunc func(t *testing.T, ...)` field in table rows that need assertions beyond simple field comparisons.

## Notes

- `MakeTree()` and `Info()` are implemented in `pkg/rolo/family.go` but commented out in `cmd/print.go`.
- `ioutil.ReadFile` in `family.go` is deprecated; prefer `os.ReadFile` when touching that code.
- Cobra commands must write output via `cmd.OutOrStdout()` (e.g. `fmt.Fprintln(cmd.OutOrStdout(), ...)`) — bare `fmt.Println` bypasses `SetOut` and breaks test capture.
- `Person.Aka` renders as `"nickname"` (double-quoted) in the family name header via `familyName()`.

## Agent skills

### Issue tracker

Issues live in GitHub Issues at `asphaltbuffet/wherefolk`, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles use their default label strings. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
