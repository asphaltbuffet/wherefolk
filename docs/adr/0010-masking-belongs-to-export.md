# Masking belongs to export, never to the editing UI

§5.5 fixes two kinds of absence. A **Withheld field** — one the Editor marked hidden — renders
`[private]`, so nobody re-collects it next year. A **Suppressed field** — one a tier omits, or any
contact detail of a deceased person — renders as nothing at all, because a marker would advertise
that the data exists. Item 11 implemented both in `internal/web`: `view.go` substituted `[private]`
into the detail pane and blanked a deceased person's phone and email before the template saw them.

That was the wrong layer. Both rules are statements about what an **audience** receives, and the
Editor is not an audience — they are the document's author, the sole writer, working over a
loopback-bound service on a tailnet (ADR-0001, ADR-0007) against a JSON file they can open in an
editor. Masking there protects nothing and costs the ability to work: a field the Editor cannot see
is a field they cannot correct or clear, and item 5 makes the detail pane the editing form, so a
`[private]` placeholder would land *inside an input* and save that literal string as somebody's
phone number.

The editing UI therefore renders every stored value, always. Withholding is expressed as a checkbox
beside a populated input. Suppression does not appear at all: a deceased Dependent's recorded phone
number is shown and editable like any other field.

## Consequences

`hide()` and the deceased-blanking come out of `internal/web`, and the tests that pinned them invert
— they now assert the value *is* shown. This is a deliberate deletion of working, tested behaviour,
not a regression.

The `Private` constant stays, exported, with its §5.5 reasoning attached. Item 8's tier filter is
its only consumer and imports it rather than reinventing the string, so the one place `[private]`
is spelled remains the one place its rationale is written down.

Item 8 inherits both rules whole. That is a larger item than it looked, because the masking it must
implement is richer than what was removed — tiers, age gating, and Truncated Dates all fold into
the same filter — and it no longer has a partial implementation in the web layer to build on. The
rules' real home is §5.5 and CONTEXT.md, which is where they are specified; the twenty lines in
`view.go` were a sketch of them in the wrong place, and carrying that sketch forward into an unused
package would have guessed at a shape item 8 must decide for itself.

The invariant item 11 wrote into `view.go` — that a withheld value must never reach a template —
narrows rather than disappears. It binds the export pipeline absolutely, and no longer binds the
editing pane at all.
