// Wherefolk Directory layout.
//
// This file is deliberately NOT embedded in the binary (ADR-0004): adjusting
// spacing or type is an edit here plus a restart, not a rebuild and redeploy.
// The Go side emits data and calls these functions (directory, households,
// household, memorial, birthdays); it sets no margins, fonts or spacing of its
// own, so every layout decision — and every word of section copy — is in this
// file.
//
// Layout is not customisable (§5.1). There is one typeface, one set of margins
// and one block format, on purpose: the Editor wants a directory that looks
// right, not a document to design.

// title-page is page 1 of every Directory (CONTEXT.md, Title page). It states
// the Directory Title and the facts that let a reader judge a copy at a
// glance; the footer prints DO NOT DISTRIBUTE beneath it for Full.
#let title-page(title, generated, tier) = {
  align(center + horizon, {
    text(size: 28pt, weight: "bold", title)
    v(1.2em)
    text(size: 11pt, fill: luma(80), "Generated " + generated)
    if tier != "" {
      linebreak()
      text(size: 11pt, fill: luma(80), tier + " tier")
    }
  })
  pagebreak()
}

// contents-page is the Table of Contents (CONTEXT.md). Level 1 is a section;
// level 2 is a first-generation Branch, whose heading household() hides.
#let contents-page() = {
  outline(title: "Contents", depth: 2)
  pagebreak(weak: true)
}

#let directory(title: "", generated: "", tier: "", restricted: false, body) = {
  set document(title: title)
  set page(
    paper: "us-letter",
    margin: (x: 2cm, y: 2cm),
    footer: context [
      #set text(size: 8pt, fill: luma(100))
      #if restricted {
        text(weight: "bold", fill: luma(0), "DO NOT DISTRIBUTE")
        h(0.8em)
      }
      // The Title page counts as page 1 but states its tier and date in its
      // body, so its footer carries neither them nor a page number.
      #if counter(page).get().first() > 1 [
        #if tier != "" { tier + " tier · " }Generated #generated
        #h(1fr)
        #counter(page).display("1 of 1", both: true)
      ]
    ],
  )

  set text(font: ("Libertinus Serif", "DejaVu Serif"), size: 10pt)
  set par(justify: false)

  // A section starts on a fresh page under a plain bold heading. The
  // Table of Contents' own title is a level-1 heading too, and takes the same
  // look.
  show heading.where(level: 1): it => {
    pagebreak(weak: true)
    text(size: 14pt, weight: "bold", it.body)
    v(0.6em)
  }

  title-page(title, generated, tier)
  contents-page()
  body
}

// households is the section holding every Household block.
#let households(body) = {
  heading(level: 1, "Households")
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

// target marks where a Household's block begins, so the Birthday Calendar can
// link to it and print its page. id is the Household's stored ID; as a label
// it is never printed and does not appear in the PDF. metadata is invisible
// and takes no space, so the block looks exactly as it did without it.
#let target(id) = [#metadata(none)#label(id)]

// household is one block of the Directory.
//
// breakable: false is the whole reason this project renders through Typst
// rather than a Go PDF library (ADR-0004): a Household is never split across a
// page break, and Typst does that pagination itself.
#let household(
  id: "",
  name: "",
  anniversary: "",
  address: (),
  shared: "",
  adults: (),
  dependents: (),
  contents: false,
) = {
  block(breakable: false, width: 100%, inset: (y: 0.4em), {
    target(id)
    // A first-generation Branch is listed in the Table of Contents through a
    // heading that takes no space and prints nothing: the block must look like
    // every other Household, because Branches are never nested in the rendered
    // output (ADR-0002). place() keeps it out of the flow; hide() keeps it
    // locatable, so the outline still knows its page.
    if contents { place(hide(heading(level: 2, name))) }
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
  id: "",
  name: "",
  anniversary: "",
  address: (),
  shared: "",
  adults: (),
  dependents: (),
  contents: false,
) = {
  block(breakable: false, width: 100%, inset: (y: 0.3em), {
    target(id)
    // Listed in the Table of Contents as household() explains.
    if contents { place(hide(heading(level: 2, name))) }
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

// birthdays is the Birthday Calendar: every living person, surname first,
// against the twelve months, with the day of their birthday in its month.
//
// Who appears, what each name says and the order were all decided in Go; this
// only lays the rows out. It is its own section, so it starts on a fresh page
// and is listed in the Table of Contents.
// The month header repeats on every page, so page five still says which column
// is Sep. Alternate rows are shaded so the eye can follow a row from name to
// day. A row is never split across a page break; a surname group may be.
//
// At the right of each name cell is the page the person's Household block
// begins on, as "p. 14", so a reader can turn to the address; in a PDF viewer
// the name and the page both jump there. The page is the footer's own counter
// at the block's target, so the two always agree. The "p." keeps a bare number
// beside a name from reading as an age, and the italic keeps it quieter than
// the name. A row's household with no target is a compile error, never an
// unlinked row.
#let birthdays(rows) = {
  let months = ("Jan", "Feb", "Mar", "Apr", "May", "Jun",
                "Jul", "Aug", "Sep", "Oct", "Nov", "Dec")

  heading(level: 1, "Birthdays")

  set table.cell(breakable: false)
  table(
    columns: (1fr,) + (2.4em,) * 12,
    inset: (x: 4pt, y: 3pt),
    align: (x, _) => if x == 0 { left } else { center },
    stroke: (x, y) => (
      left: if x > 0 { 0.4pt + luma(170) } else { none },
      bottom: if y == 0 { 0.6pt + luma(60) } else { none },
    ),
    fill: (_, y) => if y > 0 and calc.even(y) { luma(242) },
    table.header(
      repeat: true,
      [],
      ..months.map(m => text(size: 9pt, weight: "semibold", m)),
    ),
    ..rows.map(r => {
      let home = label(r.household)
      (
        text(size: 9pt, {
          link(home, r.name)
          h(1fr)
          link(home, emph(context "p. " + str(counter(page).at(home).first())))
        }),
        ..range(1, 13).map(m => text(size: 9pt, if m == r.month { r.day } else { "" })),
      )
    }).flatten(),
  )
}
