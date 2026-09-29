// Wherefolk Directory layout.
//
// This file is deliberately NOT embedded in the binary (ADR-0004): adjusting
// spacing or type is an edit here plus a restart, not a rebuild and redeploy.
// The Go side emits data and calls three of these functions (directory,
// household, memorial); it sets no margins, fonts or spacing of its own, so
// every layout decision is in this file.
//
// Layout is not customisable (§5.1). There is one typeface, one set of margins
// and one block format, on purpose: the Editor wants a directory that looks
// right, not a document to design.

#let directory(generated: "", tier: "", restricted: false, body) = {
  set page(
    paper: "us-letter",
    margin: (x: 2cm, y: 2cm),
    footer: context [
      #set text(size: 8pt, fill: luma(100))
      #if restricted {
        text(weight: "bold", fill: luma(0), "DO NOT DISTRIBUTE")
        h(0.8em)
      }
      #if tier != "" { tier + " tier · " }Generated #generated
      #h(1fr)
      #counter(page).display("1 of 1", both: true)
    ],
  )

  set text(font: ("Libertinus Serif", "DejaVu Serif"), size: 10pt)
  set par(justify: false)

  body
}

// row-tracks is every person row's column layout: name, dates, phone, email.
//
// It is one value for the whole Directory, on purpose. Each Household is its
// own grid, and Typst sizes auto tracks per grid, so auto columns would move
// from block to block; fixed tracks keep the phone column where the reader's
// eye learned to find it. The dates track fits a Whole-date range with
// abbreviated months ("Sep 30, 1928 – Sep 30, 2011"), the widest date a row
// holds; anything longer wraps inside its cell rather than widening every row.
#let row-tracks = (9em, 13em, 8em, 1fr)

// lifespan is a row's dates cell.
//
// A deceased person's dates are one range, the headstone convention; with no
// birth date it is the death alone. A living person's birth date carries "b.".
// This only composes strings Build already filtered: which dates exist, and
// whether they are Truncated or Whole, was decided there (§5.3).
#let lifespan(p) = {
  if p.death != "" {
    if p.birth != "" { p.birth + " – " + p.death } else { "d. " + p.death }
  } else if p.birth != "" {
    "b. " + p.birth
  } else {
    ""
  }
}

// people-grid renders people as aligned rows, one per person.
//
// Every field is printed only when non-empty. An empty string means "nothing to
// print" and never means "suppressed" — suppression happens before the data
// reaches this file, and prints nothing at all by design (§5.5). An empty cell
// still occupies its track, so the columns never shift.
#let people-grid(people, ink: luma(0)) = grid(
  columns: row-tracks,
  column-gutter: 0.8em,
  row-gutter: 0.5em,
  ..people.map(p => (
    text(weight: "semibold", fill: ink, p.name),
    text(size: 9pt, fill: luma(80), lifespan(p)),
    text(size: 9pt, fill: ink, p.phone),
    text(size: 9pt, fill: ink, p.email),
  )).flatten(),
)

// household is one block of the Directory.
//
// breakable: false is the whole reason this project renders through Typst
// rather than a Go PDF library (ADR-0004): a Household is never split across a
// page break, and Typst does that pagination itself.
#let household(
  name: "",
  anniversary: "",
  address: (),
  shared: "",
  adults: (),
  dependents: (),
) = {
  block(breakable: false, width: 100%, inset: (y: 0.4em), {
    text(size: 12pt, weight: "bold", name)

    if address.len() > 0 {
      linebreak()
      text(size: 9pt, address.join(linebreak()))
    } else if shared != "" {
      linebreak()
      text(size: 9pt, style: "italic", fill: luma(80), "Address: see " + shared)
    }

    if anniversary != "" {
      linebreak()
      text(size: 9pt, fill: luma(80), "Anniversary " + anniversary)
    }

    // No "Dependents" label: in a grid a label is a row with no cells, and
    // order already says who is who. A gap keeps the grouping visible.
    v(0.4em)
    people-grid(adults)

    if dependents.len() > 0 {
      v(0.6em)
      people-grid(dependents)
    }
  })

  v(0.5em)
}

// memorial is a Household whose adults have all died.
//
// It renders more quietly than a live Household — grey, with the anniversary
// as "m." under the heading — because there is nothing in it to act on. It must
// still appear in every tier so that descendants group beneath it and no Path
// points at a node missing from the document (§5.4).
#let memorial(
  name: "",
  anniversary: "",
  address: (),
  shared: "",
  adults: (),
  dependents: (),
) = {
  block(breakable: false, width: 100%, inset: (y: 0.3em), {
    text(size: 11pt, weight: "bold", fill: luma(60), name)

    if anniversary != "" {
      linebreak()
      text(size: 9pt, fill: luma(100), "m. " + anniversary)
    }

    // The same tracks as a live Household, so the Memorial reads as part of
    // one table; its phone and email cells are empty, because Build suppresses
    // a deceased person's contact details (§5.4).
    v(0.3em)
    people-grid(adults, ink: luma(60))

    if dependents.len() > 0 {
      v(0.5em)
      people-grid(dependents, ink: luma(60))
    }
  })

  v(0.5em)
}
