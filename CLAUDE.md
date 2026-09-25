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

# Run the server against the example data, then visit http://127.0.0.1:8099/status
# (bare `go run .` looks for /var/lib/wherefolk/directory.json and exits non-zero)
WHEREFOLK_DATA=./testdata WHEREFOLK_TEMPLATE=./template WHEREFOLK_PORT=8099 go run .

# Render the example Directory to a PDF (needs typst; `nix develop` provides it)
WHEREFOLK_TEMPLATE=./template go test ./internal/render/ -run TestRenderer -v
```

There is no CLI. The binary is a service: `main` loads the store and starts the web server.
An Operator CLI (export, validate, repair) may return as its own work item, built with the
standard library `flag` package. See [docs/design/high-level-design.md](docs/design/high-level-design.md).

## Architecture

- **`main.go`** — entry point: reads config, loads the store, starts the web server
- **`internal/config/`** — environment parsing (`WHEREFOLK_DATA`, `WHEREFOLK_TEMPLATE`, `WHEREFOLK_PORT`, `WHEREFOLK_LOG_LEVEL`)
  - `Config` carries the log *level*; `main` builds the `*slog.Logger` from it and injects it.
    Nothing outside `main` touches slog's package default — constructors take a `*slog.Logger`.
- **`internal/web/`** — HTTP handlers and embedded templates
  - Templates whose basename starts with `_` are **fragments**: parsed into every page's set and
    rendered *without* the layout, because htmx swaps them into a page that is already loaded.
    `render` emits a whole page and takes an HTTP status; `renderFragment` emits one named template.
  - Navigation state lives in the URL, never in a cookie or client state (ADR-0008). `/h/{id}` is
    the selection; `?open=`/`?close=` carry the expanded Branches, `?pane=closed` collapses the
    tree pane.
  - View models in `view.go` hold rendered **strings**, not `rolo` values. A withheld field becomes
    `[private]` and a Memorial Household's contact details are blanked *there*, so a value the
    Editor withheld cannot reach a template and leak through a later markup change.
  - URLs bound for `hx-*` attributes are built with `url.Values` in the view model, never
    concatenated in a template: `html/template` percent-encodes into `href` but not into `hx-get`.
  - The write path is `POST /h/{id}`: parse → refuse-if-unstorable → apply to a **deep clone** →
    normalise → save → swap the clone in → redirect (ADR-0008). A shallow copy would share every
    Household's `Adults`, `Dependents` and `Address.Lines`, so the clone must be deep or a failed
    save corrupts the served Directory.
  - `commit` holds the write lock for the mutation and renders nothing; `handleSave` holds no lock
    and turns the outcome into a response; `refuse` takes its own read lock and releases it before
    rendering. Rendering under the write lock would let one Editor on a slow connection block every
    reader for the length of the response.
  - **Two failure modes, deliberately different.** Input that cannot be stored — a date that will
    not parse — refuses the submit with **422** and re-renders the form from the Editor's own
    submission, so their typing survives; it is *not* a `rolo.Finding`. A `Finding` observes a value
    that *is* stored and never blocks a save (§4.5).
  - **Every field of every person must render as an input.** `parseSubmission` reads fields with
    `url.Values.Get`, which cannot tell a field submitted empty from one absent, so a field the
    template stops rendering would silently clear itself on the next save.
    `TestFormRendersEveryEditableField` guards this. Each checkbox needs its paired hidden `off`
    input for the same reason — without it a hidden flag could never be turned back off.
  - `store.Save` is reached through an injected `Saver`, and IDs through injected generators, so
    this package keeps no filesystem dependency and tests get deterministic identities.
  - **The editing UI never masks** (ADR-0010). `[private]` and deceased-contact suppression belong
    to export; `view.go` renders every stored value.
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
- **`internal/render/`** — the Directory as a printed document
  - `Directory`/`Household`/`Person` in `model.go` hold rendered **strings**, not `rolo` values,
    for the same reason `web/view.go` does: a tier rule cannot be forgotten about a value that
    never arrives here as a date or a flag.
  - **This package applies no tier rules.** No `[private]`, no truncation, no age computation.
    `Build` copies every stored value through as written; item 8's tier filter replaces that
    constructor rather than wrapping it, so withholding and suppression live in exactly one place.
  - `escape` handles Typst's markup characters (`#`, `@`, `*`, `_`, `[`, `$`, `<`, `\`) at every
    interpolation point. Go assembles markup by hand with no template engine auto-escaping it, so
    an address line reading `#1 Elm St` would otherwise parse as a Typst code expression.
  - `Markup` emits **data and calls into the template's functions**, never a margin or a font.
    Every layout decision is in `template/directory.typ`.
  - `Typst.CompilePDF`/`CompileSVG` stage the template and generated source into a scratch
    directory, because the generated markup imports `directory.typ` as a sibling. They are separate
    methods because Typst's CLI is asymmetric: a PDF is one file, an SVG export is **one file per
    page** and needs a `{p}` placeholder in the output name, so `SVG` returns `[][]byte`.
- **`internal/buildmeta/`** — build metadata (`Version`, `GitCommit`, `BuildDate`) injected via ldflags.
  The package is deliberately not named `version` or `buildinfo`: both collide with stdlib
  (`go/version`, `debug/buildinfo`). The ldflag paths in `mise.toml` and `.goreleaser.yml` are
  strings, so renaming this package silently stops injection unless they are updated too.

## Data Format

`testdata/directory.json` is the canonical example. A document is an object with a `schema`
version and a flat `households` array — **not** a nested tree. Each Household carries its own `id`
and an optional `parent`; the hierarchy is rebuilt from those links by `rolo.BuildTree`.

Sibling Households are ordered by their eldest adult's birth date, derived rather than stored.

See `CONTEXT.md` for the domain vocabulary and `docs/adr/` for the decisions behind this shape.

## Testing

- All tests must be table-driven: a `tests []struct{ name string; ... }` slice iterated with `t.Run(tt.name, ...)`.
- Test data lives in `testdata/directory.json` — **four Households and ten people**: a Memorial root
  (`h_meml01`, both adults deceased) anchoring a Branch two levels deep, plus one standalone root.
  Between them they cover a birth name, an anniversary, living and deceased Dependents, an `aka`,
  multiple address lines, and a Shared Address (`h_lang02` points at `h_lang01`). The Memorial and
  Shared Address cases exist so the canonical example exercises `internal/render`'s two special
  paths. `TestExampleDirectoryIsCanonicallyFormatted` pins this file's byte-level formatting, so
  hand edits must match its exact indentation and key order or that test fails — re-pin by running
  the document back through `store.Save` rather than hand-matching, since `Save` emits Go struct
  field order and no formatter can infer it.
- Use a `checkFunc func(t *testing.T, ...)` field in table rows that need assertions beyond simple field comparisons.
- Tests that invoke `typst` call `requireTypst(t)`, which **skips** when the binary is absent.
  Typst is a host dependency (ADR-0004) provided by the devShell in `flake.nix`, so inside
  `nix develop` (or any shell with `typst` on `PATH`) `go test ./...` runs everything, and outside
  it exactly seven render tests skip rather than fail. A skipped render test is not a passing one —
  check the output for `SKIP` before believing the pipeline works.

## Notes

- `Person.DisplayName()` renders a nickname as `Given "Aka" Surname`. `html/template` escapes those
  quotes in rendered HTML, so tests asserting on output must expect `&#34;`, not `"`.
- `rolo.SearchPeople` is a linear scan, deliberately: an index would need invalidating on every edit
  once item 5 makes the document mutable, and a few hundred people scan instantly.
- **Two kinds of absence, and they must render differently** (§5.5). A *withheld* field — one the
  Editor marked hidden — renders `[private]`, so nobody re-collects it next year. A *suppressed*
  field — one a tier omits, or any contact detail of a deceased person — renders as nothing at
  all, because a marker would advertise that the data exists.
  Both rules apply to **export only** — the editing UI shows every stored value, because the Editor
  is the document's author rather than one of its audiences (ADR-0010).
- Normalisation is called explicitly by the editing layer, not by `store.Save` — a hand-repaired
  document is loaded and saved exactly as written.
- **`template/directory.typ` is not embedded**, deliberately (ADR-0004). It is an Operator
  affordance: "the addresses look cramped" is a file edit and a restart, not a rebuild. Nothing in
  `internal/render` may start embedding it without revisiting that ADR.

## Agent skills

### Issue tracker

Issues live in GitHub Issues at `asphaltbuffet/wherefolk`, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles use their default label strings. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.
