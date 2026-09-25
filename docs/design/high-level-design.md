# Wherefolk — High-Level Design

**Status:** draft, 2026-09-22
**Scope:** system shape and UI/UX. Implementation detail is deliberately absent; this document
exists to be split into implementation sections.

Terms used here are defined in [`CONTEXT.md`](../../CONTEXT.md). Decisions marked (ADR-N) are
recorded in [`docs/adr/`](../adr/).

---

## 1. Purpose and users

Wherefolk maintains an **address book for an extended family** and publishes it as a printable,
shareable **Directory**. Contact information is the primary job; important dates (birth, marriage,
death) and legible family relationships are secondary.

There are two people in this system, with very different needs:

| Role | Who | Needs |
|---|---|---|
| **Editor** | A ~70-year-old non-technical relative | To find a person quickly, correct a detail, and produce a document to send. Never sees a terminal, a file path, or an error code. |
| **Operator** | The developer | Full administrative access to the host, the ability to repair data by hand, and a system that does not generate support calls. |

The Editor is not the developer. Every usability decision below follows from that.

### Non-goals

- **Multi-user editing.** One Editor. No accounts, no permissions, no merge conflicts.
- **Phone and tablet layouts.** Desktop only; a phone UI is a separate design effort with a
  different shape. This licenses layouts that do not degrade gracefully.
- **Genealogy.** This is an address book that happens to be tree-shaped, not a family-history tool.
  No sources, no uncertainty, no GEDCOM.
- **Non-text information.** No photographs, scans, audio, or attachments — permanently, not as a
  deferred feature. Images would roughly double the project's surface area (blob storage alongside
  the JSON file, server-side normalisation to keep output uniform, EXIF stripping, a difficult
  upload interaction for this Editor) in service of a keepsake rather than an address book. A family
  yearbook is a different product and deserves its own design.
- **Inbound corrections.** Relatives never write to the Directory; see §5.6.
- **Public hosting.** See §2.

---

## 2. System shape

A single Go binary serving a web UI, hosted on Operator-controlled hardware, reached by the Editor
over a **Tailscale** mesh network (ADR-0001).

```
  Editor's machine                    Operator's host
  ┌──────────────┐                    ┌────────────────────────┐
  │   Browser    │ ─── Tailscale ───▶ │  wherefolk (Go binary) │
  │  (bookmark)  │                    │    ├── web UI          │
  └──────────────┘                    │    ├── export engine   │
                                      │    └── JSON store      │
                                      └────────────────────────┘
                                               │
                                          nightly snapshots
```

Consequences of this shape:

- **No authentication subsystem.** The network authenticates; the app treats every request as
  trusted. The binary binds to the Tailscale interface, not `0.0.0.0`, so this holds by
  construction.
- **No installation on the Editor's machine.** Their experience is a bookmark. OS-agnosticism comes
  free — it is a browser.
- **No server lifecycle for the Editor.** They never start, stop, or update anything.
- **The Operator owns recovery.** No remote troubleshooting of someone else's machine.

### 2.1 Packaging and deployment

The artifact is a **Docker image**, not a host installation. The Operator runs NixOS, but a
container isolates the service from host configuration changes and — more valuably — pins Typst's
version inside the image, so a host upgrade can never silently reflow the Directory.

```
  ┌─ docker compose ──────────────────────────────────┐
  │                                                   │
  │  ┌─ tailscale (official image) ─┐                 │
  │  │  tailscaled + Serve (HTTPS)  │◀── tailnet      │
  │  │  volume: /var/lib/tailscale  │                 │
  │  └──────────────┬───────────────┘                 │
  │                 │ network_mode: service:tailscale │
  │  ┌─ wherefolk ──┴───────────────┐                 │
  │  │  Go binary, :8080 localhost  │                 │
  │  │  Typst (pinned)              │                 │
  │  │  volume: /var/lib/wherefolk  │                 │
  │  └──────────────────────────────┘                 │
  └───────────────────────────────────────────────────┘
```

**Base image:** Debian slim, with Typst installed from its official release tarball at a pinned
version and checksum, fetched in a builder stage so that neither `curl` nor the tarball reaches the
final image.

The original rationale here — "Alpine is avoided because Typst ships glibc binaries and musl is a
real risk" — no longer holds: as of 0.14.2 upstream publishes **only** musl builds for amd64 and
arm64, and they are `static-pie` linked, so they carry no libc dependency and run unchanged on
Debian. Debian slim is kept for the reason that survives — it is an ordinary base the Operator can
shell into and add a font to — not for libc compatibility.

Typst embeds `Libertinus Serif`, the template's primary typeface, so the Directory renders
correctly with no system fonts installed. A template that reaches for a font Typst does not embed
would need one added to the image; the template's `DejaVu Serif` fallback is currently unresolved
and unused, which Typst reports as a warning rather than an error.

**Frontend:** server-rendered Go templates with htmx, embedded in the binary via `embed`. No
JavaScript build step, no `node_modules`, no separate asset serving — the interaction budget here
is a tree, a form, and an export dialog. Hand-written JavaScript is reserved for tree
expand/collapse, which should feel instant — but item 4 shipped without any: the toggles are
ordinary links whose state lives in the URL (ADR-0008), so the licence is still unspent and the
only script served is htmx itself. This keeps the supply-chain surface near zero, which matters for
an application whose defining constraint is privacy.

**Nix** remains the development environment (pinned Go toolchain, `typst`), not the deployment
mechanism. GoReleaser is dropped: its value is cross-platform release archives for many users, and
this has one target and one user who downloads nothing.

**Volumes:** the JSON store and Tailscale's node state are both named volumes. Tailscale state
*must* persist — without it every restart authenticates as a new node, the tailnet fills with
`wherefolk-1`, `wherefolk-2`, and the Editor's bookmark stops resolving.

**Schema migration** runs at startup: the store carries a `schema` version, the app snapshots and
applies migrations up to its own version, and **refuses to start if the data is newer than the
binary understands**. That last case is a rollback, where silently truncating unknown fields would
be the worst bug in the system. Deployment is therefore always "pull the image and restart," with
no manual step to forget.

The snapshot taken before a migration is written to `snapshots/` beside the document, named for the
version it holds and the time it was taken — `snapshots/pre-migrate-v1-20260922.json`. Work item 6's
nightly snapshots share that directory.

**The runner itself is deferred until a second schema version exists.** Schema 1 is the only version
there has ever been: `Load` already refuses anything newer, and a migration chain with no migrations
in it is machinery built against a guess at a change that has not happened. The seam it will occupy
is already in `Load` — between the version probe and the decode, so a document whose shape predates
the current `Document` struct is migrated as bytes before anything tries to unmarshal it.

### 2.2 Tailscale configuration

**Sidecar, not host.** The official `tailscale/tailscale` image runs `tailscaled`; the application
container joins its network namespace. This keeps the app container to a single process, where
Docker's restart policies, health checks, and log handling actually work, and it avoids running a
process supervisor inside an application image.

**`tailscale serve`, never `funnel`.** Serve publishes to the tailnet only, with a genuine
Let's Encrypt certificate for `https://wherefolk.<tailnet>.ts.net`, so the Editor sees a padlock and
no browser warnings — a "Not secure" badge on a page of family addresses is precisely the thing that
generates an alarmed phone call. **Funnel is the one-word difference that would publish the
Directory to the open internet**, so the deployment asserts it is disabled rather than merely not
enabling it.

The public certificate means the *name* `wherefolk.<tailnet>.ts.net` appears in Certificate
Transparency logs. Nothing about it is reachable or reveals contents, but the host's existence is
publicly observable.

**Authentication is an OAuth client with `tag:wherefolk`**, not a raw auth key. Auth keys expire at
90 days, which would mean the node silently fails to come up one morning next quarter; OAuth clients
mint node keys indefinitely and bind the node to a tag.

**An ACL restricts `tag:wherefolk` to the Operator's and Editor's devices.** Tailnet membership is
otherwise coarse — every device on the tailnet could reach the service, including machines added
later for unrelated reasons. This is the layer where "privacy is built-in" is actually enforced.

**Debugging** happens via `docker exec` or a temporarily published port. The app always binds
loopback — the interface is a package constant, not a setting — so no code path or configuration
value can expose it beyond the host and nothing can be left switched on by accident. The port is
configurable via `WHEREFOLK_PORT` (default `8080`) because a collision in the shared network
namespace is plausible; splitting the address this way keeps the security-bearing half out of the
environment (ADR-0007).

### 2.3 Secrets

Three: the Tailscale OAuth client, the two Healthchecks.io ping URLs, and the Full-tier passphrase.

They live in a `.env` file on the host, outside git, referenced by the compose file and managed by
**agenix** — the mechanism the Operator already runs. Baking them into the image would leak them
into any registry; Docker secrets would be over-engineering for a single-host compose deployment.

The passphrase is a different kind of secret from the other two — shared with relatives by phone,
rotated rarely, and protecting an exported artifact rather than infrastructure. It stays
Operator-managed nonetheless: a passphrase the Editor can edit is one they can accidentally blank.

### 2.4 Health and monitoring

The Editor uses this a few times a month, so an unnoticed outage could last until they call. Two
mechanisms, catching different failures:

**`/healthz` with a Docker `HEALTHCHECK`** verifies the store is readable and Typst is present and
version-compatible. This catches a wedged process *from inside* and restarts it.

`/status` is its human-readable companion, built in item 11: the same facts — schema version,
document path, Household and Person counts, Typst availability — rendered for an Operator who is
looking into a problem rather than for a probe deciding whether to restart. It is Operator-facing
diagnostics, deliberately not part of the Editor's two-pane interface (§4.1).

**Healthchecks.io**, which the Operator already runs, catches what an internal check cannot: a dead
container cannot report that it is dead. It is a dead-man's switch — the job pings outward on
success and the *absence* of a ping raises the alert — which is the only shape that works here,
since nothing outside the tailnet can reach `/healthz` to poll it.

| Check | Pinged | Grace | Means |
|---|---|---|---|
| `wherefolk-snapshot` | On verified snapshot completion, daily | Hours | Backups have stopped |
| `wherefolk-alive` | Hourly by the app, only if the store reads and Typst answers | Generous | Service genuinely down, not merely restarting |

Snapshot failures ping `/fail` so the Operator learns *why*, not merely *that*.

Two details that decide whether this monitoring means anything:

- **The snapshot check verifies, it does not merely write.** A job that writes a zero-byte file
  every night would ping happily for six months and leave nothing to restore. The snapshot is read
  back and confirmed to parse as JSON with a plausible person count before the ping is sent.
- **Ping URLs are secrets**, in the same category as the OAuth client — anyone holding one can
  silence the alerts. They live in the secrets mechanism, never in the compose file in git.

---

## 3. Data model

### Shape

Stored **flat**: every Person and Household is a record with a stable internal ID and a parent
reference. The hierarchy is derived at load time (ADR-0002).

- A **Person** has name, optional nickname, optional birth name, dates, phone, email, and
  per-field `hidden` flags.
- A **Household** has one or two adult Persons, an optional Anniversary, an optional Address, zero
  or more **Dependents**, and a parent Household reference.
- A node is a Household when it has a spouse, has Dependents, or has its own Address. Otherwise it
  is a Dependent listed inside its parent's Household.

**Death is a property of a Person, never of a Household.** A Household persists as long as it has a
surviving adult or any descendants, keeping its name and Path — `Clyde/Doris` stays the navigation
label forever, because that is how the family refers to that branch. The Address belongs to the
surviving adult; when the last adult dies the Household becomes a **Memorial Household**, carrying
names and dates but no contact details, and is never deleted, because it anchors the Path of every
Branch beneath it.

**Promotion is structural, not declared.** Adding a spouse to Dave *is* the act of making Dave a
Household; there is no checkbox. The UI announces the change when it happens (§4.4) so it is never
a surprise.

**Shared Address** is a reference, not a copy: a Household living at its parent's address points at
it, stays in sync when the parent moves, and renders as a back-reference rather than a repeated
block.

### Storage

A single JSON file, written atomically (temp file → fsync → rename). At a few hundred people this
is tens of kilobytes.

Chosen over SQLite because there is exactly one writer, and because a plain JSON file preserves the
Operator's emergency repair path and outlives the application — it is readable in twenty years
whether or not this code still compiles. It is also diffable, which makes snapshots cheap to review.

### Safety net

Three layers, targeting the two accidents that actually happen:

1. **Undo** for the current editing session — covers fat-fingering, noticed immediately.
2. **Trash** — deleted Households are recoverable for 30 days rather than vanishing. Covers the
   deletion noticed a fortnight later.
3. **Nightly snapshots** on the host, retained for a year. The Operator's backstop, covering disk
   failure and the Operator's own maintenance mistakes.

Full version history is deliberately excluded: it sounds responsible, absorbs a large share of the
project, and its "what changed" UI is hard to make legible to this Editor.

---

## 4. The editing interface

### 4.1 Layout

Two panes, filesystem-style. The tree pane is collapsible.

```
┌──────────────────────┬────────────────────────────────────────┐
│ ▾ Aden / Nettie      │  Dave & Diane Whitlock                 │
│   ▾ Clyde / Doris    │  Aden/Nettie › Clyde/Doris › Dave/Diane│
│     ▸ Dave / Diane   │  ────────────────────────────────────  │
│       Marie / Tom    │  Address    1412 Oak St                │
│     ▸ Susan / Ray    │             Springfield, IL 62704      │
│   ▸ Harold / June    │  Anniversary  June 14, 1998            │
│                      │                                        │
│ [ Search… ]          │  Dave Whitlock    b. 1971-03-02        │
│                      │    555-0142 · dave@example.com         │
│                      │  Diane Whitlock   b. 1973-08-19        │
│                      │    555-0143 · [private]                │
│                      │  ─ Dependents ─                        │
│                      │  Ellie   b. 2009-05-11                 │
└──────────────────────┴────────────────────────────────────────┘
```

The **Path is always visible** as a breadcrumb in the detail pane. It is the disambiguation
mechanism; hiding it would undercut the reason for tree navigation in the first place.

### 4.2 Navigation

The tree is primary. The Editor finds Dave by walking `Aden/Nettie › Clyde/Doris › Dave/Diane`,
which identifies him unambiguously among five Daves in a way a name search cannot.

### 4.3 Search as a shortcut *through* the tree

Search is secondary and must not become an alternative interface. Results carry their Path as
context:

```
  Dave Whitlock    Aden/Nettie › Clyde/Doris › Dave
  Dave Whitlock    Harold/June › Dave
  Dave Reeves      Aden/Nettie › Susan/Ray › Dave
```

The Path shown is the **whole** chain, including the person's own Household, not just their
ancestry. Two brothers who each head a Household under the same parents would otherwise render
identical context lines; including the leaf distinguishes them.

Selecting a result **expands the tree to that node and selects it** — it does not open a detached
editor. Search relocates the Editor within their mental model rather than bypassing it.

### 4.4 Editing

- **The detail pane is the form.** There is no view mode and no Edit button: the pane always renders
  its fields as inputs, and the Editor edits where they read. There is one Directory and no
  "save as".
- **One Household, one Save.** The pane submits as a whole, so a change that restructures the tree
  — promoting a Dependent, adding or removing a person — is validated and announced once, rather
  than firing halfway through the Editor's typing. Navigation controls (the Path breadcrumb, the
  tree) sit outside the form so following a link is never a submit.
- **Promotion is asked for, on the Dependent's row.** The pane governs one Household, so it cannot
  grow a nested form for each Dependent's future spouse. The Editor marks the Dependent for a
  household of their own; the new Household's own pane is where the spouse and Address are then
  filled in. The trigger is still structural (§3) — what makes Dave a Household is that he now has
  one — and nothing demotes (ADR-0009).
- **Normalise on save, then display the normalised value.** `555.201.0001` becomes `555-201-0001`
  in front of the Editor, so they see the correction and absorb the house style without being
  scolded. Input is forgiving; storage is consistent.
- **Structural changes are announced.** Adding Diane as Dave's spouse moves Dave out of his
  parents' block into his own. The UI says so plainly, names where he went, and links there.
  The undo beside that sentence is item 6's: item 5 ships the announcement with a slot for the
  control, and the Safety net fills it. Until then the Editor's recovery is the nightly snapshot,
  which is why the announcement must name the change precisely enough to reverse by hand.
- **Per-field hidden.** Any field can be marked hidden — not just whole people. Per-person exclusion
  is too blunt to get used; the realistic request is "my address stays out, my name is fine."
- **The editing UI never masks.** Hidden is a checkbox beside a populated input, not a replaced
  value: withholding and suppression are statements about *export*, and the Editor is the
  document's author rather than one of its audiences. A value the Editor cannot see is one they
  cannot correct or clear, so `[private]` and deceased-contact suppression (§5.5) belong to the
  tier filter alone and never to this pane.

### 4.5 Validation

Forgiving, and phrased as observation rather than rejection. Dates, phone numbers, and email
addresses are checked; failures explain rather than block. A missing birthdate is not an error — it
is a fact with an export consequence (§5.3), surfaced at export time rather than nagged about
during editing.

A Finding's message is **displayed verbatim to the Editor**, so it is written for them: plain
English, no library internals, and a hint at what to change. "Check for a missing @ or a stray
space" is the register; a parser's own error text is not.

**Questionable data and unstorable input are different, and only one of them is a Finding.** A
Finding is an observation about a value that *is* stored: an unrecognised phone number prints as
written, a death before a birth is a warning, and neither stops a save. Input that cannot be
represented at all — `June-ish 1998` in a date field, which yields no `Date` — is not a Finding,
because there is nothing to observe. That submit is refused, the pane re-renders with every box
exactly as the Editor left it, and the offending field carries a plain-English explanation. This
is the more forgiving reading of §4.4: a save that silently dropped the words the Editor typed
would discard their work to preserve a rule about not blocking.

**There are two validation passes, and they want opposite answers about absence.** The
editing-time pass (`ValidateHouseholds`) stays silent about every empty field, because nagging
about data that is merely incomplete trains the Editor to ignore findings. The export pre-flight
(§5.7) reports exactly what the editing pass suppresses — a missing birthdate means that person's
contact details will be withheld, which the Editor must see *before* sending the Directory out.
Same data, same question, correct answers that differ by context.

---

## 5. Export

### 5.1 Fixed layout

The rendered Directory is a **flat sequence of Household blocks**, ordered by a depth-first walk of
the tree. A Household never nests inside its parent's block, regardless of depth (ADR-0002). This
is how printed family and church directories actually look, and it avoids the unreadable
indentation that deep nesting produces.

Layout is **not customisable** — one typeface, one set of margins, one block format. Layout choice
here is a support-call generator with no upside: the Editor wants a directory that looks right, not
a document to design. Consistency by removal of choice.

### 5.1a Rendering pipeline

**Typst**, invoked as a subprocess against a template file on disk (ADR-0004):

```
  flat records ──▶ tier filter ──▶ Typst markup ──▶ typst compile ──┬──▶ PDF  (export)
                                                                    └──▶ SVG  (preview)
```

Go never performs layout — it generates markup and shells out. One renderer produces both the
in-app preview and the printed export, so the preview cannot drift from what prints.

Typst is a **host dependency**, not vendored. The application verifies its presence and version at
startup and fails with an Operator-facing message, so a missing binary never surfaces to the Editor
as an opaque error mid-export.

The template on disk, rather than embedded in the binary, is a deliberate Operator affordance:
"the addresses look cramped" is a template edit and a restart, not a rebuild and redeploy.

The template's location is `WHEREFOLK_TEMPLATE`, defaulting to `template/` beneath the data volume
so the Operator edits it through the mount they already have. The renderer is verified at startup —
binary present, version readable, `directory.typ` in place — and reported on `/status` beside the
document path (§2.4).

### 5.2 Tiers

Named for what the recipient will do with the document, not by an abstract sensitivity scale
(ADR-0003). The Editor picks one at export time by its description, not by a number.

| Tier | For | Living people's dates | Phone | Email | Minors' details |
|---|---|---|---|---|---|
| **Mail** | Sending cards | Truncated | — | — | — |
| **Call** | Phoning relatives | Truncated | 18+ only | — | — |
| **Digital** | Email and messaging | Truncated | 18+ only | 18+ only | — |
| **Full** | Full details | Whole | All | All | Included |

"Minors' details" is the phone and email that the 18+ columns gate, not an additional rule: a
minor's name and Truncated birth date appear in every tier, because the Directory is how the family
knows when to send a birthday card.

All four include full mailing addresses and the names of every family member, living and deceased.
**Full** is additionally passphrase-protected (a `pdfcpu` post-processing step, since Typst cannot
encrypt) and carries a `DO NOT DISTRIBUTE` footer on every page; its audience is small enough that
distributing a passphrase by phone is realistic.

No tier exports data marked hidden. **Proof** (§5.6) is a fifth tier of a different kind.

### 5.3 Dates

Truncation exists to keep a living person's identity-verification key — full name plus full date of
birth — out of wide circulation. It therefore follows *who the date concerns*, not the tier alone:

- A **living** person's birth date is Truncated outside the Full tier.
- A **deceased** person's birth and death dates are always Whole, in every tier. The dead cannot be
  impersonated this way, so truncation would cost genealogical reference value and buy nothing.
- An **Anniversary** follows its Household: Truncated while either adult is living — a surviving
  spouse's wedding date is a plausible security question — and Whole once both have died.

Stated in one line: *a date is truncated while the person it concerns is living.*

### 5.4 Memorial Households

A Memorial Household appears in **every** tier as a names-and-dates reference, with no contact
details. Its adults' dates and its Anniversary are Whole, because they concern only the dead; a
Dependent who is still living keeps §5.3's rule, so their birth date is Truncated outside Full like
anyone else's. It renders more compactly than a live Household — a heading with dates
rather than a full entry — since there is nothing in it to act on, but it must appear so that
descendants group correctly beneath it and no Path points at a node missing from the document.

### 5.5 Two kinds of absence

These are distinct and must render differently:

- **Hidden** — the Editor withheld it. Renders as `[private]`, so nobody "helpfully" re-collects it
  next year.
- **Tier-suppressed** — the tier omits it. Renders as **nothing at all**. A marker here would
  advertise that the data exists, leaking exactly what the tier is meant to omit.

`[private]` is used in preference to a lock glyph: it survives any font stack, needs no legend, and
reads correctly aloud to a screen reader. A lock icon may accompany it only if verified to embed
cleanly in the PDF.

**Both kinds of absence belong to export, and neither appears in the editing UI** (ADR-0010). The
rules above govern the tier filter and every rendered Directory; the detail pane shows the Editor
every stored value, withheld or not, marked by a checkbox. §5.4's rule that a Memorial Household
publishes no contact details is likewise a statement about what is *published* — the pane still
shows a deceased person's recorded phone number, because the Editor must be able to correct or
clear it. A value the Editor cannot see is a value they cannot fix.

### 5.6 Proof Sheets

Relatives cannot correct what they have never seen, so the Directory renders a **Proof Sheet** per
Household — one Household per page, showing everything held about them — which the Editor mails or
emails to that household. Corrections come back by whatever channel the recipient prefers and the
Editor types them in. There is no inbound path: accepting submissions would reintroduce the public
exposure, identity model, and review queue that §2 removed (ADR-0005).

Proof differs from the other four tiers structurally, not just in field visibility:

- **One Household per page**, so each can be sent separately.
- **Only Households with a living member.** Memorial Households are excluded automatically — there
  is nobody to send one to.
- **Withheld fields are shown**, with their actual values, marked plainly as not appearing in the
  shared Directory (`123 Elm St — withheld from all exports`). The recipient is the data subject;
  showing `[private]` would give them nothing to confirm, and they cannot tell a stale withheld
  address from a current one. The marking must be unmistakable, or a recipient will think their
  suppression failed.
- **Own Household only.** A sheet never shows withheld fields belonging to another Household, even
  a parent's or sibling's.

A full run produces roughly one page per living Household, which for a large family is sixty-plus
pages. The export UI should therefore support generating sheets for a single Branch, and warn on
page count before a whole-family run.

### 5.7 Age gating

Age is computed at export time, which makes **exports non-reproducible** — the same Directory
exported months apart differs as people turn 18. Every export is therefore stamped with its
generation date.

People with **no recorded birthdate are treated as minors** (fail closed). Because this silently
suppresses details for elderly relatives whose birth year nobody knows, export shows a pre-flight
warning listing them by name, turning a silent data-quality problem into a visible, fixable one.

### 5.8 What export cannot do

An exported file cannot be protected from a recipient who is meant to read it. PDF permission flags
are advisory and stripped in one command; rasterising text merely invites OCR while destroying
searchability and accessibility. **Disclosure is controlled by what enters the file**, which is why
tiers and hidden flags are the whole mechanism. Every export carries a footer naming its tier
and date — weak social pressure, but free, and honest that distribution is the real control.

---

## 6. Implementation sections

Roughly ordered by dependency; each is independently specifiable. The numbers are stable
identifiers, not a build order — item 11 is a prerequisite of item 4 and is built before it.

| # | Section | Depends on |
|---|---|---|
| 1 | ✅ **Data model & store** — flat records, stable IDs, atomic JSON write, load-time tree derivation, `schema` version field | — |
| 2 | ⏸️ **Migration** — *deferred until a schema 2 exists* (§2.1). The refuse-if-newer guard already shipped in item 1; there is no v0 data to convert. | 1 |
| 3 | ✅ **Validation & normalisation** — dates, phones, emails; normalise-on-save | 1 |
| 4 | ✅ **Tree + search navigation** — two-pane shell, Path breadcrumb, search-with-context. Navigation state lives in the URL (ADR-0008) | 1, 11 |
| 5 | ✅ **Detail editing** — the pane is the form (§4.4), per-field hidden, add/remove a person, declared Promotion, structural-change announcements. Saves via Post/Redirect/Get (ADR-0008); masking moved to export (ADR-0010); Promotion is one-way (ADR-0009) | 3, 4 |
| 6 | **Safety net** — session undo, 30-day trash, nightly snapshots. Inherits two slots from item 5: the `.announce-actions` div in `_announce.html` where the undo control belongs, and Household deletion, which item 5 left out because deleting with no recovery path contradicts §3 | 1 |
| 7 | ✅ **Typst template & render engine** — flat Household blocks, Memorial blocks, fixed layout, shared-address back-references, PDF + SVG output. The render model is strings only, so item 8's filter replaces its constructor rather than threading a tier through the markup generator | 1 |
| 8 | ✅ **Tier filter** — field gating, age computation, date truncation, `[private]` vs. absence. `render.Build(tree, tier, asOf)` is the only constructor of the render model; the footer names the tier, and Full carries `DO NOT DISTRIBUTE`. Suppression beats withholding; Shared Addresses resolve through withheld and Memorial targets | 7 |
| 9 | **Export UI** — tier chooser by description, SVG preview, `pdfcpu` passphrase on Full. Pre-flight warnings split out to item 15 | 8 |
| 10 | **Proof Sheets** — per-Household pagination, withheld-field disclosure, Branch selection | 8 |
| 11 | ✅ **Web shell** — Go templates, htmx, embedded assets, loopback binding, `WHEREFOLK_DATA`/`WHEREFOLK_PORT`, Operator `/status` page as the tracer bullet. Foundational: every UI item (4, 5, 9, 10) is built on it, so it ships first | — |
| 12 | **Container image** — Debian slim, pinned Typst, compose file with Tailscale sidecar, volumes | 11 |
| 13 | **Tailnet setup** — OAuth client, `tag:wherefolk`, ACL, Serve with HTTPS, Funnel assertion, agenix-managed `.env` | 12 |
| 14 | **Health & snapshots** — `/healthz`, Docker `HEALTHCHECK`, verified snapshot job, two Healthchecks.io dead-man's switches | 6, 12 |
| 15 | **Export pre-flight warnings** (§5.7) — on the export page, list each living person whose missing birth date removes something from *that* tier's export (a recorded phone in Call; a phone or email in Digital; nobody in Mail or Full), with their Path and a link to their Household. A warning, never a block. Computed by the same code in `internal/render` that applies the rule, so the warning and the PDF cannot disagree. Split from item 9 as a usability improvement rather than a prerequisite | 9 |

### Fate of the existing code

- `pkg/rolo` — the recursive `Family` walk does not survive §1 and §7. Rewritten.
- `cmd/` — **deleted in item 11.** The Cobra CLI is superseded by the web UI, and the binary now
  takes no arguments at all, so nothing can make it do anything other than serve (§2.2). An Operator
  CLI (export, validate, repair) may return as its own work item, built with the standard library
  `flag` package; that is a different audience from the Editor.
- `internal/tui` — lipgloss styles have no role in a web UI. Retained only if an Operator CLI keeps
  terminal output.

---

## 7. Open questions

None blocking. The four questions left open in the first draft are now resolved: rendering via
Typst (§5.1a, ADR-0004), Memorial Households (§3, §5.4), non-text information excluded permanently
(§1), and outbound-only corrections via Proof Sheets (§5.6, ADR-0005).

Deferred, and deliberately not designed:

- **Phone and tablet layouts** — a separate design effort with a different shape (§1).
- **A second Editor** — would require reintroducing identity, which ADR-0001 removed.
