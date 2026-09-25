# Render through Typst as a subprocess

A family directory is a long flow of variable-height Household blocks, so its quality rests on
pagination — never splitting a Household across a page break, consistent vertical rhythm — which
native Go PDF libraries leave the caller to implement by hand. Typst is invoked as a subprocess
against a template file on disk, producing PDF for export and SVG for the in-app preview, so a
single renderer defines the layout and the preview cannot drift from the print.

## Considered Options

Headless Chrome was rejected for its ~150MB dependency despite CSS Paged Media handling pagination
well. Embedding Typst through CGO was rejected because a Rust toolchain in the build breaks simple
cross-compilation — a worse problem than the subprocess it would avoid.

## Consequences

Typst is a host dependency, not vendored, so the application must verify its presence and version
at startup and fail with an Operator-facing message rather than surfacing an opaque error to the
Editor at export time. The template lives on disk rather than embedded in the binary, so layout
adjustments are a file edit and a restart instead of a rebuild. Typst cannot encrypt PDFs, so the
Full tier's passphrase protection is a separate post-processing step via `pdfcpu`.

**The startup check makes the renderer a deployment dependency, not just a runtime one.** The image
originally used a `distroless/static` base, which was correct while nothing shelled out: it has no
typst, no shell and no package manager. Making the check fatal turned that into a container that
exits 1 before binding a port. The two decisions were each defensible alone and only collided when
the render engine shipped. The image therefore moved to Debian slim, installs Typst from its pinned
tarball, and carries `template/` — a renderer the binary refuses to start without must be *in* the
artifact that ships the binary.

The template ships in the image at `/usr/local/share/wherefolk/template` rather than on the data
volume, because it is versioned with the code that generates markup for it: a template expecting
arguments the binary no longer emits fails at export time. `WHEREFOLK_TEMPLATE` points there
explicitly, since the config default sits *under* the volume mount and an empty named volume would
otherwise shadow it. Mounting over that path keeps the "edit and restart" affordance intact.
