# Add Proof Sheet form fields after Typst, positioned by query

A Proof Sheet is a fillable PDF form, and Typst cannot emit form fields. Typst still lays out the
whole page (labels, notes, the underlines and the Reply Instructions) through a `proof()` entry
point in `template/directory.typ`, and it marks each field's place with a `metadata` element.
`typst query` reads those positions back, and pdfcpu's `api.Create` adds the fields over the
compiled PDF as a library step, in the same way `Encrypt` works (ADR-0011). The page stays an
Operator-editable template (ADR-0004), and the fields can never drift from the layout, because
the template decides where they go.

## Considered Options

- **Lay the whole page out in pdfcpu's JSON.** One tool, but layout would live in Go and JSON
  instead of the template, and the sheet would lose the Directory's fonts and typography.

## Consequences

Underlines are page content drawn by Typst, not field borders, because viewers disagree about
whether form widgets print. Field values are drawn by the viewer in a font pdfcpu declares, so a
character that font cannot encode must refuse the sheet with a sentence rather than print `?`.
Whether pdfcpu can embed the Directory's own font without its config directory is settled by a
spike before implementation.
