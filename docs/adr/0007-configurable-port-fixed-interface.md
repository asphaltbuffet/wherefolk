# Configure the port, hardcode the loopback interface

ADR-0001 removed authentication on the grounds that the tailnet authenticates and the binary binds
loopback only, which makes the bind interface a security property rather than a setting. A single
`WHEREFOLK_ADDR` variable — the conventional Go shape — would put that property one typo away from
`0.0.0.0:8080` in the same `.env` file that holds the OAuth client, so the two halves of the address
are split: `WHEREFOLK_PORT` is configurable and defaults to `8080`, while the host is a package
constant that no environment variable, flag, or code path can change.

## Consequences

A wrong port makes the service unreachable, which is loud and self-correcting; a wrong interface
would publish family addresses with no error at all, so only the harmless half is exposed. The port
stays configurable because the app shares a network namespace with the Tailscale sidecar
(`network_mode: service:tailscale`), where a collision is plausible and rebuilding the image to
change a number would be the wrong remedy.

An unparseable or out-of-range `WHEREFOLK_PORT` is a fatal startup error rather than a fall back to
the default: a container serving on an unexpected port while `tailscale serve` forwards elsewhere
passes its own health check and presents as a confusing outage.

Debugging access is unchanged and stays at the container layer per §2.2 — `docker exec`, or
`docker`'s own port publishing, both of which are visible in the compose file rather than inside the
binary.
