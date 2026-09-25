# Encrypt the Full tier with pdfcpu as a Go library, not a host binary

The Full-tier Directory is passphrase-protected after Typst renders it, because Typst cannot
encrypt a PDF (ADR-0003). That step uses pdfcpu compiled into the binary as a Go library, rather
than a `pdfcpu` executable invoked as a subprocess the way Typst is (ADR-0004).

Typst is a host dependency because it is the layout engine: its version decides how the Directory
reflows, so it is pinned deliberately in the image, `mise.toml` and `flake.nix`, and the template
beside it is an Operator affordance. Encryption has none of those properties. Its output does not
change with the version in any way a reader would see, and there is nothing for the Operator to
edit. As a host binary it would add another tarball, checksum pair and version pin to keep in
agreement, plus another reason for startup to fail, and it would buy nothing.

## Consequences

The module gains pdfcpu's dependency tree (`golang.org/x/image`, `golang.org/x/text`, a YAML
parser). That is a real cost for a project whose defining constraint is privacy, and it is accepted
because `go.sum` pins it and Go's checksum database verifies it, which is at least as strong as a
checksum in a Dockerfile. Encryption is then a pure `[]byte → []byte` function, testable without
anything installed, and the container image changes not at all.
