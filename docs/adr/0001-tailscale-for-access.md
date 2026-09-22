# Reach the service over Tailscale rather than exposing it publicly

Wherefolk is a web service hosted on hardware the developer controls, but its sole user is a
non-technical relative on a different network. Joining both machines to a Tailscale mesh lets the
user reach it via a bookmark while the developer keeps full administrative access, with no public
attack surface for a dataset of relatives' home addresses, birthdates, and phone numbers.

## Consequences

The application has no concept of a user, a session, or a login, because the network layer has
already authenticated the caller — this removes the largest subsystem in the alternative design and
the support burden of password resets for a 70-year-old. It should bind to the Tailscale interface
rather than `0.0.0.0`, so the privacy property is enforced by code rather than by configuration.
Adding a second editor later means reintroducing identity from scratch; an audit trail is
impossible until then.
