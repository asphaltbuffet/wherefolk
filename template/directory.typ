// Wherefolk Directory layout.
//
// This file is deliberately NOT embedded in the binary (ADR-0004): adjusting
// spacing or type is an edit here plus a restart, not a rebuild and redeploy.
// The Go side emits data and calls these three functions; it sets no margins,
// fonts or spacing of its own, so every layout decision is in this file.
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

// contact renders one person's line: name, dates, then contact details.
//
// Every field is printed only when non-empty. An empty string means "nothing to
// print" and never means "suppressed" — suppression happens before the data
// reaches this file, and prints nothing at all by design (§5.5).
#let contact(p) = {
  let dates = ()
  if p.birth != "" { dates.push("b. " + p.birth) }
  if p.death != "" { dates.push("d. " + p.death) }

  let reach = ()
  if p.phone != "" { reach.push(p.phone) }
  if p.email != "" { reach.push(p.email) }

  block(breakable: false, {
    text(weight: "semibold", p.name)
    if dates.len() > 0 {
      h(0.6em)
      text(size: 9pt, fill: luma(80), dates.join(" · "))
    }
    if reach.len() > 0 {
      linebreak()
      h(1.2em)
      text(size: 9pt, reach.join(" · "))
    }
  })
}

// household is one block of the Directory.
//
// breakable: false is the whole reason this project renders through Typst
// rather than a Go PDF library (ADR-0004): a Household is never split across a
// page break, and Typst does that pagination itself.
#let household(
  label: "",
  anniversary: "",
  address: (),
  shared: "",
  adults: (),
  dependents: (),
) = {
  block(breakable: false, width: 100%, inset: (y: 0.4em), {
    text(size: 12pt, weight: "bold", label)

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

    for a in adults {
      v(0.25em)
      contact(a)
    }

    if dependents.len() > 0 {
      v(0.3em)
      text(size: 8pt, fill: luma(120), smallcaps("Dependents"))
      for d in dependents {
        v(0.15em)
        contact(d)
      }
    }
  })

  v(0.5em)
}

// memorial is a Household whose adults have all died.
//
// It renders more compactly than a live Household — a heading with dates rather
// than a full entry — because there is nothing in it to act on. It must still
// appear in every tier so that descendants group beneath it and no Path points
// at a node missing from the document (§5.4).
#let memorial(
  label: "",
  anniversary: "",
  address: (),
  shared: "",
  adults: (),
  dependents: (),
) = {
  block(breakable: false, width: 100%, inset: (y: 0.3em), {
    text(size: 11pt, weight: "bold", fill: luma(60), label)

    for a in adults {
      linebreak()
      h(0.8em)
      text(size: 9pt, a.name)
      let dates = ()
      if a.birth != "" { dates.push(a.birth) }
      if a.death != "" { dates.push(a.death) }
      if dates.len() > 0 {
        h(0.5em)
        text(size: 9pt, fill: luma(100), "(" + dates.join(" – ") + ")")
      }
    }

    if anniversary != "" {
      linebreak()
      h(0.8em)
      text(size: 9pt, fill: luma(100), "m. " + anniversary)
    }

    for d in dependents {
      linebreak()
      h(0.8em)
      text(size: 9pt, d.name)
    }
  })

  v(0.5em)
}
