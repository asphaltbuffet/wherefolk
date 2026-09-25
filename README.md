# wherefolk

> A family directory service: a printable, shareable address book covering an extended family.

Wherefolk holds contact information for an extended family, records important dates, and makes
familial relationships legible. It runs as a small HTTP service on a private
[Tailscale](https://tailscale.com) network — there is no public endpoint and no CLI.

See [CONTEXT.md](CONTEXT.md) for the domain vocabulary and [docs/adr/](docs/adr/) for the decisions
behind the current shape.

## Running

The binary is a service, not a toolbox: it takes no arguments and is configured entirely from the
environment.

| Variable | Default | Meaning |
|---|---|---|
| `WHEREFOLK_DATA` | `/var/lib/wherefolk` | Directory holding `directory.json` |
| `WHEREFOLK_TEMPLATE` | `/var/lib/wherefolk/template` | Directory holding `directory.typ`, the Typst layout. On disk rather than embedded so a layout tweak is a file edit and a restart — see [ADR-0004](docs/adr/0004-typst-for-rendering.md) |
| `WHEREFOLK_PORT` | `8080` | Port to listen on (the interface is fixed — see [ADR-0007](docs/adr/0007-configurable-port-fixed-interface.md)) |
| `WHEREFOLK_LOG_LEVEL` | `info` | Log threshold: `debug`, `info`, `warn`, or `error` |

### Container

Wherefolk is deployed as a container behind a Tailscale sidecar
([ADR-0006](docs/adr/0006-container-deployment-with-tailscale-sidecar.md)):

```bash
docker run --rm \
  -v /srv/wherefolk:/var/lib/wherefolk \
  ghcr.io/asphaltbuffet/wherefolk:latest
```

There is deliberately no published port. The service binds loopback only
([ADR-0001](docs/adr/0001-tailscale-for-access.md),
[ADR-0007](docs/adr/0007-configurable-port-fixed-interface.md)), so `-p` would
forward to an interface nothing listens on. The Tailscale sidecar shares the
container's network namespace and reaches the service over that same loopback,
which makes `tailscale serve` the only way in.

### Locally

```bash
mise run run    # serves ./testdata on :8099
```

Then visit <http://127.0.0.1:8099/status>.

## Data

A document is a JSON object with a `schema` version and a flat `households` array — **not** a nested
tree. Each Household carries its own `id` and an optional `parent`; the hierarchy is rebuilt from
those links at load time. [`testdata/directory.json`](testdata/directory.json) is the canonical
example.

## Development

```bash
nix develop            # enter dev shell
mise run test          # test
mise run lint          # lint
mise run build         # build with version ldflags
mise run snapshot      # goreleaser build + container image, no push
```

Rendering needs the `typst` binary. `nix develop` provides it; outside that shell the
render tests skip rather than fail.

### Releasing

Changes are described as [changie](https://changie.dev) fragments rather than edited into the
changelog directly:

```bash
changie new                        # describe a change
mise run pre-release <major|minor|patch>   # batch fragments, merge CHANGELOG, open a PR
mise run release                   # after merge: tag and push; CI publishes
```

## License

MIT — see [LICENSE](LICENSE)
