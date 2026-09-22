# Ship a container with a Tailscale sidecar, despite a NixOS host

The Operator runs NixOS, where a native module would be the obvious packaging, but the service is
shipped as a Docker image with the official `tailscale/tailscale` image as a sidecar. A container
insulates the service from host configuration changes and pins Typst's version inside the image, so
a host upgrade cannot silently reflow the Directory; the sidecar keeps the application container to
a single process rather than running a supervisor to babysit `tailscaled`.

## Consequences

Tailscale's node state must be a named volume, or every restart authenticates as a new node and the
Editor's bookmarked hostname stops resolving. Access uses `tailscale serve`, which is tailnet-only
despite its publicly-trusted certificate — **`tailscale funnel` is a one-word difference that would
publish the Directory to the open internet**, so the deployment asserts Funnel is disabled rather
than merely not enabling it. The node authenticates via an OAuth client bound to `tag:wherefolk`
rather than an auth key, which would expire at 90 days and fail one morning next quarter, and an ACL
restricts that tag to the Operator's and Editor's devices because tailnet membership is otherwise
coarse. Nix remains the development environment but not the deployment mechanism, and GoReleaser is
dropped as redundant for a single target with one user.
